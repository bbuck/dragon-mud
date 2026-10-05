package lua

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"testing/fstest"

	"bbuck.dev/dragon-mud/scripting"
)

func file(source string) *fstest.MapFile {
	return &fstest.MapFile{Data: []byte(source)}
}

func TestScopeRequire(t *testing.T) {
	e := New()
	t.Cleanup(e.Close)

	modules := fstest.MapFS{
		"counter.lua":    file(`runs = (runs or 0) + 1; return { runs = function() return runs end }`),
		"items/init.lua": file(`return { find = require("items.find").find }`),
		"items/find.lua": file(`return { find = function(name) return "found " .. name end }`),
		"quiet.lua":      file(`x = 1`),
	}
	s := newScope(t, e, "game/lua", modules, nil)

	got, err := s.Eval(context.Background(), "game/commands.lua", `
		local items = require("items")
		local first, second = require("counter"), require("counter")
		local lazy = function() return require("items.find").find("rope") end
		return {
			find = items.find("sword"),
			same = first == second,
			runs = first.runs(),
			quiet = require("quiet"),
			lazy = lazy(),
		}
	`)
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]any{"find": "found sword", "same": true, "runs": "1", "quiet": true, "lazy": "found rope"}
	for k, v := range want {
		if fmt.Sprint(got.(map[string]any)[k]) != fmt.Sprint(v) {
			t.Errorf("%s = %#v, want %#v", k, got.(map[string]any)[k], v)
		}
	}
}

func TestScopesKeepTheirOwnGlobals(t *testing.T) {
	e := New()
	t.Cleanup(e.Close)

	a := newScope(t, e, "a/lua", fstest.MapFS{}, nil)
	b := newScope(t, e, "b/lua", fstest.MapFS{}, []scripting.Module{{Name: "dragon.mine", Values: map[string]any{"owner": "b"}}})

	if _, err := a.Eval(context.Background(), "a.lua", `shared = "a"`); err != nil {
		t.Fatal(err)
	}
	got, err := b.Eval(context.Background(), "b.lua", `return { shared = shared, string = string.upper("ok"), mine = require("dragon.mine").owner }`)
	if err != nil {
		t.Fatal(err)
	}

	m := got.(map[string]any)
	if m["shared"] != nil {
		t.Errorf("b sees a's global: %v", m["shared"])
	}
	if m["string"] != "OK" {
		t.Errorf("b can't reach the engine's globals: %v", m["string"])
	}
	if m["mine"] != "b" {
		t.Errorf("b's own value = %v", m["mine"])
	}
}

func TestScopeRequireErrors(t *testing.T) {
	cases := map[string]struct {
		modules fstest.MapFS
		source  string
		want    string
	}{
		"missing": {
			modules: fstest.MapFS{"items.lua": file(`return {}`), "items/find.lua": file(`return {}`)},
			source:  `require("itmes")`,
			want:    `require: no module "itmes": looked for game/lua/itmes.lua and game/lua/itmes/init.lua. Modules in game/lua: items, items.find.`,
		},
		"no modules": {
			modules: fstest.MapFS{},
			source:  `require("items")`,
			want:    `There are no modules in game/lua yet; a module is a .lua file there that returns a table.`,
		},
		"bad name": {
			modules: fstest.MapFS{},
			source:  `require("../secrets")`,
			want:    `require: "../secrets" isn't a module name. Name modules by their path under game/lua with dots`,
		},
		"cycle": {
			modules: fstest.MapFS{"a.lua": file(`return require("b")`), "b.lua": file(`return require("a")`)},
			source:  `require("a")`,
			want:    `require: modules require each other in a loop: a → b → a. Move what they share into a module both can require.`,
		},
		"error in module": {
			modules: fstest.MapFS{"broken.lua": file(`error("boom")`)},
			source:  `require("broken")`,
			want:    `game/lua/broken.lua:1: boom`,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e := New()
			t.Cleanup(e.Close)

			_, err := newScope(t, e, "game/lua", tc.modules, nil).Eval(context.Background(), "game/commands.lua", tc.source)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want it to contain %q", err, tc.want)
			}
		})
	}
}

func newScope(t *testing.T, e *Engine, dir string, files fstest.MapFS, modules []scripting.Module) scripting.Scope {
	t.Helper()

	s, err := e.Scope(dir, files, modules)
	if err != nil {
		t.Fatal(err)
	}

	return s
}

func TestEngineModulesAreRequired(t *testing.T) {
	e := New()
	t.Cleanup(e.Close)
	if err := e.Load(scripting.Module{Name: "dragon.world", Values: map[string]any{"size": 3}}); err != nil {
		t.Fatal(err)
	}

	// A plugin's own lua/dragon/world.lua can't shadow the engine's module.
	s := newScope(t, e, "game/lua", fstest.MapFS{"dragon/world.lua": file(`return { size = "mine" }`)}, nil)
	got, err := s.Eval(context.Background(), "game/hooks.lua", `return require("dragon.world").size`)
	if err != nil || fmt.Sprint(got) != "3" {
		t.Errorf("size = %v, %v; want the engine's 3", got, err)
	}

	for source, want := range map[string]string{
		`return world.size`:             `world isn't a global. Add local world = require("dragon.world") at the top of the file.`,
		`return require("dragon.wrld")`: `require: there's no module "dragon.wrld". Modules: dragon.world.`,
	} {
		_, err := s.Eval(context.Background(), "game/hooks.lua", source)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: error = %v, want it to contain %q", source, err, want)
		}
	}

	// Outside any scope, require still reaches the engine's modules.
	got, err = e.Eval(context.Background(), "plugin.lua", `return require("dragon.world").size`)
	if err != nil || fmt.Sprint(got) != "3" {
		t.Errorf("outside a scope: %v, %v", got, err)
	}
}
