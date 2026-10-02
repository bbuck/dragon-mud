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
		Plugins:    plugins,
		Handlers:   []Handler{armor, shields, game},
		WiringFile: "game/wiring.lua",
	})
	if err == nil {
		t.Fatal("expected an error")
	}

	for _, want := range []string{
		`hook "hit"`,
		"go in a circle",
		"armor says after shields",
		"shields says after armor",
		`game/wiring.lua: hooks = { hit = { order = {`,
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
			[]string{"game/wiring.lua: hooks.hti is wired", `no plugin handles "hti"`, `Did you mean "hit"?`},
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
			_, err := New(Config{Plugins: plugins, Handlers: handlers, Wiring: tt.wiring, WiringFile: "game/wiring.lua"})
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
	if err == nil || !strings.Contains(err.Error(), "armor/hooks.lua: two handlers") {
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

	r, err := New(Config{Plugins: plugins, Handlers: []Handler{cancel, never}})
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
		{"true", []any{true}, "armor/hooks.lua: the hit handler returned true. A hook handler returns nothing"},
		{"string", []any{"ok"}, "returned a string"},
		{"reason", []any{false, 3.0}, "cancelled with a number as its reason"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := Handler{Hook: "hit", Plugin: "armor", Fn: fn(func(...any) ([]any, error) { return tt.returns, nil })}
			r, err := New(Config{Plugins: plugins, Handlers: []Handler{h}})
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
	r, err := New(Config{Plugins: plugins})
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

	r, err := New(Config{Plugins: plugins, Handlers: []Handler{failing, ok}})
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
