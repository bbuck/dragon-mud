package lua

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"bbuck.dev/dragon-mud/scripting"
)

// newEngine returns an engine with a "test" module whose record function
// stores its arguments in *got and returns ret.
func newEngine(t *testing.T, got *scripting.Args, ret any) *Engine {
	t.Helper()

	e := New()
	t.Cleanup(e.Close)

	err := e.Load(scripting.Module{
		Name: "test",
		Funcs: map[string]scripting.Func{
			"record": func(args scripting.Args) (any, error) {
				*got = args
				return ret, nil
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	return e
}

func run(t *testing.T, e *Engine, source string) {
	t.Helper()

	if err := e.Run(context.Background(), "test", source); err != nil {
		t.Fatal(err)
	}
}

func TestArgumentsFromLua(t *testing.T) {
	var got scripting.Args
	e := newEngine(t, &got, nil)

	run(t, e, `local test = require("test"); test.record(nil, true, 1.5, "hi", {1, 2}, {a = 1, [3] = "x"})`)

	want := scripting.Args{
		nil,
		true,
		1.5,
		"hi",
		[]any{1.0, 2.0},
		map[string]any{"a": 1.0, "3": "x"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v\nwant %#v", got, want)
	}
}

func TestReturnValuesToLua(t *testing.T) {
	tests := []struct {
		name   string
		ret    any
		script string
	}{
		{"int slice", []int{4, 5}, `local test = require("test"); local r = test.record(); assert(#r == 2 and r[1] == 4 and r[2] == 5)`},
		{"map", map[string]int{"hp": 34}, `local test = require("test"); assert(test.record().hp == 34)`},
		{"nested", map[string]any{"list": []string{"a"}}, `local test = require("test"); assert(test.record().list[1] == "a")`},
		{"nil", nil, `local test = require("test"); assert(test.record() == nil)`},
		{"uint", uint8(7), `local test = require("test"); assert(test.record() == 7)`},
		{"results", scripting.Results{nil, "why"}, `local test = require("test"); local v, why = test.record(); assert(v == nil and why == "why")`},
		{"no results", scripting.Results{}, `local test = require("test"); assert(select("#", test.record()) == 0)`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got scripting.Args
			run(t, newEngine(t, &got, tt.ret), tt.script)
		})
	}
}

func TestModuleValues(t *testing.T) {
	e := New()
	defer e.Close()

	err := e.Load(scripting.Module{
		Name:   "game",
		Values: map[string]any{"name": "DragonMUD", "max_level": 50},
	})
	if err != nil {
		t.Fatal(err)
	}

	run(t, e, `local game = require("game"); assert(game.name == "DragonMUD" and game.max_level == 50)`)
}

func TestLoadRejectsNameInUse(t *testing.T) {
	e := New()
	defer e.Close()

	if err := e.Load(scripting.Module{Name: "dragon.world"}); err != nil {
		t.Fatal(err)
	}
	if err := e.Load(scripting.Module{Name: "dragon.world"}); err == nil {
		t.Error("loading a second module named \"dragon.world\" should fail")
	}
}

func TestFuncErrorRaisesInLua(t *testing.T) {
	e := New()
	defer e.Close()

	err := e.Load(scripting.Module{
		Name: "die",
		Funcs: map[string]scripting.Func{
			"roll": func(args scripting.Args) (any, error) {
				_, err := args.String(0)
				return nil, err
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// The error can be caught in Lua...
	run(t, e, `
		local die = require("die"); local ok, msg = pcall(die.roll, 6)
		assert(not ok)
		assert(string.find(msg, "die.roll: argument #1: expected string, got number", 1, true), msg)
	`)

	// ...and otherwise surfaces from Run.
	err = e.Run(context.Background(), "plugin.lua", `local die = require("die"); die.roll()`)
	if err == nil || !strings.Contains(err.Error(), "expected string, got nothing") {
		t.Errorf("Run error = %v", err)
	}
}

func TestEvalReturnsValue(t *testing.T) {
	e := New()
	defer e.Close()

	got, err := e.Eval(context.Background(), "commands.lua", `
		return {
			say = { desc = "Say something", execute = function() end },
		}
	`)
	if err != nil {
		t.Fatal(err)
	}

	commands, ok := got.(map[string]any)
	if !ok {
		t.Fatalf("Eval returned %#v, want a map", got)
	}
	say, _ := commands["say"].(map[string]any)
	if say["desc"] != "Say something" {
		t.Errorf("say.desc = %#v", say["desc"])
	}
	if _, ok := say["execute"].(scripting.Function); !ok {
		t.Errorf("say.execute = %#v, want a function", say["execute"])
	}
}

func TestEvalWithoutReturn(t *testing.T) {
	e := New()
	defer e.Close()

	got, err := e.Eval(context.Background(), "empty.lua", `local x = 1`)
	if err != nil || got != nil {
		t.Errorf("Eval = %#v, %v; want nil, nil", got, err)
	}
}

func TestSyntaxError(t *testing.T) {
	e := New()
	defer e.Close()

	err := e.Run(context.Background(), "broken.lua", `if then`)
	if err == nil || !strings.Contains(err.Error(), "broken.lua") {
		t.Errorf("Run error = %v, want it to name the script", err)
	}
}

func TestRunawayScriptIsInterrupted(t *testing.T) {
	e := New()
	defer e.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := e.Run(ctx, "loop.lua", `while true do end`)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Run error = %v, want deadline exceeded", err)
	}

	// The engine is still usable afterwards.
	run(t, e, `assert(1 + 1 == 2)`)
}

func TestCallingLuaFunctionsFromGo(t *testing.T) {
	var got scripting.Args
	e := newEngine(t, &got, nil)

	run(t, e, `local test = require("test"); test.record(function(a, b) return a + b end)`)

	fn, err := got.Function(0)
	if err != nil {
		t.Fatal(err)
	}

	result, err := fn.Call(context.Background(), 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if result != 3.0 {
		t.Errorf("result = %#v, want 3", result)
	}
}

func TestFunctionFromAnotherEngine(t *testing.T) {
	var got scripting.Args
	run(t, newEngine(t, &got, nil), `local test = require("test"); test.record(function() end)`)
	fn, _ := got.Function(0)

	other := New()
	defer other.Close()

	err := other.Load(scripting.Module{Name: "saved", Values: map[string]any{"fn": fn}})
	if err == nil {
		t.Error("loading another engine's function should fail")
	}
}

func TestFunctionPassedBackToLua(t *testing.T) {
	var got scripting.Args
	e := newEngine(t, &got, nil)

	run(t, e, `local test = require("test"); test.record(function() return "hello" end)`)
	fn, _ := got.Function(0)

	if err := e.Load(scripting.Module{
		Name:   "saved",
		Values: map[string]any{"fn": fn},
	}); err != nil {
		t.Fatal(err)
	}

	run(t, e, `local saved = require("saved"); assert(saved.fn() == "hello")`)
}

func TestSelfReferencingTable(t *testing.T) {
	e := New()
	defer e.Close()

	err := e.Load(scripting.Module{
		Name: "test",
		Funcs: map[string]scripting.Func{
			"take": func(scripting.Args) (any, error) { return nil, nil },
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	err = e.Run(context.Background(), "cycle.lua", `local test = require("test"); local t = {}; t.self = t; test.take(t)`)
	if err == nil || !strings.Contains(err.Error(), "nested more than") {
		t.Errorf("Run error = %v, want nesting error", err)
	}
}

func TestSandbox(t *testing.T) {
	e := New()
	defer e.Close()

	for _, name := range []string{"os", "io", "debug", "package", "dofile", "loadfile"} {
		t.Run(name, func(t *testing.T) {
			run(t, e, `assert(`+name+` == nil, "`+name+` should not be available")`)
		})
	}

	// require only reaches the engine's modules, never files or libraries.
	for _, name := range []string{"os", "io", "debug"} {
		if err := e.Run(context.Background(), "test", `require("`+name+`")`); err == nil {
			t.Errorf("require(%q) succeeded", name)
		}
	}

	run(t, e, `assert(string.upper("ok") == "OK" and math.floor(1.5) == 1 and table.concat({"a", "b"}) == "ab")`)
}

func TestCallAllReturnsEveryResult(t *testing.T) {
	e := New()
	t.Cleanup(e.Close)

	fn, err := e.Eval(context.Background(), "test", `return function(x) return nil, "no " .. x end`)
	if err != nil {
		t.Fatal(err)
	}

	results, err := fn.(scripting.Function).CallAll(context.Background(), "luck")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(results, []any{nil, "no luck"}) {
		t.Errorf("CallAll = %#v", results)
	}

	if results, _ := fn.(scripting.Function).CallAll(context.Background(), "x"); len(results) != 2 {
		t.Errorf("second call returned %d results; the stack leaked", len(results))
	}
}

func TestFunctionSource(t *testing.T) {
	e := New()
	defer e.Close()

	v, err := e.Eval(context.Background(), "game/handlers.lua", "local x = 1\n\nreturn function() end")
	if err != nil {
		t.Fatal(err)
	}
	if got := v.(scripting.Function).Source(); got != "game/handlers.lua:3" {
		t.Errorf("Source() = %q", got)
	}
}
