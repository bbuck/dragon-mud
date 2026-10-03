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
