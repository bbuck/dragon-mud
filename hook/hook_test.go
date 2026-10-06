package hook

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

// fn is a Go stand-in for a script function.
type fn func(args ...any) ([]any, error)

func (f fn) Call(ctx context.Context, args ...any) (any, error) {
	results, err := f(args...)
	if err != nil || len(results) == 0 {
		return nil, err
	}

	return results[0], nil
}

func (f fn) CallAll(ctx context.Context, args ...any) ([]any, error) {
	return f(args...)
}

func (f fn) Source() string { return "" }

// tag returns a handler function that appends name to the payload's "seen"
// list and returns the payload.
func tag(name string) fn {
	return func(args ...any) ([]any, error) {
		payload := args[0].(map[string]any)
		seen, _ := payload["seen"].([]string)
		next := map[string]any{}
		for k, v := range payload {
			next[k] = v
		}
		next["seen"] = append(append([]string{}, seen...), name)
		return []any{next}, nil
	}
}

func handler(hook, plugin string) Handler {
	return Handler{Hook: hook, Plugin: plugin, Fn: tag(plugin)}
}

func order(t *testing.T, r *Registry, name string) []string {
	t.Helper()

	c, ok := r.Chain(name)
	if !ok {
		t.Fatalf("no chain %q", name)
	}

	var plugins []string
	for _, h := range c.Handlers {
		plugins = append(plugins, h.Plugin)
	}

	return plugins
}

var plugins = []string{"dragon:chat", "armor", "shields", "game"}

// decls declares the hooks these tests run.
var decls = []Decl{
	{Name: "hit", Plugin: "dragon:chat", Fields: []Field{
		{Name: "damage", Desc: "how much", Optional: true},
		{Name: "seen", Desc: "who saw it", Optional: true},
	}},
	{Name: "died", Plugin: "dragon:chat"},
	{Name: "nothing", Plugin: "dragon:chat", Fields: []Field{{Name: "a", Desc: "a", Optional: true}}},
}

func TestLoadOrderIsTheDefault(t *testing.T) {
	r, err := New(Config{
		Plugins: plugins,
		Handlers: []Handler{
			handler("hit", "game"),
			handler("hit", "armor"),
			handler("hit", "dragon:chat"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	want := []string{"dragon:chat", "armor", "game"}
	if got := order(t, r, "hit"); !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestBeforeAndAfter(t *testing.T) {
	armor := handler("hit", "armor")
	armor.After = []string{"shields", "not-installed"}
	chat := handler("hit", "dragon:chat")
	chat.After = []string{"game"}

	r, err := New(Config{
		Plugins:  plugins,
		Handlers: []Handler{chat, armor, handler("hit", "shields"), handler("hit", "game")},
	})
	if err != nil {
		t.Fatal(err)
	}

	want := []string{"shields", "armor", "game", "dragon:chat"}
	if got := order(t, r, "hit"); !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	c, _ := r.Chain("hit")
	if got := r.Explain(c, c.Handlers[1]); got != "after shields, not-installed (not installed)" {
		t.Errorf("Explain = %q", got)
	}
}

func TestCycleIsExplained(t *testing.T) {
	armor := handler("hit", "armor")
	armor.After = []string{"shields"}
	shields := handler("hit", "shields")
	shields.After = []string{"armor"}
	// game depends on the cycle but isn't part of it.
	game := handler("hit", "game")
	game.After = []string{"armor"}

	_, err := New(Config{
		Plugins:  plugins,
		Handlers: []Handler{armor, shields, game},
	})
	if err == nil {
		t.Fatal("expected an error")
	}

	for _, want := range []string{
		`hook "hit"`,
		"go in a circle",
		"armor says after shields",
		"shields says after armor",
		`the game's events.wiring: ["hit"] = { order = {`,
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q doesn't mention %q", err, want)
		}
	}
	if strings.Contains(err.Error(), "game says") {
		t.Errorf("error %q includes a handler outside the cycle", err)
	}
}

func TestWiringOrderBreaksCycles(t *testing.T) {
	armor := handler("hit", "armor")
	armor.After = []string{"shields"}
	shields := handler("hit", "shields")
	shields.After = []string{"armor"}

	r, err := New(Config{
		Plugins:  plugins,
		Handlers: []Handler{armor, shields, handler("hit", "game")},
		Wiring:   map[string]Wiring{"hit": {Order: []string{"game", "shields", "armor"}}},
	})
	if err != nil {
		t.Fatal(err)
	}

	want := []string{"game", "shields", "armor"}
	if got := order(t, r, "hit"); !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestWiringDisable(t *testing.T) {
	r, err := New(Config{
		Plugins:  plugins,
		Handlers: []Handler{handler("hit", "armor"), handler("hit", "game")},
		Wiring:   map[string]Wiring{"hit": {Disable: []string{"armor"}}},
	})
	if err != nil {
		t.Fatal(err)
	}

	if got := order(t, r, "hit"); !reflect.DeepEqual(got, []string{"game"}) {
		t.Errorf("got %v", got)
	}
	c, _ := r.Chain("hit")
	if len(c.Disabled) != 1 || c.Disabled[0].Plugin != "armor" {
		t.Errorf("Disabled = %v", c.Disabled)
	}
}

func TestWiringErrors(t *testing.T) {
	handlers := []Handler{handler("hit", "armor"), handler("hit", "game")}

	tests := []struct {
		name   string
		wiring map[string]Wiring
		want   []string
	}{
		{
			"unknown hook",
			map[string]Wiring{"hti": {Disable: []string{"armor"}}},
			[]string{`the game's events.wiring["hti"] is wired`, `no plugin handles "hti"`, `Did you mean "hit"?`},
		},
		{
			"unknown plugin",
			map[string]Wiring{"hit": {Disable: []string{"amror"}}},
			[]string{`disables "amror", which has no hit handler`, `Did you mean "armor"?`, `"armor", "game"`},
		},
		{
			"missing from order",
			map[string]Wiring{"hit": {Order: []string{"game"}}},
			[]string{`leaves out "armor"`, "Add it to order, or to disable"},
		},
		{
			"ordered and disabled",
			map[string]Wiring{"hit": {Order: []string{"game", "armor"}, Disable: []string{"armor"}}},
			[]string{`both orders and disables "armor"`},
		},
		{
			"ordered twice",
			map[string]Wiring{"hit": {Order: []string{"game", "armor", "game"}}},
			[]string{`orders "game" twice`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New(Config{Plugins: plugins, Handlers: handlers, Wiring: tt.wiring})
			if err == nil {
				t.Fatal("expected an error")
			}
			for _, want := range tt.want {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q doesn't mention %q", err, want)
				}
			}
		})
	}
}

func TestOneHandlerPerPlugin(t *testing.T) {
	_, err := New(Config{Plugins: plugins, Handlers: []Handler{handler("hit", "armor"), handler("hit", "armor")}})
	if err == nil || !strings.Contains(err.Error(), `events.handlers["hit"] in armor: two handlers`) {
		t.Errorf("got %v", err)
	}
}

func TestSelfOrderingIsAnError(t *testing.T) {
	armor := handler("hit", "armor")
	armor.Before = []string{"armor"}

	_, err := New(Config{Plugins: plugins, Handlers: []Handler{armor}})
	if err == nil || !strings.Contains(err.Error(), `before = "armor", its own plugin`) {
		t.Errorf("got %v", err)
	}
}

func TestRunPassesThePayloadAlong(t *testing.T) {
	keep := Handler{Hook: "hit", Plugin: "shields", Fn: fn(func(...any) ([]any, error) { return nil, nil })}

	r, err := New(Config{
		Plugins:  plugins,
		Handlers: []Handler{handler("hit", "armor"), keep, handler("hit", "game")},
		Decls:    decls,
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := r.Run(context.Background(), "hit", map[string]any{"damage": 5})
	if err != nil {
		t.Fatal(err)
	}
	if result.Cancelled {
		t.Error("cancelled")
	}
	if got := result.Payload["seen"]; !reflect.DeepEqual(got, []string{"armor", "game"}) {
		t.Errorf("seen = %v", got)
	}
	if result.Payload["damage"] != 5 {
		t.Errorf("damage = %v", result.Payload["damage"])
	}
}

func TestRunCancels(t *testing.T) {
	cancel := Handler{Hook: "hit", Plugin: "armor", Fn: fn(func(...any) ([]any, error) {
		return []any{false, "Your armor holds."}, nil
	})}
	never := Handler{Hook: "hit", Plugin: "game", Fn: fn(func(...any) ([]any, error) {
		t.Error("ran after a cancel")
		return nil, nil
	})}

	r, err := New(Config{Plugins: plugins, Handlers: []Handler{cancel, never}, Decls: decls})
	if err != nil {
		t.Fatal(err)
	}

	result, err := r.Run(context.Background(), "hit", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Cancelled || result.Reason != "Your armor holds." || result.By != "armor" {
		t.Errorf("got %+v", result)
	}
}

func TestRunRejectsBadReturns(t *testing.T) {
	tests := []struct {
		name    string
		returns []any
		want    string
	}{
		{"true", []any{true}, `events.handlers["hit"] in armor: the hit handler returned true. A hook handler returns nothing`},
		{"string", []any{"ok"}, "returned a string"},
		{"reason", []any{false, 3.0}, "cancelled with a number as its reason"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := Handler{Hook: "hit", Plugin: "armor", Fn: fn(func(...any) ([]any, error) { return tt.returns, nil })}
			r, err := New(Config{Plugins: plugins, Handlers: []Handler{h}, Decls: decls})
			if err != nil {
				t.Fatal(err)
			}

			_, err = r.Run(context.Background(), "hit", nil)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("got %v, want %q", err, tt.want)
			}
		})
	}
}

func TestRunWithoutHandlers(t *testing.T) {
	r, err := New(Config{Plugins: plugins, Decls: decls})
	if err != nil {
		t.Fatal(err)
	}

	payload := map[string]any{"a": 1}
	result, err := r.Run(context.Background(), "nothing", payload)
	if err != nil || result.Cancelled || !reflect.DeepEqual(result.Payload, payload) {
		t.Errorf("got %+v, %v", result, err)
	}
}

func TestNotifyKeepsGoingAfterFailures(t *testing.T) {
	var called []string
	failing := Handler{Hook: "died", Plugin: "armor", Fn: fn(func(...any) ([]any, error) {
		called = append(called, "armor")
		return nil, errors.New("boom")
	})}
	ok := Handler{Hook: "died", Plugin: "game", Fn: fn(func(...any) ([]any, error) {
		called = append(called, "game")
		return []any{false, "ignored"}, nil
	})}

	r, err := New(Config{Plugins: plugins, Handlers: []Handler{failing, ok}, Decls: decls})
	if err != nil {
		t.Fatal(err)
	}

	err = r.Notify(context.Background(), "died", nil)
	if err == nil || !strings.Contains(err.Error(), "died handler from armor failed: boom") {
		t.Errorf("got %v", err)
	}
	if !reflect.DeepEqual(called, []string{"armor", "game"}) {
		t.Errorf("called %v", called)
	}
}

func TestEventsMatchTheirDeclaration(t *testing.T) {
	say := Decl{Name: "dragon:before_say", Plugin: "dragon:chat", Fields: []Field{
		{Name: "actor", Desc: "who's speaking"},
		{Name: "message", Desc: "what they say"},
		{Name: "target", Desc: "who they speak to", Optional: true},
	}}
	tooltip := Decl{Name: "tooltip", Fields: []Field{{Name: "entity", Desc: "what"}}, Extra: "data for the template"}
	section := Decl{Name: "section:", Prefix: true, Fields: []Field{{Name: "data", Desc: "the view's data"}}}
	r, err := New(Config{Plugins: plugins, Decls: []Decl{say, tooltip, section, {Name: "booted"}}})
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name  string
		hook  string
		event map[string]any
		want  string
	}{
		{"fits", "dragon:before_say", map[string]any{"actor": 1, "message": "hi"}, ""},
		{"optional given", "dragon:before_say", map[string]any{"actor": 1, "message": "hi", "target": 2}, ""},
		{"unknown field", "dragon:before_say", map[string]any{"actor": 1, "message": "hi", "mesage": "hi"},
			`dragon:before_say: the event has a field "mesage", which dragon:before_say doesn't have. Did you mean "message"? Its fields are actor, message and target. Run dragon hooks dragon:before_say to see what each is for.`},
		{"missing field", "dragon:before_say", map[string]any{"speaker": 1, "message": "hi"},
			`the event has a field "speaker"`},
		{"missing required", "dragon:before_say", map[string]any{"message": "hi"},
			`dragon:before_say: the event is missing actor (who's speaking), which dragon:before_say needs.`},
		{"no fields", "booted", map[string]any{"actor": 1}, `which booted doesn't have. It has no fields.`},
		{"extra allowed", "tooltip", map[string]any{"entity": 1, "colour": "red"}, ""},
		{"prefix", "section:room.exits", map[string]any{"data": 1}, ""},
		{"prefix checks", "section:room.exits", map[string]any{"data": 1, "parts": 2}, `has a field "parts"`},
		{"undeclared", "dragon:before_sya", nil,
			`no plugin declares the hook "dragon:before_sya". Did you mean "dragon:before_say"? The plugin that runs a hook or notification declares it in its events.declare`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := r.Run(context.Background(), tt.hook, tt.event)
			notifyErr := r.Notify(context.Background(), tt.hook, tt.event)
			for _, err := range []error{err, notifyErr} {
				switch {
				case tt.want == "" && err != nil:
					t.Errorf("got %v", err)
				case tt.want != "" && (err == nil || !strings.Contains(err.Error(), tt.want)):
					t.Errorf("got %v\nwant it to contain %q", err, tt.want)
				}
			}
		})
	}
}

func TestHandlersReturnDeclaredFields(t *testing.T) {
	typo := Handler{Hook: "hit", Plugin: "armor", Fn: fn(func(...any) ([]any, error) {
		return []any{map[string]any{"damage": 1, "damge": 2}}, nil
	})}
	r, err := New(Config{Plugins: plugins, Handlers: []Handler{typo}, Decls: decls})
	if err != nil {
		t.Fatal(err)
	}

	_, err = r.Run(context.Background(), "hit", map[string]any{"damage": 1})
	want := `events.handlers["hit"] in armor: the hit handler returned an event that has a field "damge", which hit doesn't have. Did you mean "damage"?`
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Errorf("got %v\nwant it to contain %q", err, want)
	}
}

func TestOneDeclarationPerHook(t *testing.T) {
	_, err := New(Config{Plugins: plugins, Decls: []Decl{
		{Name: "hit", Plugin: "armor"},
		{Name: "hit", Plugin: "shields"},
	}})
	want := `armor and shields both declare "hit". A hook has one declaration, from the plugin that runs it; rename one of them, with its plugin's name in front, like "shields:hit".`
	if err == nil || err.Error() != want {
		t.Errorf("got %v\nwant %q", err, want)
	}
}

func TestUndeclaredHandlers(t *testing.T) {
	r, err := New(Config{Plugins: plugins, Handlers: []Handler{handler("hit", "armor"), handler("mapping:drawn", "game")}, Decls: decls})
	if err != nil {
		t.Fatal(err)
	}
	if got := r.Undeclared(); !reflect.DeepEqual(got, []string{"mapping:drawn"}) {
		t.Errorf("Undeclared() = %v", got)
	}
}
