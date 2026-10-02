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

	run(t, e, `test.record(nil, true, 1.5, "hi", {1, 2}, {a = 1, [3] = "x"})`)

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
		{"int slice", []int{4, 5}, `local r = test.record(); assert(#r == 2 and r[1] == 4 and r[2] == 5)`},
		{"map", map[string]int{"hp": 34}, `assert(test.record().hp == 34)`},
		{"nested", map[string]any{"list": []string{"a"}}, `assert(test.record().list[1] == "a")`},
		{"nil", nil, `assert(test.record() == nil)`},
		{"uint", uint8(7), `assert(test.record() == 7)`},
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

	run(t, e, `assert(game.name == "DragonMUD" and game.max_level == 50)`)
}

func TestLoadRejectsNameInUse(t *testing.T) {
	e := New()
	defer e.Close()

	if err := e.Load(scripting.Module{Name: "string"}); err == nil {
		t.Error("loading a module named \"string\" should fail")
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
		local ok, msg = pcall(die.roll, 6)
		assert(not ok)
		assert(string.find(msg, "die.roll: argument #1: expected string, got number", 1, true), msg)
	`)

	// ...and otherwise surfaces from Run.
	err = e.Run(context.Background(), "plugin.lua", `die.roll()`)
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

	run(t, e, `test.record(function(a, b) return a + b end)`)

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
	run(t, newEngine(t, &got, nil), `test.record(function() end)`)
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

	run(t, e, `test.record(function() return "hello" end)`)
	fn, _ := got.Function(0)

	if err := e.Load(scripting.Module{
		Name:   "saved",
		Values: map[string]any{"fn": fn},
	}); err != nil {
		t.Fatal(err)
	}

	run(t, e, `assert(saved.fn() == "hello")`)
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

	err = e.Run(context.Background(), "cycle.lua", `local t = {}; t.self = t; test.take(t)`)
	if err == nil || !strings.Contains(err.Error(), "nested more than") {
		t.Errorf("Run error = %v, want nesting error", err)
	}
}

func TestSandbox(t *testing.T) {
	e := New()
	defer e.Close()

	for _, name := range []string{"os", "io", "debug", "package", "dofile", "loadfile", "require"} {
		t.Run(name, func(t *testing.T) {
			run(t, e, `assert(`+name+` == nil, "`+name+` should not be available")`)
		})
	}

	run(t, e, `assert(string.upper("ok") == "OK" and math.floor(1.5) == 1 and table.concat({"a", "b"}) == "ab")`)
}
