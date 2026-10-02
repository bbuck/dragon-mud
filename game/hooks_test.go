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
// containing unwanted arrives first.
func (c *client) expectWithout(want, unwanted string) {
	c.t.Helper()

	timeout := time.After(2 * time.Second)
	for {
		select {
		case m := <-c.conn.messages:
			if strings.Contains(m.Text, unwanted) {
				c.t.Fatalf("received %q before %q", m.Text, want)
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
		"plugin.lua": {Data: []byte(`return { name = "game" }`)},
		"hooks.lua": {Data: []byte(`
			return {
				before_say = function(event)
					if event.message:find("darn") then
						return false, "Mind your language."
					end
					event.message = event.message:upper()
					return event
				end,
			}
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
			before_say = function(event)
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
			return { hooks = { player_entered = { disable = { "dragon:presence" } } } }
		`)},
	})

	alice := connect(t, g)
	alice.login("Alice")
	bob := connect(t, g)
	bob.login("Bob")

	bob.send("say hi")
	alice.expectWithout(`Bob says, "hi"`, "has arrived")
}

func TestGameHandlerRunsAfterBasics(t *testing.T) {
	g := startGame(t, fstest.MapFS{
		"plugin.lua": {Data: []byte(`return { name = "game" }`)},
		"hooks.lua": {Data: []byte(`
			return {
				player_entered = function(event)
					game.broadcast("Trumpets sound for " .. event.player:get("name") .. ".")
				end,
			}
		`)},
	})

	alice := connect(t, g)
	alice.login("Alice")
	alice.expect("Trumpets sound for Alice.")
	bob := connect(t, g)
	bob.login("Bob")

	alice.expectWithout("Bob has arrived.", "Trumpets")
	alice.expect("Trumpets sound for Bob.")
}

func TestFailingNotificationDoesntStopOthers(t *testing.T) {
	g := startGame(t, fstest.MapFS{
		"plugin.lua": {Data: []byte(`return { name = "game" }`)},
		"hooks.lua": {Data: []byte(`
			return {
				player_entered = {
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
			fstest.MapFS{"wiring.lua": {Data: []byte(`return { hooks = { player_entered = { disable = { "dragon:presense" } } } }`)}},
			`game/wiring.lua: hooks.player_entered disables "dragon:presense", which has no player_entered handler. Did you mean "dragon:presence"? Plugins with a player_entered handler: "dragon:presence".`,
		},
		{
			"wiring unknown hook",
			fstest.MapFS{"wiring.lua": {Data: []byte(`return { hooks = { player_entred = { disable = { "dragon:chat" } } } }`)}},
			`game/wiring.lua: hooks.player_entred is wired, but no plugin handles "player_entred". Did you mean "player_entered"?`,
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
		"hooks.lua": {Data: []byte(`
			return {
				can_dance = function(event)
					if event.partner == "nobody" then return false, "Dance with whom?" end
				end,
				danced = function(event) event.actor:send("The crowd cheers.") end,
			}
		`)},
		"commands.lua": {Data: []byte(`
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
			"hooks.lua":  {Data: []byte(`return { player_entered = { before = { "dragon:presence" }, handler = function() end } }`)},
		}),
	})
	if err != nil {
		t.Fatal(err)
	}

	if got := hooks.Names(); !reflect.DeepEqual(got, []string{"player_entered", "player_left"}) {
		t.Errorf("Names = %v", got)
	}

	c, _ := hooks.Chain("player_entered")
	var order []string
	for _, h := range c.Handlers {
		order = append(order, h.Plugin)
	}
	if !reflect.DeepEqual(order, []string{"game", "dragon:presence"}) {
		t.Errorf("order = %v", order)
	}
}
