package scaffold

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"bbuck.dev/dragon-mud/builtin"
	"bbuck.dev/dragon-mud/config"
)

func TestNewCreatesLoadableGame(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "mygame")

	if err := New(dir, Data{Name: `The "Dragon's" Rest`}); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Name != `The "Dragon's" Rest` {
		t.Errorf("name = %q", cfg.Name)
	}
	if !slices.Equal(cfg.Builtins, builtin.Names) {
		t.Errorf("builtins = %q, want every built-in %q", cfg.Builtins, builtin.Names)
	}

	for _, name := range []string{"game/commands.lua", "game/hooks.lua", ".gitignore"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("missing %s: %v", name, err)
		}
	}
}

func TestNewRefusesExistingDirectory(t *testing.T) {
	if err := New(t.TempDir(), Data{Name: "x"}); err == nil {
		t.Error("expected an error for an existing directory")
	}
}

// Generated games indent with tabs.
func TestTemplateIndentsWithTabs(t *testing.T) {
	err := fs.WalkDir(files, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := fs.ReadFile(files, name)
		if err != nil {
			return err
		}
		for i, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, " ") {
				t.Errorf("%s:%d is indented with spaces: %q", name, i+1, line)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
