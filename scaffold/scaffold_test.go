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

	for _, name := range []string{"game/init.lua", "game/lua/commands.lua", "game/lua/handlers.lua", "game/tests/tavern_test.lua", ".gitignore"} {
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

// Generated games and plugins indent with tabs.
func TestTemplateIndentsWithTabs(t *testing.T) {
	for _, fsys := range []fs.FS{files, pluginFiles} {
		err := fs.WalkDir(fsys, ".", func(name string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			data, err := fs.ReadFile(fsys, name)
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
}
