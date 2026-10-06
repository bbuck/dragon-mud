package game

import (
	"log/slog"
	"maps"
	"strings"
	"testing"
	"testing/fstest"

	"bbuck.dev/dragon-mud/auth"
	"bbuck.dev/dragon-mud/plugin"
	"bbuck.dev/dragon-mud/scripting"
	"bbuck.dev/dragon-mud/scripting/lua"
)

// localPlugin is a plugin in game/plugins/name with the given manifest
// settings after its name.
func localPlugin(name, manifest string, files fstest.MapFS) plugin.Source {
	files = maps.Clone(files)
	if files == nil {
		files = fstest.MapFS{}
	}
	files["plugin.toml"] = file("name = \"" + name + "\"\n" + manifest)

	return plugin.Source{Origin: "game/plugins/" + name, Files: files}
}

// newGameWithPlugins is newGame with local plugins loaded between the
// built-ins and the game's own plugin.
func newGameWithPlugins(t *testing.T, gameFiles fstest.MapFS, locals ...plugin.Source) (*Game, error) {
	t.Helper()

	all := sources(t, nil)
	all = append(all, locals...)
	if gameFiles != nil {
		all = append(all, plugin.Source{Origin: "game", Files: withInit(gameFiles), Game: true})
	}

	return New(t.Context(), Options{
		Name:      "Test Realm",
		NewEngine: func() scripting.Engine { return lua.New() },
		Plugins:   all,
		Store:     openStore(t),
		Hasher:    auth.NewHasher(cheapParams, 4),
		Log:       slog.New(slog.DiscardHandler),
	})
}

func startGameWithPlugins(t *testing.T, gameFiles fstest.MapFS, locals ...plugin.Source) *Game {
	t.Helper()

	g, err := newGameWithPlugins(t, gameFiles, locals...)
	if err != nil {
		t.Fatal(err)
	}
	runGame(t, g)

	return g
}

// The game imports any API a loaded plugin provides, with no manifest.
func TestGameImportsChat(t *testing.T) {
	g := startGame(t, fstest.MapFS{
		"lua/commands.lua": file(`
			local chat = require("@dragon:chat")
			return {
				greet = { execute = function(actor) chat.say(actor, "Well met!") end },
				bow = { execute = function(actor)
					local ok, reason = chat.emote(actor, "bows.")
					if not ok then actor:send("Stopped: " .. reason) end
				end },
			}
		`),
		"lua/handlers.lua": file(`
			return {
				["dragon:before_emote"] = function(event)
					if event.action == "bows." then return false, "No bowing." end
				end,
			}
		`),
	})

	alice := connect(t, g)
	alice.login("alice")
	bob := connect(t, g)
	bob.login("bob")
	alice.expect("Bob has arrived.")

	bob.send("greet")
	bob.expect(`You say, "Well met!"`)
	alice.expect(`Bob says, "Well met!"`)

	bob.send("bow")
	bob.expect("Stopped: No bowing.")
}

// A plugin imports what it depends on, even from a plugin that loads after
// it: the provider's init.lua runs first.
func TestPluginImportsADependency(t *testing.T) {
	mapping := localPlugin("mapping", "[depends]\nrooms = \"^1.2\"", fstest.MapFS{
		"init.lua": file(`return { commands = require("commands") }`),
		"lua/commands.lua": file(`
			local rooms = require("@rooms")
			return { where = { execute = function(actor) actor:send("You're in " .. rooms.name() .. ".") end } }
		`),
	})
	rooms := localPlugin("grid-rooms", "[provides]\nrooms = \"1.3\"", fstest.MapFS{
		"init.lua":          file(`return { api = "rooms_api" }`),
		"lua/rooms_api.lua": file(`return { name = function() return "the grid" end }`),
	})

	g := startGameWithPlugins(t, nil, mapping, rooms)
	alice := connect(t, g)
	alice.login("alice")
	alice.send("where")
	alice.expect("You're in the grid.")
}

// An optional dependency no plugin provides imports as nil.
func TestOptionalDependencyMayBeMissing(t *testing.T) {
	sky := localPlugin("sky", "[depends]\nweather = { version = \"^1.0\", optional = true }", fstest.MapFS{
		"init.lua": file(`return { commands = require("commands") }`),
		"lua/commands.lua": file(`
			local weather = require("@weather")
			return { sky = { execute = function(actor)
				actor:send(weather and weather.now() or "The sky is clear.")
			end } }
		`),
	})

	g := startGameWithPlugins(t, nil, sky)
	alice := connect(t, g)
	alice.login("alice")
	alice.send("sky")
	alice.expect("The sky is clear.")
}

// A plugin may provide a dragon: API in place of the built-in that does,
// and what imports it gets the replacement.
func TestPluginReplacesABuiltinAPI(t *testing.T) {
	var all []plugin.Source
	for _, src := range sources(t, nil) {
		if src.Origin != "chat" {
			all = append(all, src)
		}
	}
	all = append(all,
		localPlugin("fancy-chat", "[provides]\n\"dragon:chat\" = \"0.1\"", fstest.MapFS{
			"init.lua":    file(`return { api = "api" }`),
			"lua/api.lua": file(`return { say = function(actor, message) actor:send("~ " .. message .. " ~") return true end }`),
		}),
		plugin.Source{Origin: "game", Game: true, Files: withInit(fstest.MapFS{
			"lua/commands.lua": file(`
				local chat = require("@dragon:chat")
				return { greet = { execute = function(actor) chat.say(actor, "hello") end } }
			`),
		})},
	)

	g, err := New(t.Context(), Options{
		Name:      "Test Realm",
		NewEngine: func() scripting.Engine { return lua.New() },
		Plugins:   all,
		Store:     openStore(t),
		Hasher:    auth.NewHasher(cheapParams, 4),
		Log:       slog.New(slog.DiscardHandler),
	})
	if err != nil {
		t.Fatal(err)
	}
	runGame(t, g)

	alice := connect(t, g)
	alice.login("alice")
	alice.send("greet")
	alice.expect("~ hello ~")
}

func TestAPIErrors(t *testing.T) {
	rooms := func(version string) plugin.Source {
		return localPlugin("grid-rooms", "[provides]\n\"johns:rooms\" = \""+version+"\"", fstest.MapFS{
			"init.lua":    file(`return { api = "api" }`),
			"lua/api.lua": file(`return { name = function() return "the grid" end }`),
		})
	}
	importer := func(manifest, imports string) plugin.Source {
		return localPlugin("mapping", manifest, fstest.MapFS{
			"init.lua": file(`local x = require("@` + imports + `") return {}`),
		})
	}

	tests := []struct {
		name   string
		game   fstest.MapFS
		locals []plugin.Source
		want   string
	}{
		{
			"undeclared dependency",
			nil,
			[]plugin.Source{rooms("1.3"), importer("", "johns:rooms")},
			"require(\"@johns:rooms\"): mapping doesn't depend on the johns:rooms API. Add it to mapping's plugin.toml:\n\n[depends]\n\"johns:rooms\" = \"^1.3\"",
		},
		{
			"no provider",
			nil,
			[]plugin.Source{importer("[depends]\nrooms = \"^1.0\"", "rooms")},
			`game/plugins/mapping: mapping depends on the rooms API (^1.0), but no plugin the game loads provides it. Add a plugin that does, or make the dependency optional in plugin.toml: rooms = { version = "^1.0", optional = true }.`,
		},
		{
			"wrong version",
			nil,
			[]plugin.Source{rooms("1.1"), importer("[depends]\n\"johns:rooms\" = \"^1.2\"", "johns:rooms")},
			`game/plugins/mapping: mapping depends on the johns:rooms API ^1.2, but grid-rooms provides johns:rooms 1.1. Use a version of grid-rooms that provides a matching one, or change mapping's [depends] to "johns:rooms" = "^1.1" if it works with 1.1.`,
		},
		{
			"below 1.0 a minor breaks",
			nil,
			[]plugin.Source{rooms("0.3"), importer("[depends]\n\"johns:rooms\" = \"^0.2\"", "johns:rooms")},
			`mapping depends on the johns:rooms API ^0.2, but grid-rooms provides johns:rooms 0.3.`,
		},
		{
			"two providers",
			nil,
			[]plugin.Source{localPlugin("chatter", "[provides]\n\"dragon:chat\" = \"0.1\"", fstest.MapFS{"init.lua": file(`return { api = "api" }`), "lua/api.lua": file(`return {}`)})},
			`dragon:chat (chat) and chatter (game/plugins/chatter) both provide the dragon:chat API, and a game loads one provider for each API. Load only one of them: drop a built-in from builtins in dragon.toml, or remove a plugin from game/plugins/.`,
		},
		{
			"unknown API",
			fstest.MapFS{"init.lua": file(`local c = require("@dragon:chta") return {}`)},
			nil,
			`require("@dragon:chta"): no plugin the game loads provides the dragon:chta API. Did you mean "dragon:chat"? APIs: dragon:chat.`,
		},
		{
			"API without its namespace",
			fstest.MapFS{"init.lua": file(`local c = require("@chat") return {}`)},
			nil,
			`require("@chat"): no plugin the game loads provides the chat API. Did you mean "dragon:chat"? APIs: dragon:chat.`,
		},
		{
			"dependency without its namespace",
			nil,
			[]plugin.Source{importer("[depends]\nchat = \"^0.1\"", "chat")},
			`mapping depends on the chat API (^0.1), but no plugin the game loads provides it. Did you mean "dragon:chat"?`,
		},
		{
			"dragon: is reserved",
			nil,
			[]plugin.Source{localPlugin("magic", "[provides]\n\"dragon:spells\" = \"1.0\"", fstest.MapFS{"init.lua": file(`return { api = "api" }`), "lua/api.lua": file(`return {}`)})},
			`game/plugins/magic: magic provides dragon:spells, but dragon: is reserved for the engine's APIs, and it has none by that name. Name an API for whoever owns its contract, like "magic:spells" = "1.0".`,
		},
		{
			"namespaced APIs in an api table",
			nil,
			[]plugin.Source{localPlugin("rooms", "[provides]\n\"johns:rooms\" = \"1.0\"\n\"johns:exits\" = \"1.0\"", fstest.MapFS{"init.lua": file(`return { api = { ["johns:rooms"] = "rooms" } }`), "lua/rooms.lua": file(`return {}`)})},
			`api: plugin.toml provides johns:exits, but api has no module for it. Add ["johns:exits"] = "..." to api.`,
		},
		{
			"loop",
			nil,
			[]plugin.Source{
				localPlugin("left", "[provides]\nleft = \"1.0\"\n[depends]\nright = \"1.0\"", fstest.MapFS{"init.lua": file(`local r = require("@right") return { api = "api" }`), "lua/api.lua": file(`return {}`)}),
				localPlugin("right", "[provides]\nright = \"1.0\"\n[depends]\nleft = \"1.0\"", fstest.MapFS{"init.lua": file(`local l = require("@left") return { api = "api" }`), "lua/api.lua": file(`return {}`)}),
			},
			`plugins import each other's APIs as they load, in a loop: left → right → left. Import the API inside the function that uses it, not at the top of the file, so it loads when it's called.`,
		},
		{
			"own API while loading",
			nil,
			[]plugin.Source{localPlugin("rooms", "[provides]\nrooms = \"1.0\"", fstest.MapFS{"init.lua": file(`local r = require("@rooms") return { api = "api" }`), "lua/api.lua": file(`return {}`)})},
			`rooms provides the rooms API itself, and it isn't loaded yet. Require its module directly, like require("api").`,
		},
		{
			"provides without api",
			nil,
			[]plugin.Source{localPlugin("rooms", "[provides]\nrooms = \"1.0\"", fstest.MapFS{"init.lua": file(`return {}`)})},
			`game/plugins/rooms: plugin.toml provides rooms, but init.lua has no api. Add api = "api" to its table, naming the module in lua/ that returns the API's functions.`,
		},
		{
			"api without provides",
			nil,
			[]plugin.Source{localPlugin("rooms", "", fstest.MapFS{"init.lua": file(`return { api = "api" }`)})},
			"game/plugins/rooms: api: plugin.toml provides no API. Add one to it, with its version, like:\n\n[provides]\nrooms = \"0.1\"",
		},
		{
			"several APIs need a table",
			nil,
			[]plugin.Source{localPlugin("rooms", "[provides]\nrooms = \"1.0\"\nexits = \"1.0\"", fstest.MapFS{"init.lua": file(`return { api = "api" }`)})},
			`api: plugin.toml provides exits and rooms, so api names a module for each, like api = { exits = "api", ... }.`,
		},
		{
			"api for something not provided",
			nil,
			[]plugin.Source{localPlugin("rooms", "[provides]\nrooms = \"1.0\"", fstest.MapFS{"init.lua": file(`return { api = { room = "api" } }`)})},
			`api.room: plugin.toml doesn't provide room. Did you mean "rooms"? It provides rooms.`,
		},
		{
			"game provides",
			fstest.MapFS{"init.lua": file(`return { api = "api" }`)},
			nil,
			`api: the game's own plugin can't provide an API. Move it to a plugin in game/plugins/, whose plugin.toml says what it provides.`,
		},
		{
			"api module isn't a table",
			nil,
			[]plugin.Source{localPlugin("rooms", "[provides]\nrooms = \"1.0\"", fstest.MapFS{"init.lua": file(`return { api = "api" }`), "lua/api.lua": file(`local x = 1`)})},
			`game/plugins/rooms: the rooms API (lua/api.lua) is a boolean, but it must be a table keyed by name`,
		},
		{
			"bad version",
			nil,
			[]plugin.Source{localPlugin("rooms", "[provides]\nrooms = \"v1\"", nil)},
			`game/plugins/rooms: plugin.toml: provides.rooms: "v1" isn't a version. Use one to three numbers, like "1.2" or "1.2.3".`,
		},
		{
			"bad constraint",
			nil,
			[]plugin.Source{localPlugin("mapping", "[depends]\nrooms = \">= 1.2\"", nil)},
			`plugin.toml: depends.rooms: ">= 1.2" isn't a version constraint. Use "^1.2" for 1.2 up to 2.0, "~1.2" for 1.2 up to 1.3, or "=1.2.3" for exactly 1.2.3.`,
		},
		{
			"unknown dependency setting",
			nil,
			[]plugin.Source{localPlugin("mapping", "[depends]\nrooms = { version = \"^1.2\", optinal = true }", nil)},
			`plugin.toml: depends.rooms: unknown setting "optinal". Did you mean "optional"? A dependency has version and optional.`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := newGameWithPlugins(t, tt.game, tt.locals...)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %v\nwant it to contain %q", err, tt.want)
			}
		})
	}
}
