package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
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
