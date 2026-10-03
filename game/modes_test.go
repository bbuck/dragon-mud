package game

import (
	"log/slog"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"bbuck.dev/dragon-mud/auth"
	"bbuck.dev/dragon-mud/plugin"
	"bbuck.dev/dragon-mud/scripting"
	"bbuck.dev/dragon-mud/scripting/lua"
)

var confirming = fstest.MapFS{
	"plugin.lua": file(`return { name = "game" }`),
	"commands.lua": file(`
		return {
			destroy = { forms = { { "destroy <thing:object:held>", function(actor, args)
				game.session(actor):push_mode("confirm_destroy", { thing = args.thing })
			end } } },
			make = { execute = function(actor, args)
				world.create({ location = actor, properties = { name = args.text } })
				actor:send("Made " .. args.text .. ".")
			end },
		}
	`),
	"modes.lua": file(`
		return {
			confirm_destroy = {
				enter = function(session, state)
					session:prompt({ text = "Destroy " .. state.thing:get("name") .. "?", choices = { "yes", "no" } })
				end,
				forms = {
					{ "yes", function(session, args, state)
						world.destroy(state.thing)
						session:send("Destroyed.")
						session:pop_mode()
					end },
					{ "no", function(session) session:send("Kept.") session:pop_mode() end },
				},
			},
		}
	`),
}

func TestModeFormsAndChoices(t *testing.T) {
	g := startGame(t, confirming)

	alice := connect(t, g)
	alice.login("Alice")
	alice.send("make lamp")
	alice.expect("Made lamp.")

	alice.send("destroy lamp")
	m := alice.expect("Destroy lamp?")
	if !strings.Contains(m.Text, "1. yes\n  2. no") || !strings.Contains(m.HTML, `<dragon-choice value="yes">`) {
		t.Errorf("prompt = %q / %q, want numbered and clickable choices", m.Text, m.HTML)
	}

	// Commands don't run while the mode waits for an answer.
	alice.send("say hi")
	alice.expect("Huh? You can type:\n  [c]yes[x]\n  [c]no[x]")

	alice.send("2")
	alice.expect("Kept.")

	// Back to commands.
	alice.send("destroy lamp")
	alice.expect("Destroy lamp?")
	alice.send("yes")
	alice.expect("Destroyed.")
	alice.send("destroy lamp")
	alice.expect("You aren't carrying 'lamp'.")
}

func TestModeStateAndResume(t *testing.T) {
	g := startGame(t, fstest.MapFS{
		"plugin.lua": file(`return { name = "game" }`),
		"commands.lua": file(`
			return {
				describe = { execute = function(actor)
					game.session(actor):push_mode("describe")
				end },
				desc = { execute = function(actor)
					actor:send("Description: " .. (actor:get("description") or "none"))
				end },
			}
		`),
		"modes.lua": file(`
			return {
				describe = {
					enter = function(session)
						session:push_mode("editor", { lines = {} })
					end,
					resume = function(session, state, text)
						session.character:set("description", text)
						session:send("Saved.")
						session:pop_mode()
					end,
				},
				editor = {
					enter = function(session)
						session:prompt("Type your text. End with a line holding only a period.")
					end,
					input = function(session, line, state)
						if line == "." then
							session:pop_mode(table.concat(state.lines, " "))
							return
						end
						table.insert(state.lines, line)
						return state
					end,
				},
			}
		`),
	})

	alice := connect(t, g)
	alice.login("Alice")
	alice.send("describe")
	alice.expect("End with a line holding only a period.")
	alice.send("Tall and")
	alice.send("  cheerful.")
	alice.send(".")
	alice.expect("Saved.")

	alice.send("desc")
	alice.expect("Description: Tall and   cheerful.")
}

func TestPassthroughMode(t *testing.T) {
	g := startGame(t, fstest.MapFS{
		"plugin.lua": file(`return { name = "game" }`),
		"commands.lua": file(`
			return { fight = { execute = function(actor)
				game.session(actor):push_mode("combat")
				actor:send("En garde!")
			end } }
		`),
		"modes.lua": file(`
			return {
				combat = {
					passthrough = true,
					forms = {
						{ "parry", function(session) session:send("You parry.") end },
						{ "flee", function(session) session:send("You flee.") session:pop_mode() end },
					},
				},
			}
		`),
	})

	alice := connect(t, g)
	alice.login("Alice")
	alice.send("fight")
	alice.expect("En garde!")
	alice.send("parry")
	alice.expect("You parry.")
	alice.send("say still talking")
	alice.expect(`You say, "still talking"`)
	alice.send("flee")
	alice.expect("You flee.")
	alice.send("parry")
	alice.expect("Huh?")
}

func TestCharacterCreationSteps(t *testing.T) {
	g := startGame(t, fstest.MapFS{
		"plugin.lua": file(`return { name = "game" }`),
		"hooks.lua": file(`
			return {
				character_steps = function(event)
					table.insert(event.steps, "skip_me")
					table.insert(event.steps, "choose_class")
					return event
				end,
			}
		`),
		"modes.lua": file(`
			return {
				-- A step that finishes in its own enter.
				skip_me = {
					enter = function(session) session:pop_mode({ title = "the Bold" }) end,
				},
				choose_class = {
					enter = function(session, state)
						session:prompt({ text = state.draft.name .. ", choose a class:", choices = { "warrior", "mage" } })
					end,
					input = function(session, line)
						session:pop_mode({ class = line })
					end,
				},
			}
		`),
		"commands.lua": file(`
			return { me = { execute = function(actor)
				actor:send(actor:get("name") .. " " .. actor:get("title") .. ", " .. actor:get("class"))
			end } }
		`),
	})

	alice := connect(t, g)
	alice.expect("By what name")
	alice.send("alice")
	alice.expect("Create a new account?")
	alice.send("yes")
	alice.expect("Choose a password")
	alice.send("secret pass")
	alice.expect("Type it again")
	alice.send("secret pass")
	alice.expect("Alice, choose a class:\n  1. warrior\n  2. mage")
	alice.send("2")
	alice.expect("Welcome, [W]Alice[x]!")

	alice.send("me")
	alice.expect("Alice the Bold, mage")
}

func TestChoosingAmongCharacters(t *testing.T) {
	db := openStore(t)
	g := startGameWith(t, db, fstest.MapFS{
		"plugin.lua": file(`return { name = "game" }`),
		"commands.lua": file(`
			return { alt = { execute = function(actor)
				game.session(actor).account:add_character(world.create({ properties = { name = "Zed" } }))
				actor:send("Added Zed.")
			end } }
		`),
	})

	alice := connect(t, g)
	alice.login("Alice")
	alice.send("alt")
	alice.expect("Added Zed.")
	alice.send("quit")

	again := connect(t, g)
	again.relogin("alice", " secret pass ")
	m := again.expect("Who will you play?")
	if !strings.Contains(m.Text, "1. Alice\n  2. Zed") {
		t.Errorf("prompt = %q", m.Text)
	}
	again.send("bob")
	again.expect("You have no character called 'bob'.")
	again.send("2")
	again.expect("Welcome, [W]Zed[x]!")
}

// gameWithout returns a game with the built-in plugins except leave, and
// the game plugin if given.
func gameWithout(t *testing.T, gameFiles fstest.MapFS, leave ...string) (*Game, error) {
	t.Helper()

	return New(t.Context(), Options{
		Name:      "Test Realm",
		NewEngine: func() scripting.Engine { return lua.New() },
		Plugins: slices.DeleteFunc(sources(t, gameFiles), func(s plugin.Source) bool {
			return s.Builtin && slices.Contains(leave, s.Origin)
		}),
		Store:  openStore(t),
		Hasher: auth.NewHasher(cheapParams, 4),
		Log:    slog.New(slog.DiscardHandler),
	})
}

func TestGameReplacesCharacterSelect(t *testing.T) {
	selecting := fstest.MapFS{
		"plugin.lua": file(`return { name = "game" }`),
		"modes.lua": file(`
			return {
				characters = {
					enter = function(session)
						session:prompt("Press enter to begin.")
					end,
					input = function(session)
						local c = session.account.characters[1]
						if not c then
							c = world.create({ properties = { name = "Hero" } })
							session.account:add_character(c)
						end
						session:play(c)
					end,
				},
			}
		`),
	}

	_, err := newGame(t, selecting)
	want := `game/modes.lua: mode "characters" sets enter, but dragon:characters already does. Leave "characters" out of builtins in dragon.toml to use only yours, or set replace = true on "characters" in game/modes.lua.`
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("error = %v, want %q", err, want)
	}

	g, err := gameWithout(t, selecting, "characters")
	if err != nil {
		t.Fatal(err)
	}
	runGame(t, g)

	alice := connect(t, g)
	alice.expect("By what name")
	alice.send("alice")
	alice.send("yes")
	alice.send("secret pass")
	alice.send("secret pass")
	alice.expect("Press enter to begin.")
	alice.send("")
	alice.expect("Welcome, [W]Hero[x]!")
}

func TestNoCharactersMode(t *testing.T) {
	_, err := gameWithout(t, nil, "characters")
	want := `no plugin defines the "characters" mode, which runs after a player logs in to choose or create their character. Add "characters" back to builtins in dragon.toml, or define characters in game/modes.lua and have it call session:play(character).`
	if err == nil || err.Error() != want {
		t.Errorf("error = %v, want %q", err, want)
	}
}

func TestModeErrors(t *testing.T) {
	g := startGame(t, fstest.MapFS{
		"plugin.lua": file(`return { name = "game" }`),
		"commands.lua": file(`
			return {
				go = { forms = { { "go <where>", function(actor, args)
					game.session(actor):push_mode(args.where, { fn = function() end })
				end } } },
				pop = { execute = function(actor) game.session(actor):pop_mode() end },
				ask = { execute = function(actor) game.session(actor):prompt({ text = "?", choices = { "a" } }) end },
			}
		`),
		"modes.lua": file(`return { waiting = { input = function() end } }`),
	})

	alice := connect(t, g)
	alice.login("Alice")

	alice.send("go wating")
	alice.expect(`there's no mode "wating". Define it in a plugin's modes.lua. Did you mean "waiting"?`)
	alice.send("go waiting")
	alice.expect("state.fn is a function. A mode's state must be plain data so it survives a reload")
	alice.send("pop")
	alice.expect("the session isn't in a mode; pop_mode ends a mode started with push_mode")
	alice.send("ask")
	alice.expect("choices need an input mode to answer them.")
}

func TestReloadEndsRemovedModes(t *testing.T) {
	files := fstest.MapFS{
		"plugin.lua": file(`return { name = "game" }`),
		"commands.lua": file(`
			return { wait = { execute = function(actor) game.session(actor):push_mode("waiting") end } }
		`),
		"modes.lua": file(`return { waiting = { input = function(session) session:send("Still waiting.") end } }`),
	}
	g := startGame(t, files)

	alice := connect(t, g)
	alice.login("Alice")
	alice.send("wait")
	alice.send("hello")
	alice.expect("Still waiting.")

	files["modes.lua"] = file(`return {}`)
	g.Reload()
	alice.expect("The game was updated, and what you were doing has ended.")
	alice.send("say free")
	alice.expect(`You say, "free"`)
}

func TestModesFileErrors(t *testing.T) {
	tests := []struct{ name, source, want string }{
		{"bad name", `return { ["Bad-Name"] = {} }`, `game/modes.lua: mode "Bad-Name" isn't a valid mode name.`},
		{"not a table", `return { waiting = true }`, `game/modes.lua: mode "waiting" must be a table such as { input = function(session, line, state) ... end }, not a boolean.`},
		{"unknown field", `return { waiting = { inptu = function() end } }`, `game/modes.lua: mode "waiting" has an unknown field "inptu". Did you mean "input"?`},
		{"handler not a function", `return { waiting = { enter = "hi" } }`, `game/modes.lua: mode "waiting": enter must be function(session, state) ... end, not a string.`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := newGame(t, fstest.MapFS{
				"plugin.lua": file(`return { name = "game" }`),
				"modes.lua":  file(tt.source),
			})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %v, want it to contain %q", err, tt.want)
			}
		})
	}
}

func TestCancelledCreationDisconnects(t *testing.T) {
	g := startGame(t, fstest.MapFS{
		"plugin.lua": file(`return { name = "game" }`),
		"hooks.lua": file(`
			return { character_steps = function() return false, "The realm is closed to newcomers." end }
		`),
	})

	alice := connect(t, g)
	alice.expect("By what name")
	alice.send("alice")
	alice.send("yes")
	alice.send("secret pass")
	alice.send("secret pass")
	alice.expect("The realm is closed to newcomers.")
	alice.expect("You can't create a character right now.")
	select {
	case <-alice.conn.closed:
	case <-time.After(2 * time.Second):
		t.Fatal("the session stayed open")
	}
}
