package game

import (
	"log/slog"
	"strings"
	"testing"
	"testing/fstest"

	"bbuck.dev/dragon-mud/auth"
	"bbuck.dev/dragon-mud/plugin"
	"bbuck.dev/dragon-mud/scripting"
	"bbuck.dev/dragon-mud/scripting/lua"
)

func TestDuplicatePluginNames(t *testing.T) {
	local := plugin.Source{Origin: "game/plugins/extras", Files: fstest.MapFS{
		"plugin.lua": file(`return { name = "game" }`),
	}}
	game := plugin.Source{Origin: "game", Game: true, Files: fstest.MapFS{
		"plugin.lua": file(`return { name = "game" }`),
	}}

	_, err := New(t.Context(), Options{
		Name:      "Test Realm",
		NewEngine: func() scripting.Engine { return lua.New() },
		Plugins:   append(sources(t, nil), local, game),
		Store:     openStore(t),
		Hasher:    auth.NewHasher(cheapParams, 4),
		Log:       slog.New(slog.DiscardHandler),
	})
	want := `game: plugin.lua names the plugin "game", but game/plugins/extras already has that name. Plugin names must be unique; rename one of them in its plugin.lua.`
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("error = %v, want it to contain %q", err, want)
	}
}

func TestPluginRequiresItsOwnModules(t *testing.T) {
	g := startGame(t, fstest.MapFS{
		"plugin.lua":    file(`return { name = "game" }`),
		"lua/items.lua": file(`return { describe = function(name) return "a shiny " .. name end }`),
		"commands.lua": file(`
			local items = require("items")
			return {
				inspect = { forms = { { "inspect <thing>", function(actor, args)
					actor:send(items.describe(args.thing))
				end } } },
			}
		`),
		"modes.lua": file(`
			local items = require("items")
			return { browsing = { input = function(session, line) session:send(items.describe(line)) end } }
		`),
	})

	alice := connect(t, g)
	alice.login("Alice")

	alice.send("inspect sword")
	alice.expect("a shiny sword")
}

func TestBootedRunsOnceBeforeInput(t *testing.T) {
	g := startGame(t, fstest.MapFS{
		"plugin.lua": file(`return { name = "game" }`),
		"hooks.lua": file(`
			return {
				["dragon:booted"] = function()
					local tavern = world.keyed("tavern") or world.create({ key = "tavern", properties = { boots = 0 } })
					tavern:set("boots", tavern:get("boots") + 1)
				end,
			}
		`),
		"commands.lua": file(`
			return {
				boots = { execute = function(actor)
					actor:send("booted " .. world.keyed("tavern"):get("boots") .. " times")
				end },
			}
		`),
	})

	alice := connect(t, g)
	alice.login("Alice")

	alice.send("boots")
	alice.expect("booted 1 times")
}
