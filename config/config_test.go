package config

import (
	"os"
	"path/filepath"
	"reflect"
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

func TestLogTargets(t *testing.T) {
	cfg, err := Load(writeConfig(t, `name = "x"`))
	if err != nil {
		t.Fatal(err)
	}
	if want := []Log{{Target: "stderr", Level: "info", Format: "plain"}}; !reflect.DeepEqual(cfg.Log, want) {
		t.Errorf("default logs = %+v, want %+v", cfg.Log, want)
	}

	cfg, err = Load(writeConfig(t, `
[[log]]
target = "stderr"
format = "Pretty"

[[log]]
target = "logs/game.log"
level = "debug"
format = "json"
`))
	if err != nil {
		t.Fatal(err)
	}
	want := []Log{
		{Target: "stderr", Level: "info", Format: "pretty"},
		{Target: "logs/game.log", Level: "debug", Format: "json"},
	}
	if !reflect.DeepEqual(cfg.Log, want) {
		t.Errorf("logs = %+v, want %+v", cfg.Log, want)
	}
}

func TestLogTargetErrors(t *testing.T) {
	tests := []struct {
		name, contents, want string
	}{
		{"no target", "[[log]]\nlevel = \"debug\"",
			`[[log]] #1 has no target. Set target = "stderr", "stdout" or a file path like "logs/game.log".`},
		{"bad level", "[[log]]\ntarget = \"stderr\"\n[[log]]\ntarget = \"x.log\"\nlevel = \"verbose\"",
			`[[log]] #2: level "verbose" isn't a log level. Use debug, info, warn or error.`},
		{"bad format", "[[log]]\ntarget = \"stderr\"\nformat = \"color\"",
			`[[log]] #1: format "color" isn't a log format. Use plain, pretty or json: plain is aligned text, pretty is the same in color, and json is one object per line.`},
		{"same target twice", "[[log]]\ntarget = \"x.log\"\n[[log]]\ntarget = \"x.log\"",
			`[[log]] #2: another [[log]] already writes to "x.log". Use one table per target.`},
		{"unknown key", "[[log]]\ntarget = \"stderr\"\nrenderer = \"json\"", `unknown setting "log.renderer"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Load(writeConfig(t, tt.contents))
			if err == nil || !strings.HasSuffix(err.Error(), tt.want) {
				t.Errorf("Load error = %v\nwant it to end with %q", err, tt.want)
			}
		})
	}
}

func TestDragon(t *testing.T) {
	cfg, err := Load(writeConfig(t, `name = "x"`))
	if err != nil || !cfg.Dragon {
		t.Errorf("dragon defaults to %v (error %v), want true", cfg.Dragon, err)
	}

	cfg, err = Load(writeConfig(t, "name = \"x\"\ndragon = false"))
	if err != nil || cfg.Dragon {
		t.Errorf("dragon = false gave %v (error %v)", cfg.Dragon, err)
	}
}

func TestBuiltins(t *testing.T) {
	cfg, err := Load(writeConfig(t, `name = "x"`))
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"chat", "help", "presence"}; !reflect.DeepEqual(cfg.Builtins, want) {
		t.Errorf("default builtins = %q, want every built-in %q", cfg.Builtins, want)
	}

	cfg, err = Load(writeConfig(t, `builtins = ["presence", "help"]`))
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"presence", "help"}; !reflect.DeepEqual(cfg.Builtins, want) {
		t.Errorf("builtins = %q, want %q", cfg.Builtins, want)
	}

	cfg, err = Load(writeConfig(t, `builtins = []`))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Builtins) != 0 {
		t.Errorf("builtins = %q, want none", cfg.Builtins)
	}
}

func TestBuiltinsErrors(t *testing.T) {
	tests := []struct {
		name, contents, want string
	}{
		{
			"unknown", `builtins = ["chta"]`,
			`builtins: "chta" isn't a built-in plugin. Did you mean "chat"? The built-ins are chat, help and presence.`,
		},
		{
			"prefixed", `builtins = ["dragon:chat"]`,
			`builtins: write "chat", not "dragon:chat". Everything in builtins is a built-in plugin, so the "dragon:" prefix is implied.`,
		},
		{"twice", `builtins = ["chat", "help", "chat"]`, `builtins: "chat" is listed twice. Remove one.`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Load(writeConfig(t, tt.contents))
			if err == nil || !strings.HasSuffix(err.Error(), tt.want) {
				t.Errorf("Load error = %v, want it to end with %q", err, tt.want)
			}
		})
	}
}
