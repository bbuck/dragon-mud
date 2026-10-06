package world

import (
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestInheritance(t *testing.T) {
	w := New()
	sword := w.Create()
	if err := sword.Set("damage", "1d8"); err != nil {
		t.Fatal(err)
	}
	if err := sword.Set("weight", 3); err != nil {
		t.Fatal(err)
	}

	blade := w.Create()
	if err := blade.SetParent(sword); err != nil {
		t.Fatal(err)
	}
	blade.Set("damage", "1d10")

	if v, _ := blade.Get("damage"); v != "1d10" {
		t.Errorf("own damage = %v, want 1d10", v)
	}
	if v, _ := blade.Get("weight"); v != int64(3) {
		t.Errorf("inherited weight = %#v, want int64(3)", v)
	}
	if _, ok := blade.GetOwn("weight"); ok {
		t.Error("weight should not be blade's own property")
	}

	blade.Delete("damage")
	if v, _ := blade.Get("damage"); v != "1d8" {
		t.Errorf("damage after delete = %v, want inherited 1d8", v)
	}

	dagger := w.Create()
	must(t, dagger.SetParent(blade))
	switch {
	case !dagger.IsA(sword), !dagger.IsA(blade), !blade.IsA(sword):
		t.Error("IsA missed an ancestor")
	case !sword.IsA(sword):
		t.Error("an object should be itself")
	case sword.IsA(blade), blade.IsA(dagger):
		t.Error("IsA found an ancestor among descendants")
	}

	if err := sword.SetParent(blade); err == nil {
		t.Error("an object inherited from its own child")
	}
	if err := sword.SetParent(sword); err == nil {
		t.Error("an object inherited from itself")
	}
}

func TestContainment(t *testing.T) {
	w := New()
	room, chest, coin := w.Create(), w.Create(), w.Create()

	must(t, chest.MoveTo(room))
	must(t, coin.MoveTo(chest))

	if coin.Location() != chest || !reflect.DeepEqual(room.Contents(), []*Object{chest}) {
		t.Fatal("containment not recorded")
	}
	if err := room.MoveTo(coin); err == nil {
		t.Error("a room moved inside something it contains")
	}

	must(t, coin.MoveTo(room))
	if len(chest.Contents()) != 0 || !reflect.DeepEqual(room.Contents(), []*Object{chest, coin}) {
		t.Errorf("after move: chest has %v, room has %v", chest.Contents(), room.Contents())
	}
}

func TestDestroy(t *testing.T) {
	w := New()
	room, chest, coin, base, child := w.Create(), w.Create(), w.Create(), w.Create(), w.Create()
	must(t, chest.MoveTo(room))
	must(t, coin.MoveTo(chest))
	must(t, chest.SetParent(base))
	must(t, child.SetParent(chest))
	must(t, chest.SetKey("chest"))

	w.Destroy(chest)

	if coin.Location() != room {
		t.Error("contents of a destroyed object should move to its location")
	}
	if child.Parent() != base {
		t.Error("children of a destroyed object should inherit from its parent")
	}
	if _, ok := w.Keyed("chest"); ok {
		t.Error("a destroyed object's key is still taken")
	}
	if err := chest.Set("x", 1); err != ErrDestroyed {
		t.Errorf("Set on destroyed object = %v, want ErrDestroyed", err)
	}
}

func TestKeys(t *testing.T) {
	w := New()
	a, b := w.Create(), w.Create()

	must(t, a.SetKey("tavern"))
	if err := b.SetKey("tavern"); err == nil {
		t.Error("two objects share a key")
	}
	if err := b.SetKey("Bad Key"); err == nil {
		t.Error("invalid key accepted")
	}

	must(t, a.SetKey("inn"))
	must(t, b.SetKey("tavern"))
	if o, _ := w.Keyed("tavern"); o != b {
		t.Error("rekeyed object still holds its old key")
	}
}

func TestPropertyValues(t *testing.T) {
	w := New()
	o := w.Create()

	must(t, o.Set("tags", []string{"shiny", "heavy"}))
	must(t, o.Set("stats", map[string]any{"str": 10, "dex": 12.5}))

	tags, _ := o.Get("tags")
	if !reflect.DeepEqual(tags, []any{"shiny", "heavy"}) {
		t.Errorf("tags = %#v", tags)
	}

	// Changing a returned value doesn't change the property.
	tags.([]any)[0] = "dull"
	if again, _ := o.Get("tags"); again.([]any)[0] != "shiny" {
		t.Error("Get returned the stored list, not a copy")
	}

	if err := o.Set("fn", func() {}); err == nil {
		t.Error("a function was stored")
	}
	if err := o.Set("bad name", 1); err == nil {
		t.Error("invalid property name accepted")
	}
}

func TestChangesAndLoad(t *testing.T) {
	w := New()
	room, item := w.Create(), w.Create()
	must(t, room.SetKey("void"))
	must(t, item.MoveTo(room))
	must(t, item.SetParent(room))
	must(t, item.Set("desc", "a pebble"))

	changes := w.Changes()
	if len(changes.Saved) != 2 || len(changes.Destroyed) != 0 {
		t.Fatalf("changes = %+v", changes)
	}
	if !w.Changes().Empty() {
		t.Error("Changes didn't reset")
	}

	loaded, err := Load(changes.Saved)
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.Changes().Empty() {
		t.Error("a freshly loaded world has changes")
	}

	lroom, ok := loaded.Keyed("void")
	if !ok || lroom.ID() != room.ID() {
		t.Fatal("keyed room not loaded")
	}
	litem := lroom.Contents()[0]
	if litem.Parent() != lroom {
		t.Error("parent not loaded")
	}
	if v, _ := litem.Get("desc"); v != "a pebble" {
		t.Errorf("desc = %v", v)
	}

	w.Destroy(item)
	if got := w.Changes(); !reflect.DeepEqual(got.Destroyed, []ID{item.ID()}) {
		t.Errorf("destroyed = %v", got.Destroyed)
	}
}

func TestLoadRejectsBadData(t *testing.T) {
	tests := map[string][]Record{
		"missing parent": {{ID: "a", Parent: "nope"}},
		"parent cycle":   {{ID: "a", Parent: "b"}, {ID: "b", Parent: "a"}},
		"location cycle": {{ID: "a", Location: "a"}},
		"duplicate key":  {{ID: "a", Key: "x"}, {ID: "b", Key: "x"}},
		"duplicate id":   {{ID: "a"}, {ID: "a"}},
	}

	for name, records := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := Load(records); err == nil {
				t.Error("Load succeeded")
			}
		})
	}
}

func TestIDs(t *testing.T) {
	w := New()
	id := w.Create().ID()
	if len(id) != idLength || strings.Trim(string(id), idAlphabet) != "" {
		t.Errorf("id %q isn't %d characters from the alphabet", id, idLength)
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func TestUnsaved(t *testing.T) {
	w := New()
	a, b := w.Create(), w.Create()
	w.Changes()

	must(t, a.Set("x", 1))
	w.Destroy(b)
	failed := w.Changes()

	w.Unsaved(failed)
	if got := w.Changes(); !reflect.DeepEqual(got, failed) {
		t.Errorf("after Unsaved, changes = %+v, want %+v", got, failed)
	}
}

func TestRefs(t *testing.T) {
	w := New()
	room, exit := w.Create(), w.Create()

	must(t, exit.Set("to", room))
	must(t, exit.Set("route", []any{room, Ref{ID: room.ID()}}))
	if v, _ := exit.Get("to"); v != (Ref{ID: room.ID()}) {
		t.Errorf("to = %#v, want a ref to the room", v)
	}

	if err := exit.Set("bad", map[string]any{RefKey: "x"}); err == nil {
		t.Errorf("a map using %q as a key was stored", RefKey)
	}

	// Refs to destroyed objects are kept; readers decide what they mean.
	w.Destroy(room)
	if v, _ := exit.Get("to"); v != (Ref{ID: room.ID()}) {
		t.Errorf("after destroy, to = %#v", v)
	}
}

func TestTypes(t *testing.T) {
	w := New()
	wolf, pup := w.Create(), w.Create()
	if err := pup.SetParent(wolf); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"mobs:mob", "mobs:animal", "mobs:mob"} {
		if err := wolf.AddType(name); err != nil {
			t.Fatal(err)
		}
	}
	if err := pup.AddType("mobs:young"); err != nil {
		t.Fatal(err)
	}

	if got, want := pup.AllTypes(), []string{"mobs:young", "mobs:mob", "mobs:animal"}; !slices.Equal(got, want) {
		t.Errorf("AllTypes = %q, want %q", got, want)
	}
	if got := pup.Types(); !slices.Equal(got, []string{"mobs:young"}) {
		t.Errorf("Types = %q", got)
	}

	// Removing a type the object inherits leaves it.
	if err := pup.RemoveType("mobs:mob"); err != nil {
		t.Fatal(err)
	}
	if err := wolf.RemoveType("mobs:mob"); err != nil {
		t.Fatal(err)
	}
	if got, want := pup.AllTypes(), []string{"mobs:young", "mobs:animal"}; !slices.Equal(got, want) {
		t.Errorf("after removing, AllTypes = %q, want %q", got, want)
	}
}
