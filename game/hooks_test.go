package game

import (
	"context"
	"io"
	"log/slog"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"bbuck.dev/dragon-mud/plugin"
	"bbuck.dev/dragon-mud/scripting"
	"bbuck.dev/dragon-mud/scripting/lua"
)

// expectWithout waits for a message containing want, failing if a message
// containing any of unwanted arrives first.
func (c *client) expectWithout(want string, unwanted ...string) {
	c.t.Helper()

	timeout := time.After(2 * time.Second)
	for {
		select {
		case m := <-c.conn.messages:
			for _, u := range unwanted {
				if strings.Contains(m.Text, u) {
					c.t.Fatalf("received %q before %q", m.Text, want)
				}
			}
			if strings.Contains(m.Text, want) {
				return
			}
		case <-timeout:
			c.t.Fatalf("never received %q", want)
		}
	}
}

func TestGameHooksChangeAndCancelSay(t *testing.T) {
	files := fstest.MapFS{
		"plugin.lua":   {Data: []byte(`return { name = "game" }`)},
		"lua/room.lua": oneRoom["lua/room.lua"],
		"hooks.lua": {Data: []byte(`
			local handlers = require("room")
			handlers["dragon:before_say"] = function(event)
				if event.message:find("darn") then
					return false, "Mind your language."
				end
				event.message = event.message:upper()
				return event
			end
			return handlers
		`)},
	}
	g := startGame(t, files)

	alice := connect(t, g)
	alice.login("Alice")
	bob := connect(t, g)
	bob.login("Bob")

	bob.send("say hello")
	bob.expect(`You say, "HELLO"`)
	alice.expect(`Bob says, "HELLO"`)

	bob.send("say darn it")
	bob.expect("Mind your language.")

	bob.send("say quietly to alice")
	alice.expect(`Bob says to you, "QUIETLY"`)

	// Hooks reload with everything else.
	files["hooks.lua"] = &fstest.MapFile{Data: []byte(`
		return {
			["dragon:before_say"] = function(event)
				if event.message == "anyone?" then return false end
			end,
		}
	`)}
	g.Reload()
	bob.send("say anyone?")
	bob.send("say done")
	alice.expectWithout(`Bob says, "done"`, "anyone?")
}

func TestWiringDisablesBasicsArrival(t *testing.T) {
	g := startGame(t, fstest.MapFS{
		"plugin.lua": {Data: []byte(`return { name = "game" }`)},
		"wiring.lua": {Data: []byte(`
			return { hooks = { ["dragon:player_connected"] = { disable = { "dragon:presence" } } } }
		`)},
	})

	alice := connect(t, g)
	alice.login("Alice")
	bob := connect(t, g)
	bob.login("Bob")

	bob.send("say hi")
	alice.expectWithout(`Bob says, "hi"`, "has arrived")
}

// Arrival is announced after the game's handler, so a game that puts new
// characters somewhere has done so when presence says where they arrived.
func TestArrivalWaitsForTheGame(t *testing.T) {
	g := startGame(t, fstest.MapFS{
		"plugin.lua": {Data: []byte(`return { name = "game" }`)},
		"hooks.lua": {Data: []byte(`
			local game = require("dragon.game")
			return {
				["dragon:player_connected"] = function(event)
					game.broadcast("Trumpets sound for " .. event.actor:get("name") .. ".")
				end,
			}
		`)},
	})

	alice := connect(t, g)
	alice.login("Alice")
	alice.expect("Trumpets sound for Alice.")
	bob := connect(t, g)
	bob.login("Bob")

	alice.expectWithout("Trumpets sound for Bob.", "Bob has arrived.")
	alice.expect("Bob has arrived.")
}

func TestFailingNotificationDoesntStopOthers(t *testing.T) {
	g := startGame(t, fstest.MapFS{
		"plugin.lua": {Data: []byte(`return { name = "game" }`)},
		"hooks.lua": {Data: []byte(`
			return {
				["dragon:player_connected"] = {
					before = { "dragon:presence" },
					handler = function() error("boom") end,
				},
			}
		`)},
	})

	alice := connect(t, g)
	alice.login("Alice")
	bob := connect(t, g)
	bob.login("Bob")
	alice.expect("Bob has arrived.")
}

func TestHooksFileErrors(t *testing.T) {
	tests := []struct {
		name  string
		files fstest.MapFS
		want  string
	}{
		{
			"not a function",
			fstest.MapFS{"hooks.lua": {Data: []byte(`return { before_say = "nope" }`)}},
			`game/hooks.lua: before_say must be a function(event), or a table like { after = { "dragon:chat" }, handler = function(event) ... end }, not a string.`,
		},
		{
			"unknown field",
			fstest.MapFS{"hooks.lua": {Data: []byte(`return { before_say = { handler = function() end, afters = {} } }`)}},
			`game/hooks.lua: before_say has an unknown field "afters". Did you mean "after"?`,
		},
		{
			"string instead of list",
			fstest.MapFS{"hooks.lua": {Data: []byte(`return { before_say = { handler = function() end, after = "dragon:chat" } }`)}},
			`game/hooks.lua: before_say: after must be a list of plugins. Write after = { "dragon:chat" }.`,
		},
		{
			"bad name",
			fstest.MapFS{"hooks.lua": {Data: []byte(`return { ["Before Say"] = function() end }`)}},
			`game/hooks.lua: Before Say isn't a valid hook name.`,
		},
		{
			"wiring typo",
			fstest.MapFS{"wiring.lua": {Data: []byte(`return { hooks = { ["dragon:player_connected"] = { disable = { "dragon:presense" } } } }`)}},
			`game/wiring.lua: hooks.dragon:player_connected disables "dragon:presense", which has no dragon:player_connected handler. Did you mean "dragon:presence"? Plugins with a dragon:player_connected handler: "dragon:presence".`,
		},
		{
			"wiring unknown hook",
			fstest.MapFS{"wiring.lua": {Data: []byte(`return { hooks = { ["dragon:player_conected"] = { disable = { "dragon:chat" } } } }`)}},
			`game/wiring.lua: hooks.dragon:player_conected is wired, but no plugin handles "dragon:player_conected". Did you mean "dragon:player_connected"?`,
		},
		{
			"wiring unknown field",
			fstest.MapFS{"wiring.lua": {Data: []byte(`return { hook = {} }`)}},
			`game/wiring.lua has an unknown field "hook". Did you mean "hooks"?`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.files["plugin.lua"] = &fstest.MapFile{Data: []byte(`return { name = "game" }`)}
			_, err := newGame(t, tt.files)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %v\nwant it to contain %q", err, tt.want)
			}
		})
	}
}

func TestOnlyTheGameWires(t *testing.T) {
	srcs := append(sources(t, nil), plugin.Source{
		Origin: "plugins/extra",
		Files: fstest.MapFS{
			"plugin.lua": {Data: []byte(`return { name = "extra" }`)},
			"wiring.lua": {Data: []byte(`return {}`)},
		},
	})

	_, err := New(context.Background(), Options{
		NewEngine: func() scripting.Engine { return lua.New() },
		Plugins:   srcs,
		Store:     openStore(t),
		Log:       slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	want := "plugins/extra: extra/wiring.lua: only the game's own plugin can wire hooks."
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Errorf("error = %v\nwant it to contain %q", err, want)
	}
}

func TestHooksRunFromLua(t *testing.T) {
	g := startGame(t, fstest.MapFS{
		"plugin.lua": {Data: []byte(`return { name = "game" }`)},
		"events.lua": {Data: []byte(`
			return {
				can_dance = { fields = { partner = "who with" } },
				danced = { fields = { actor = "who danced" } },
				nothing_handles_this = {},
			}
		`)},
		"hooks.lua": {Data: []byte(`
			return {
				can_dance = function(event)
					if event.partner == "nobody" then return false, "Dance with whom?" end
				end,
				danced = function(event) event.actor:send("The crowd cheers.") end,
			}
		`)},
		"commands.lua": {Data: []byte(`
						local hooks = require("dragon.hooks")
			return {
				dance = {
					forms = {
						{ "dance with <partner>", function(actor, args)
							local event, reason = hooks.run("can_dance", { partner = args.partner })
							if not event then
								actor:send(reason)
								return
							end
							actor:send("You dance with " .. event.partner .. ".")
							hooks.notify("danced", { actor = actor })
						end },
						{ "dance", function(actor)
							local event, reason = hooks.run("nothing_handles_this")
							actor:send(event and "Unchanged." or "Cancelled?")
						end },
					},
				},
			}
		`)},
	})

	alice := connect(t, g)
	alice.login("Alice")

	alice.send("dance with nobody")
	alice.expect("Dance with whom?")

	alice.send("dance with Bob")
	alice.expect("You dance with Bob.")
	alice.expect("The crowd cheers.")

	alice.send("dance")
	alice.expect("Unchanged.")
}

func TestHooksForTheCLI(t *testing.T) {
	hooks, err := Hooks(context.Background(), Options{
		NewEngine: func() scripting.Engine { return lua.New() },
		Plugins: sources(t, fstest.MapFS{
			"plugin.lua": {Data: []byte(`return { name = "game" }`)},
			"hooks.lua":  {Data: []byte(`return { ["dragon:player_connected"] = { before = { "dragon:presence" }, handler = function() end } }`)},
		}),
	})
	if err != nil {
		t.Fatal(err)
	}

	if got := hooks.Names(); !reflect.DeepEqual(got, []string{"dragon:player_connected", "dragon:player_disconnected"}) {
		t.Errorf("Names = %v", got)
	}

	c, _ := hooks.Chain("dragon:player_connected")
	var order []string
	for _, h := range c.Handlers {
		order = append(order, h.Plugin)
	}
	if !reflect.DeepEqual(order, []string{"game", "dragon:presence"}) {
		t.Errorf("order = %v", order)
	}
}

func TestEventsAreChecked(t *testing.T) {
	g := startGame(t, fstest.MapFS{
		"plugin.lua": {Data: []byte(`return { name = "game" }`)},
		"events.lua": {Data: []byte(`
			return {
				greeted = {
					desc = "Someone was greeted.",
					fields = { actor = "who greeted", target = { "who they greeted", optional = true } },
				},
			}
		`)},
		"hooks.lua": {Data: []byte(`
			return {
				greeted = function(event) event.actor:send("Greeted.") end,
			}
		`)},
		"commands.lua": {Data: []byte(`
			local hooks = require("dragon.hooks")
			return {
				greet = { execute = function(actor) hooks.notify("greeted", { actor = actor }) end },
				typo = { execute = function(actor) hooks.notify("greeted", { speaker = actor }) end },
				misspelled = { execute = function(actor) hooks.run("greted", { actor = actor }) end },
			}
		`)},
	})

	alice := connect(t, g)
	alice.login("Alice")

	alice.send("greet")
	alice.expect("Greeted.")
	alice.send("typo")
	alice.expect(`dragon.hooks.notify: greeted: the event has a field "speaker", which greeted doesn't have. Its fields are actor and target.`)
	alice.send("misspelled")
	alice.expect(`dragon.hooks.run: no plugin declares the hook "greted". Did you mean "greeted"?`)
}

func TestEventsFileErrors(t *testing.T) {
	tests := []struct {
		name   string
		events string
		want   string
	}{
		{
			"not a table",
			`return { greeted = "someone was greeted" }`,
			`game/events.lua: greeted must be a table like { desc = "...", fields = { actor = "who did it" } }, not a string.`,
		},
		{
			"unknown key",
			`return { greeted = { field = {} } }`,
			`game/events.lua: greeted has an unknown field "field". Did you mean "fields"?`,
		},
		{
			"field without a description",
			`return { greeted = { fields = { actor = true } } }`,
			`game/events.lua: greeted: field actor must be a description, like actor = "who did it", or a table like { "who did it", optional = true }, not a boolean.`,
		},
		{
			"optional field without a description",
			`return { greeted = { fields = { target = { optional = true } } } }`,
			`game/events.lua: greeted: field target must start with its description, like { "where from", optional = true }.`,
		},
		{
			"misspelled optional",
			`return { greeted = { fields = { target = { "who", optinal = true } } } }`,
			`game/events.lua: greeted: field target has an unknown field "optinal". Did you mean "optional"?`,
		},
		{
			"reserved namespace",
			`return { ["dragon:greeted"] = {} }`,
			`game/events.lua: dragon:greeted uses the "dragon:" namespace, which is reserved for the engine's built-in plugins. Use your plugin's name instead, like "game:greeted".`,
		},
		{
			"section",
			`return { ["section:room.exits"] = {} }`,
			`the engine declares section hooks, so plugins don't.`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := newGame(t, fstest.MapFS{
				"plugin.lua": {Data: []byte(`return { name = "game" }`)},
				"events.lua": {Data: []byte(tt.events)},
			})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %v\nwant it to contain %q", err, tt.want)
			}
		})
	}
}

func TestUndeclaredHandlersAreLogged(t *testing.T) {
	var log strings.Builder
	_, err := New(context.Background(), Options{
		Name:      "Test Realm",
		NewEngine: func() scripting.Engine { return lua.New() },
		Plugins: sources(t, fstest.MapFS{
			"plugin.lua": {Data: []byte(`return { name = "game" }`)},
			"hooks.lua":  {Data: []byte(`return { ["dragon:player_conected"] = function() end }`)},
		}),
		Store: openStore(t),
		Log:   slog.New(slog.NewTextHandler(&log, nil)),
	})
	if err != nil {
		t.Fatal(err)
	}

	want := `no plugin declares dragon:player_conected, so its handlers never run; it may be misspelled, or from a plugin the game doesn't load. Did you mean \"dragon:player_connected\"?"`
	if !strings.Contains(log.String(), want) || !strings.Contains(log.String(), "handlers=game/hooks.lua") {
		t.Errorf("log = %s\nwant it to contain %s", log.String(), want)
	}
}

// A reaction to speech comes after the line it reacts to, for everyone.
func TestReactionsComeAfterTheAction(t *testing.T) {
	g := startGame(t, fstest.MapFS{
		"plugin.lua":   {Data: []byte(`return { name = "game" }`)},
		"lua/room.lua": oneRoom["lua/room.lua"],
		"hooks.lua": {Data: []byte(`
			local game = require("dragon.game")
			local handlers = require("room")
			for name, handler in pairs({
				["dragon:said"] = function(event)
					if event.message:find("hail") then
						game.broadcast("The keeper nods to " .. event.actor:get("name") .. ".")
					end
				end,
				["dragon:before_emote"] = function(event)
					if event.action:find("dances") then return false, "Not in here." end
					event.action = event.action:upper()
					return event
				end,
				["dragon:emoted"] = function(event)
					game.broadcast("The keeper saw: " .. event.action)
				end,
			}) do handlers[name] = handler end
			return handlers
		`)},
	})

	alice := connect(t, g)
	alice.login("Alice")
	bob := connect(t, g)
	bob.login("Bob")

	alice.send("say hail")
	alice.expectWithout(`You say, "hail"`, "keeper")
	alice.expect("The keeper nods to Alice.")
	bob.expectWithout(`Alice says, "hail"`, "keeper")
	bob.expect("The keeper nods to Alice.")

	alice.send("say hail to bob")
	bob.expectWithout(`Alice says to you, "hail"`, "keeper")
	bob.expect("The keeper nods to Alice.")

	alice.send("emote waves.")
	bob.expectWithout("Alice WAVES.", "keeper")
	bob.expect("The keeper saw: WAVES.")

	alice.send("emote dances.")
	alice.expectWithout("Not in here.", "DANCES")
}
