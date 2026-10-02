package store

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"

	"bbuck.dev/dragon-mud/world"
)

func open(t *testing.T, path string) *Store {
	t.Helper()

	s, err := Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })

	return s
}

func load(t *testing.T, s *Store) *world.World {
	t.Helper()

	records, err := s.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	w, err := world.Load(records)
	if err != nil {
		t.Fatal(err)
	}

	return w
}

func save(t *testing.T, s *Store, w *world.World) {
	t.Helper()

	if err := s.Save(context.Background(), w.Changes()); err != nil {
		t.Fatal(err)
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func TestRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data", "world.db")
	s := open(t, path)

	w := load(t, s)
	room, rock := w.Create(), w.Create()
	must(t, room.SetName("void"))
	must(t, rock.SetParent(room))
	must(t, rock.MoveTo(room))
	must(t, rock.Set("weight", 3))
	must(t, rock.Set("density", 2.5))
	must(t, rock.Set("tags", []string{"grey"}))
	must(t, rock.Set("stats", map[string]any{"hp": 10, "nested": []any{1, "two", nil, true}}))
	save(t, s, w)
	s.Close()

	w2 := load(t, open(t, path))
	if w2.Len() != 2 {
		t.Fatalf("loaded %d objects, want 2", w2.Len())
	}

	r2, ok := w2.Named("void")
	if !ok {
		t.Fatal("room not found by name")
	}
	rock2, _ := w2.Get(rock.ID())
	if rock2.Location() != r2 || rock2.Parent() != r2 {
		t.Error("parent or location not saved")
	}
	for _, name := range rock.Properties() {
		want, _ := rock.GetOwn(name)
		got, _ := rock2.GetOwn(name)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s = %#v, want %#v", name, got, want)
		}
	}
}

func TestSaveChanges(t *testing.T) {
	s := open(t, filepath.Join(t.TempDir(), "world.db"))

	w := load(t, s)
	a, b, c := w.Create(), w.Create(), w.Create()
	must(t, a.SetName("first"))
	must(t, b.SetName("second"))
	must(t, c.MoveTo(a))
	must(t, a.Set("gone", true))
	save(t, s, w)

	// Swap names, drop a property and destroy an object others point at.
	must(t, a.SetName("temp"))
	must(t, b.SetName("first"))
	must(t, a.SetName("second"))
	must(t, a.Delete("gone"))
	must(t, b.SetParent(a))
	w.Destroy(a)
	save(t, s, w)

	w2 := load(t, s)
	if w2.Len() != 2 {
		t.Fatalf("loaded %d objects, want 2", w2.Len())
	}
	if o, _ := w2.Named("first"); o == nil || o.ID() != b.ID() {
		t.Error("swapped name not saved")
	}
	if _, ok := w2.Named("second"); ok {
		t.Error("destroyed object's name still saved")
	}
	c2, _ := w2.Get(c.ID())
	if c2.Location() != nil {
		t.Error("contents of a destroyed object weren't moved out")
	}
}

func TestNewerSchemaRefused(t *testing.T) {
	path := filepath.Join(t.TempDir(), "world.db")
	s := open(t, path)
	if _, err := s.db.Exec("PRAGMA user_version = 999"); err != nil {
		t.Fatal(err)
	}
	s.Close()

	if _, err := Open(context.Background(), path); err == nil {
		t.Error("opened a database from a newer engine")
	}
}
