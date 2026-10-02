package lua

import (
	"context"
	"errors"
	"strings"
	"testing"

	"bbuck.dev/dragon-mud/scripting"
)

// counters is a toy handle type: each key names a counter Go owns.
func counters(values map[string]int) *scripting.Type {
	t := &scripting.Type{Name: "counter"}
	t.Fields = map[string]scripting.Field{
		"name":  func(key any) (any, error) { return key, nil },
		"value": func(key any) (any, error) { return values[key.(string)], nil },
		"broken": func(key any) (any, error) {
			return nil, errors.New("this field is broken")
		},
	}
	t.Methods = map[string]scripting.Method{
		"add": func(key any, args scripting.Args) (any, error) {
			n, err := args.Int(0)
			if err != nil {
				return nil, err
			}
			values[key.(string)] += n
			return values[key.(string)], nil
		},
		"other": func(key any, args scripting.Args) (any, error) {
			name, err := args.String(0)
			if err != nil {
				return nil, err
			}
			return scripting.Handle{Type: t, Key: name}, nil
		},
	}
	t.String = func(key any) string { return "counter " + key.(string) }

	return t
}

func handleEngine(t *testing.T, values map[string]int) (*Engine, *scripting.Type) {
	t.Helper()

	typ := counters(values)
	e := New()
	t.Cleanup(e.Close)

	err := e.Load(scripting.Module{
		Name: "counters",
		Funcs: map[string]scripting.Func{
			"get": func(args scripting.Args) (any, error) {
				name, err := args.String(0)
				if err != nil {
					return nil, err
				}
				return scripting.Handle{Type: typ, Key: name}, nil
			},
			"name_of": func(args scripting.Args) (any, error) {
				h, err := args.Handle(0, typ)
				if err != nil {
					return nil, err
				}
				return h.Key, nil
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	return e, typ
}

func TestHandleFieldsAndMethods(t *testing.T) {
	values := map[string]int{"hits": 1}
	e, _ := handleEngine(t, values)

	result, err := e.Eval(context.Background(), "test", `
		local c = counters.get("hits")
		local after = c:add(2)
		return { name = c.name, value = c.value, after = after, text = tostring(c) }
	`)
	if err != nil {
		t.Fatal(err)
	}

	got := result.(map[string]any)
	if got["name"] != "hits" || got["value"] != 3.0 || got["after"] != 3.0 || got["text"] != "counter hits" {
		t.Errorf("got %v", got)
	}
	if values["hits"] != 3 {
		t.Errorf("hits = %d, want 3", values["hits"])
	}
}

func TestHandleIdentity(t *testing.T) {
	e, _ := handleEngine(t, map[string]int{})

	result, err := e.Eval(context.Background(), "test", `
		local a, b = counters.get("x"), counters.get("x")
		local seen = { [a] = true }
		return { same = a == b, keyed = seen[b] == true, other = a ~= counters.get("y"),
		         via_method = a:other("y") == counters.get("y") }
	`)
	if err != nil {
		t.Fatal(err)
	}

	for name, ok := range result.(map[string]any) {
		if ok != true {
			t.Errorf("%s = %v, want true", name, ok)
		}
	}
}

func TestHandlesCrossTheBoundary(t *testing.T) {
	e, typ := handleEngine(t, map[string]int{})

	name, err := e.Eval(context.Background(), "test", `return counters.name_of(counters.get("z"))`)
	if err != nil || name != "z" {
		t.Errorf("name_of = %v, %v", name, err)
	}

	value, err := e.Eval(context.Background(), "test", `return counters.get("z")`)
	if err != nil {
		t.Fatal(err)
	}
	if h, ok := value.(scripting.Handle); !ok || h.Type != typ || h.Key != "z" {
		t.Errorf("returned %#v, want the handle", value)
	}

	// A handle passed back into Lua is the same value it was.
	fn, err := e.Eval(context.Background(), "test", `
		local z = counters.get("z")
		return function(h) return h == z end
	`)
	if err != nil {
		t.Fatal(err)
	}
	same, err := fn.(scripting.Function).Call(context.Background(), value)
	if err != nil || same != true {
		t.Errorf("handle from Go == handle in Lua: %v, %v", same, err)
	}
}

func TestHandleErrors(t *testing.T) {
	e, _ := handleEngine(t, map[string]int{})

	tests := map[string]string{
		`counters.get("x"):sub(1)`:                     `counter has no field or method "sub"`,
		`counters.get("x").name = "y"`:                 "can't be assigned",
		`counters.get("x").add(1)`:                     "call it with a colon",
		`counters.get("x"):add("lots")`:                "counter:add: argument #1: expected integer",
		`local _ = counters.get("x").broken`:           "counter.broken: this field is broken",
		`counters.name_of("x")`:                        "expected counter, got string",
		`getmetatable(counters.get("x")).__index = {}`: "attempt to index",
	}

	for source, want := range tests {
		err := e.Run(context.Background(), "test", source)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: error = %v, want %q", source, err, want)
		}
	}
}

func TestHandleKeyMustBeComparable(t *testing.T) {
	e, typ := handleEngine(t, map[string]int{})

	_, err := e.toLua(scripting.Handle{Type: typ, Key: []int{1}}, 0)
	if err == nil {
		t.Error("a handle with an uncomparable key was accepted")
	}
}
