package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"bbuck.dev/dragon-mud/hook"
)

func TestPluginSourcesLoadsListedBuiltinsInEngineOrder(t *testing.T) {
	sources, err := pluginSources(t.TempDir(), []string{"presence", "chat"})
	if err != nil {
		t.Fatal(err)
	}

	var got []string
	for _, s := range sources {
		got = append(got, s.Origin)
	}
	if want := []string{"built-in plugin chat", "built-in plugin presence"}; !slices.Equal(got, want) {
		t.Errorf("sources = %q, want %q", got, want)
	}
}

func TestPluginSourcesLoadsLocalPluginsBeforeTheGame(t *testing.T) {
	dir := t.TempDir()
	for _, d := range []string{"game/plugins/zoo", "game/plugins/combat", "game/plugins/.git"} {
		if err := os.MkdirAll(filepath.Join(dir, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "game/plugins/notes.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	sources, err := pluginSources(dir, []string{"chat"})
	if err != nil {
		t.Fatal(err)
	}

	var got []string
	for _, s := range sources {
		origin, _ := filepath.Rel(dir, s.Origin)
		if s.Builtin {
			origin = s.Origin
		}
		got = append(got, origin)
	}
	want := []string{"built-in plugin chat", "game/plugins/combat", "game/plugins/zoo", "game"}
	if !slices.Equal(got, want) {
		t.Errorf("sources = %q, want %q", got, want)
	}
	if !sources[len(sources)-1].Game || sources[1].Game {
		t.Error("only the game's own plugin should be marked Game")
	}
}

func TestShowHook(t *testing.T) {
	r, err := hook.New(hook.Config{
		Plugins: []string{"dragon:chat", "game"},
		Decls: []hook.Decl{{
			Name:   "dragon:before_say",
			Plugin: "dragon:chat",
			Desc:   "Someone is about to say something.",
			Fields: []hook.Field{
				{Name: "actor", Desc: "who's speaking"},
				{Name: "target", Desc: "who to", Optional: true},
			},
		}},
		Handlers: []hook.Handler{{Hook: "dragon:before_say", Plugin: "game"}, {Hook: "dragon:befor_say", Plugin: "game"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	var out strings.Builder
	if err := showHook(&out, r, "dragon:before_say"); err != nil {
		t.Fatal(err)
	}
	want := `dragon:before_say, declared in dragon:chat/events.lua.
  Someone is about to say something.

Fields:
  actor   who's speaking
  target  who to (optional)

Handlers, in the order they run:
  1.  game  game/hooks.lua  load order
`
	if out.String() != want {
		t.Errorf("got\n%s\nwant\n%s", out.String(), want)
	}

	out.Reset()
	if err := showHook(&out, r, "dragon:befor_say"); err != nil {
		t.Fatal(err)
	}
	if want := `No plugin declares dragon:befor_say, so nothing runs it and its handlers never run. It may be misspelled, or from a plugin the game doesn't load. Did you mean "dragon:before_say"?`; !strings.Contains(out.String(), want) {
		t.Errorf("got\n%s\nwant it to contain %s", out.String(), want)
	}

	out.Reset()
	if err := listHooks(&out, r); err != nil {
		t.Fatal(err)
	}
	if want := "dragon:befor_say   game (not declared, so never run)"; !strings.Contains(out.String(), want) {
		t.Errorf("got\n%s\nwant it to contain %s", out.String(), want)
	}
}
