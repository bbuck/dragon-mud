package store

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"bbuck.dev/dragon-mud/world"
)

func TestAccounts(t *testing.T) {
	ctx := context.Background()
	s := open(t, filepath.Join(t.TempDir(), "world.db"))

	a, err := s.CreateAccount(ctx, "Alice", "hash")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateAccount(ctx, "ALICE", "other"); !errors.Is(err, ErrNameTaken) {
		t.Errorf("duplicate account error = %v, want ErrNameTaken", err)
	}

	got, ok, err := s.Account(ctx, "alice")
	if err != nil || !ok {
		t.Fatalf("Account = %v, %v", ok, err)
	}
	if !reflect.DeepEqual(got, a) {
		t.Errorf("Account = %+v, want %+v", got, a)
	}
	if _, ok, _ := s.Account(ctx, "bob"); ok {
		t.Error("found an account that doesn't exist")
	}

	w := world.New()
	first, second := w.Create(), w.Create()
	save(t, s, w)
	must(t, s.AddCharacter(ctx, a.ID, second.ID()))
	must(t, s.AddCharacter(ctx, a.ID, first.ID()))

	ids, err := s.Characters(ctx, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(ids, []world.ID{second.ID(), first.ID()}) {
		t.Errorf("Characters = %v, want oldest first", ids)
	}

	// Destroying a character's object removes it from the account.
	w.Destroy(second)
	save(t, s, w)
	if ids, _ := s.Characters(ctx, a.ID); !reflect.DeepEqual(ids, []world.ID{first.ID()}) {
		t.Errorf("after destroy, Characters = %v", ids)
	}
}
