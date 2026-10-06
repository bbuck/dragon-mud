package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"bbuck.dev/dragon-mud/event"
	"bbuck.dev/dragon-mud/install"
	"bbuck.dev/dragon-mud/scaffold"
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

func TestShowEvent(t *testing.T) {
	r, err := event.New(event.Config{
		Plugins: []string{"dragon:chat", "game"},
		Decls: []event.Decl{{
			Name:   "dragon:before_say",
			Plugin: "dragon:chat",
			Desc:   "Someone is about to say something.",
			Fields: []event.Field{
				{Name: "actor", Desc: "who's speaking"},
				{Name: "target", Desc: "who to", Optional: true},
			},
		}},
		Handlers: []event.Handler{{Event: "dragon:before_say", Plugin: "game"}, {Event: "dragon:befor_say", Plugin: "game"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	var out strings.Builder
	if err := showEvent(&out, r, "dragon:before_say"); err != nil {
		t.Fatal(err)
	}
	want := `dragon:before_say, declared by dragon:chat.
  Someone is about to say something.

Fields:
  actor   who's speaking
  target  who to (optional)

Handlers, in the order they run:
  1.  game  events.handlers["dragon:before_say"] in game  load order
`
	if out.String() != want {
		t.Errorf("got\n%s\nwant\n%s", out.String(), want)
	}

	out.Reset()
	if err := showEvent(&out, r, "dragon:befor_say"); err != nil {
		t.Fatal(err)
	}
	if want := `No plugin declares dragon:befor_say, so nothing runs it and its handlers never run. It may be misspelled, or from a plugin the game doesn't load. Did you mean "dragon:before_say"?`; !strings.Contains(out.String(), want) {
		t.Errorf("got\n%s\nwant it to contain %s", out.String(), want)
	}

	out.Reset()
	if err := listEvents(&out, r); err != nil {
		t.Fatal(err)
	}
	if want := "dragon:befor_say   game (not declared, so never run)"; !strings.Contains(out.String(), want) {
		t.Errorf("got\n%s\nwant it to contain %s", out.String(), want)
	}
}

// The tests dragon new writes pass against the game it writes.
func TestNewGamesTestsPass(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "mygame")
	if err := scaffold.New(dir, scaffold.Data{Name: "My Game"}); err != nil {
		t.Fatal(err)
	}

	var out strings.Builder
	if err := runTest([]string{"-dir", dir}, &out); err != nil {
		t.Fatalf("%v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "3 passed, 0 failed") {
		t.Errorf("got\n%s", out.String())
	}
}

// Installed plugins load after built-ins and before local plugins, each
// after the plugins whose APIs it depends on.
func TestPluginSourcesLoadsInstalledPlugins(t *testing.T) {
	dir := t.TempDir()
	write := func(name, data string) {
		t.Helper()
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("plugins/aardvark/plugin.toml", "name = \"aardvark\"\n[depends]\n\"zoo:keeper\" = \"^1.0\"\n")
	write("plugins/zookeeper/plugin.toml", "name = \"zookeeper\"\n[provides]\n\"zoo:keeper\" = \"1.0\"\n")
	write("game/plugins/combat/plugin.toml", "name = \"combat\"\n")

	var lock install.Lock
	for _, name := range []string{"aardvark", "zookeeper"} {
		hash, err := install.Hash(os.DirFS(filepath.Join(dir, "plugins", name)))
		if err != nil {
			t.Fatal(err)
		}
		lock.Plugins = append(lock.Plugins, install.Locked{Name: name, Source: "example.com/" + name, Version: "v1.0.0", Hash: hash})
	}
	if err := install.WriteLock(dir, lock); err != nil {
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
	want := []string{"built-in plugin chat", "plugins/zookeeper", "plugins/aardvark", "game/plugins/combat", "game"}
	if !slices.Equal(got, want) {
		t.Errorf("sources = %q, want %q", got, want)
	}

	write("plugins/zookeeper/init.lua", "return {}")
	if _, err := pluginSources(dir, []string{"chat"}); err == nil || !strings.Contains(err.Error(), "plugins/zookeeper has changed") {
		t.Errorf("changed plugin: %v", err)
	}
}

func TestPluginShowsWhatItProvides(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "mygame")
	if err := scaffold.New(dir, scaffold.Data{Name: "My Game"}); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"game/plugins/items/plugin.toml": "name = \"items\"\nversion = \"0.3.0\"\ncapabilities = [\"tasks\"]\n[provides]\n\"johns:items\" = \"1.2\"\n",
		"game/plugins/items/init.lua": `return {
			api = "api",
			commands = { drop = { desc = "Put something down.", forms = { { "drop <thing:object:held>", function() end } } } },
			schema = { types = { ["items:item"] = { desc = "Something to carry.", fields = { weight = { "how heavy", type = "number", default = 1 } } } } },
			events = {
				declare = { ["items:dropped"] = { desc = "Something was dropped. Handlers react.", fields = { actor = "who", thing = "what" } } },
				handlers = { ["dragon:said"] = function() end },
			},
			tasks = { restock = { desc = "Refill the shops.", run = function() end } },
		}`,
		"game/plugins/items/lua/api.lua": `return {}`,
	}
	for name, data := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	var out strings.Builder
	if err := runPlugin([]string{"items", "-dir", dir}, &out); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"items 0.3.0, from ",
		`Provides the johns:items API 1.2.0, as require("@johns:items") (lua/api.lua).`,
		"Capabilities: tasks.",
		"drop <thing:object:held>",
		"items:dropped  (actor, thing)  Something was dropped.",
		"dragon:said  runs 1 of 1",
		"items:item  Something to carry.",
		"number  how heavy (default 1)",
		"items:restock  Refill the shops.",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output doesn't contain %q:\n%s", want, out.String())
		}
	}

	out.Reset()
	if err := runPlugin([]string{"itmes", "-dir", dir}, &out); err == nil || !strings.Contains(err.Error(), `the game loads no plugin called itmes. Did you mean "items"?`) {
		t.Errorf("misspelled plugin: %v", err)
	}
}
