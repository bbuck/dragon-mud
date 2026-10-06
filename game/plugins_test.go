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
	local := func(dir string) plugin.Source {
		return plugin.Source{Origin: "game/plugins/" + dir, Files: fstest.MapFS{"plugin.toml": file(`name = "extras"`)}}
	}

	_, err := New(t.Context(), Options{
		Name:      "Test Realm",
		NewEngine: func() scripting.Engine { return lua.New() },
		Plugins:   append(sources(t, nil), local("extras"), local("more")),
		Store:     openStore(t),
		Hasher:    auth.NewHasher(cheapParams, 4),
		Log:       slog.New(slog.DiscardHandler),
	})
	want := `game/plugins/more: plugin.toml names the plugin "extras", but game/plugins/extras already has that name. Plugin names must be unique; rename one of them in its plugin.toml.`
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("error = %v, want it to contain %q", err, want)
	}
}

func TestManifestErrors(t *testing.T) {
	tests := []struct {
		name  string
		game  bool
		files fstest.MapFS
		want  string
	}{
		{"missing", false, fstest.MapFS{}, `plugins/extra: plugin.toml not found. Every plugin needs one, with at least its name: name = "mapping".`},
		{"old manifest", false, fstest.MapFS{"plugin.lua": file(`return { name = "extra" }`)}, `plugins/extra: plugin.lua: manifests are plugin.toml now`},
		{"no name", false, fstest.MapFS{"plugin.toml": file(`version = "1.0.0"`)}, `plugin.toml needs the plugin's name, like name = "mapping".`},
		{"bad name", false, fstest.MapFS{"plugin.toml": file(`name = "Extra Things"`)}, `plugin.toml: name "Extra Things" isn't a valid plugin name.`},
		{"named game", false, fstest.MapFS{"plugin.toml": file(`name = "game"`)}, `plugin.toml: name "game" is the game's own plugin.`},
		{"unknown setting", false, fstest.MapFS{"plugin.toml": file("name = \"extra\"\nverison = \"1.0.0\"")}, `plugin.toml: unknown setting "verison". Did you mean "version"? A manifest has name and version.`},
		{"not toml", false, fstest.MapFS{"plugin.toml": file(`name = extra`)}, `plugin.toml: toml: line 1`},
		{"game manifest", true, fstest.MapFS{"plugin.toml": file(`name = "game"`)}, `game/plugin.toml: the game's own plugin has no manifest; its settings are in dragon.toml. Delete game/plugin.toml.`},
		{"old game manifest", true, fstest.MapFS{"plugin.lua": file(`return { name = "game" }`)}, `Delete game/plugin.lua.`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := plugin.Source{Origin: "plugins/extra", Files: tt.files, Game: tt.game}
			if tt.game {
				src.Origin = "game"
			}
			_, err := New(t.Context(), Options{
				Name:      "Test Realm",
				NewEngine: func() scripting.Engine { return lua.New() },
				Plugins:   append(sources(t, nil), src),
				Store:     openStore(t),
				Log:       slog.New(slog.DiscardHandler),
			})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %v\nwant it to contain %q", err, tt.want)
			}
		})
	}
}

func TestPluginRequiresItsOwnModules(t *testing.T) {
	g := startGame(t, fstest.MapFS{
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
		"hooks.lua": file(`
						local world = require("dragon.world")
			return {
				["dragon:booted"] = function()
					local tavern = world.keyed("tavern") or world.create({ key = "tavern", properties = { boots = 0 } })
					tavern:set("boots", tavern:get("boots") + 1)
				end,
			}
		`),
		"commands.lua": file(`
						local world = require("dragon.world")
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
