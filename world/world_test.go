package world

import (
	"reflect"
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
	must(t, chest.SetName("chest"))

	w.Destroy(chest)

	if coin.Location() != room {
		t.Error("contents of a destroyed object should move to its location")
	}
	if child.Parent() != base {
		t.Error("children of a destroyed object should inherit from its parent")
	}
	if _, ok := w.Named("chest"); ok {
		t.Error("a destroyed object's name is still taken")
	}
	if err := chest.Set("x", 1); err != ErrDestroyed {
		t.Errorf("Set on destroyed object = %v, want ErrDestroyed", err)
	}
}

func TestNames(t *testing.T) {
	w := New()
	a, b := w.Create(), w.Create()

	must(t, a.SetName("tavern"))
	if err := b.SetName("tavern"); err == nil {
		t.Error("two objects share a name")
	}
	if err := b.SetName("Bad Name"); err == nil {
		t.Error("invalid name accepted")
	}

	must(t, a.SetName("inn"))
	must(t, b.SetName("tavern"))
	if o, _ := w.Named("tavern"); o != b {
		t.Error("renamed object still holds its old name")
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
	must(t, room.SetName("void"))
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

	lroom, ok := loaded.Named("void")
	if !ok || lroom.ID() != room.ID() {
		t.Fatal("named room not loaded")
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
		"duplicate name": {{ID: "a", Name: "x"}, {ID: "b", Name: "x"}},
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
