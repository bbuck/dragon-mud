package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEditDependencies(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	original := `name = "Test"

[dependencies]
# Rooms on a grid.
"github.com/johns/rooms" = "^1.8"

[telnet]
enabled = true
`
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	steps := []struct {
		edit func() error
		want string
	}{
		{func() error { return SetDependency(dir, "github.com/usera/mapping", "^0.2") }, `name = "Test"

[dependencies]
# Rooms on a grid.
"github.com/johns/rooms" = "^1.8"
"github.com/usera/mapping" = "^0.2"

[telnet]
enabled = true
`},
		{func() error { return SetDependency(dir, "github.com/johns/rooms", "^2.0") }, `name = "Test"

[dependencies]
# Rooms on a grid.
"github.com/johns/rooms" = "^2.0"
"github.com/usera/mapping" = "^0.2"

[telnet]
enabled = true
`},
		{func() error { return RemoveDependency(dir, "github.com/johns/rooms") }, `name = "Test"

[dependencies]
# Rooms on a grid.
"github.com/usera/mapping" = "^0.2"

[telnet]
enabled = true
`},
	}
	for i, step := range steps {
		if err := step.edit(); err != nil {
			t.Fatal(err)
		}
		got, _ := os.ReadFile(path)
		if string(got) != step.want {
			t.Fatalf("step %d:\n%s\nwant\n%s", i+1, got, step.want)
		}
	}
}

func TestEditAddsTheSection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	if err := os.WriteFile(path, []byte("name = \"Test\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := SetDependency(dir, "github.com/johns/rooms", "^1.8"); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	if want := "name = \"Test\"\n\n[dependencies]\n\"github.com/johns/rooms\" = \"^1.8\"\n"; string(got) != want {
		t.Errorf("got\n%s", got)
	}

	cfg, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Dependencies["github.com/johns/rooms"] != "^1.8" {
		t.Errorf("dependencies = %v", cfg.Dependencies)
	}
}
