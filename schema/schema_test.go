package schema

import (
	"testing"

	"bbuck.dev/dragon-mud/world"
)

func TestAllows(t *testing.T) {
	tests := []struct {
		kind  Kind
		value any
		want  bool
	}{
		{String, "hi", true},
		{String, int64(1), false},
		{Text, "line\nline", true},
		{Number, int64(1), true},
		{Number, 1.5, true},
		{Integer, int64(1), true},
		{Integer, 2.0, true},
		{Integer, 1.5, false},
		{Boolean, false, true},
		{Boolean, "true", false},
		{Object, world.Ref{ID: "abc"}, true},
		{Object, "abc", false},
		{List, []any{1}, true},
		{List, map[string]any{}, false},
		{Map, map[string]any{"a": 1}, true},
		{Map, []any{}, true},
		{Map, []any{1}, false},
		{Any, []any{1}, true},
	}
	for _, kind := range Kinds {
		tests = append(tests, struct {
			kind  Kind
			value any
			want  bool
		}{kind, nil, true})
	}

	for _, tt := range tests {
		if got := (Field{Kind: tt.kind}).Allows(tt.value); got != tt.want {
			t.Errorf("%s field allows %#v = %v, want %v", tt.kind, tt.value, got, tt.want)
		}
	}
}

func TestDuplicateTypes(t *testing.T) {
	_, _, err := New([]Type{{Name: "item", Plugin: "a"}, {Name: "item", Plugin: "b"}}, nil)
	if err == nil || err.Error() != `a and b both declare the type "item". A type has one declaration, from the plugin that owns it; rename one of them, with its plugin's name in front.` {
		t.Errorf("got %v", err)
	}
}

func TestUnmatchedExtensions(t *testing.T) {
	_, unmatched, err := New([]Type{{Name: "item", Plugin: "a"}}, []Extension{{Type: "room", Plugin: "b"}})
	if err != nil || len(unmatched) != 1 || unmatched[0].Type != "room" {
		t.Errorf("unmatched = %v, %v", unmatched, err)
	}
}

func TestOf(t *testing.T) {
	tags := Field{Name: "tags", Kind: List, Of: &Field{Kind: String}}
	exits := Field{Name: "exits", Kind: Map, Of: &Field{Kind: Object}}
	grid := Field{Name: "grid", Kind: List, Of: &Field{Kind: List, Of: &Field{Kind: Integer}}}

	tests := []struct {
		f     Field
		value any
		want  string
	}{
		{tags, []any{"a", "b"}, ""},
		{tags, []any{"a", int64(3)}, `items:item's tags is a list of strings, so tags[2] can't be the number 3.`},
		{tags, "a", `items:item's tags is a list of strings field, so it can't hold the string "a".`},
		{exits, map[string]any{"north": world.Ref{ID: "x"}}, ""},
		{exits, map[string]any{"north": "hall"}, `items:item's exits is a map of objects, so exits.north can't be the string "hall".`},
		{grid, []any{[]any{int64(1)}, []any{int64(2), 2.5}}, `items:item's grid is a list of lists of integers, so grid[2][2] can't be the number 2.5.`},
		{grid, []any{}, ""},
	}
	for _, tt := range tests {
		err := mismatchError("items:item's "+tt.f.Name, tt.f, tt.value)
		got := ""
		if err != nil {
			got = err.Error()
		}
		if got != tt.want {
			t.Errorf("%s holding %#v:\n got %q\nwant %q", tt.f.Name, tt.value, got, tt.want)
		}
	}
}
