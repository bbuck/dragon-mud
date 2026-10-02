package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeConfig(t *testing.T, contents string) string {
	t.Helper()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}

	return dir
}

func TestLoadAppliesDefaults(t *testing.T) {
	cfg, err := Load(writeConfig(t, `name = "Dragon's Rest"`))
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Name != "Dragon's Rest" || !cfg.Telnet.Enabled || cfg.Telnet.Address != ":4000" || !cfg.Web.Client.Enabled {
		t.Errorf("unexpected config: %+v", cfg)
	}
}

func TestLoadErrors(t *testing.T) {
	tests := []struct {
		name, contents, want string
	}{
		{"nothing enabled", "[telnet]\nenabled = false\n[web.client]\nenabled = false", "no way to play"},
		{"unknown setting", "[telnet]\nport = 4000", `unknown setting "telnet.port"`},
		{"bad toml", "name = ", "reading"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Load(writeConfig(t, tt.contents))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("Load error = %v, want it to contain %q", err, tt.want)
			}
		})
	}
}

func TestLoadMissingFile(t *testing.T) {
	if _, err := Load(t.TempDir()); err == nil {
		t.Error("expected an error when dragon.toml is missing")
	}
}
