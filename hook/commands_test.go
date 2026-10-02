package hook

import (
	"context"
	"strings"
	"testing"
)

type nopFunction struct{}

func (nopFunction) Call(context.Context, ...any) (any, error) { return nil, nil }

func TestRegisterConflict(t *testing.T) {
	c := NewCommands()

	if err := c.Register(Command{Name: "look", Plugin: "dragon:basics", Execute: nopFunction{}}); err != nil {
		t.Fatal(err)
	}

	err := c.Register(Command{Name: "look", Plugin: "game", Execute: nopFunction{}})
	if err == nil || !strings.Contains(err.Error(), "override") {
		t.Fatalf("conflicting register error = %v, want it to mention override", err)
	}

	if err := c.Register(Command{Name: "LOOK", Plugin: "game", Execute: nopFunction{}, Override: true}); err != nil {
		t.Fatal(err)
	}

	cmd, ok := c.Lookup("Look")
	if !ok || cmd.Plugin != "game" {
		t.Errorf("Lookup(look) = %+v, want the game's command", cmd)
	}
}

func TestRegisterValidates(t *testing.T) {
	c := NewCommands()

	if err := c.Register(Command{Name: "two words", Execute: nopFunction{}}); err == nil {
		t.Error("expected an error for a name with spaces")
	}
	if err := c.Register(Command{Name: "say"}); err == nil {
		t.Error("expected an error for a missing execute function")
	}
}

func TestAllIsSorted(t *testing.T) {
	c := NewCommands()
	for _, name := range []string{"who", "say", "look"} {
		_ = c.Register(Command{Name: name, Execute: nopFunction{}})
	}

	var names []string
	for _, cmd := range c.All() {
		names = append(names, cmd.Name)
	}

	if strings.Join(names, ",") != "look,say,who" {
		t.Errorf("All() = %v", names)
	}
}
