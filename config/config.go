// Package config loads and validates a game's dragon.toml.
// See docs/design.md §6.
package config

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"

	"bbuck.dev/dragon-mud/builtin"
	"bbuck.dev/dragon-mud/command"
	"bbuck.dev/dragon-mud/plugin"
)

// FileName is the config file at the root of a game directory.
const FileName = "dragon.toml"

// Config is a game's configuration.
type Config struct {
	Name string `toml:"name"`

	// Dragon is the dragon that greets you when the server starts.
	Dragon bool `toml:"dragon"`

	// Builtins are the built-in plugins the game loads, by name without
	// the "dragon:" prefix. They always load in builtin.Names order,
	// whatever order they're listed in.
	Builtins []string `toml:"builtins"`

	Telnet Telnet `toml:"telnet"`
	Web    Web    `toml:"web"`
	Log    []Log  `toml:"log"`
}

// Log is one place logs go, from a [[log]] table. Each has its own level
// and format.
type Log struct {
	// Target is "stderr", "stdout", or a file path, relative to the game
	// directory.
	Target string `toml:"target"`

	// Level is the lowest level logged: debug, info, warn or error.
	Level string `toml:"level"`

	// Format is plain (aligned text), pretty (the same, colored) or json
	// (one object per line).
	Format string `toml:"format"`
}

// Log targets that aren't files.
const (
	Stderr = "stderr"
	Stdout = "stdout"
)

// Log levels and formats.
var (
	LogLevels  = []string{"debug", "info", "warn", "error"}
	LogFormats = []string{"plain", "pretty", "json"}
)

// Telnet configures the telnet transport.
type Telnet struct {
	Enabled bool   `toml:"enabled"`
	Address string `toml:"address"`
}

// Web configures the web server.
type Web struct {
	Address string `toml:"address"`
	Client  Toggle `toml:"client"`
}

// Toggle turns part of the web server on or off.
type Toggle struct {
	Enabled bool `toml:"enabled"`
}

// Default returns the configuration used for anything dragon.toml leaves
// out.
func Default() Config {
	return Config{
		Name:     "A DragonMUD Game",
		Dragon:   true,
		Builtins: slices.Clone(builtin.Names),
		Telnet:   Telnet{Enabled: true, Address: ":4000"},
		Web:      Web{Address: ":8080", Client: Toggle{Enabled: true}},
	}
}

// defaultLog is where logs go when dragon.toml has no [[log]] tables, and
// fills in what a [[log]] table leaves out.
var defaultLog = Log{Target: Stderr, Level: "info", Format: "plain"}

// Load reads dragon.toml from the game directory dir.
func Load(dir string) (Config, error) {
	cfg := Default()
	path := filepath.Join(dir, FileName)

	meta, err := toml.DecodeFile(path, &cfg)
	if err != nil {
		return Config{}, fmt.Errorf("reading %s: %w", path, err)
	}

	if undecoded := meta.Undecoded(); len(undecoded) > 0 {
		return Config{}, fmt.Errorf("%s: unknown setting %q", path, undecoded[0].String())
	}

	if len(cfg.Log) == 0 {
		cfg.Log = []Log{defaultLog}
	}
	for i := range cfg.Log {
		l := &cfg.Log[i]
		l.Level, l.Format = strings.ToLower(l.Level), strings.ToLower(l.Format)
		if l.Level == "" {
			l.Level = defaultLog.Level
		}
		if l.Format == "" {
			l.Format = defaultLog.Format
		}
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, fmt.Errorf("%s: %w", path, err)
	}

	return cfg, nil
}

// Validate checks that the configuration can run a game.
func (c Config) Validate() error {
	if !c.Telnet.Enabled && !c.Web.Client.Enabled {
		return errors.New("no way to play: enable [telnet] or [web.client]")
	}
	if c.Telnet.Enabled && c.Telnet.Address == "" {
		return errors.New("[telnet] is enabled but has no address")
	}
	if c.Web.Client.Enabled && c.Web.Address == "" {
		return errors.New("[web.client] is enabled but [web] has no address")
	}

	if err := validateBuiltins(c.Builtins); err != nil {
		return err
	}

	for i, l := range c.Log {
		where := fmt.Sprintf("[[log]] #%d", i+1)
		switch {
		case l.Target == "":
			return fmt.Errorf(`%s has no target. Set target = "stderr", "stdout" or a file path like "logs/game.log".`, where)
		case !slices.Contains(LogLevels, l.Level):
			return fmt.Errorf("%s: level %q isn't a log level. Use %s.", where, l.Level, or(LogLevels))
		case !slices.Contains(LogFormats, l.Format):
			return fmt.Errorf("%s: format %q isn't a log format. Use %s: plain is aligned text, pretty is the same in color, and json is one object per line.",
				where, l.Format, or(LogFormats))
		}
		for _, other := range c.Log[:i] {
			if other.Target == l.Target {
				return fmt.Errorf("%s: another [[log]] already writes to %q. Use one table per target.", where, l.Target)
			}
		}
	}

	return nil
}

func validateBuiltins(names []string) error {
	for i, name := range names {
		if bare, ok := strings.CutPrefix(name, plugin.BuiltinPrefix); ok && slices.Contains(builtin.Names, bare) {
			return fmt.Errorf("builtins: write %q, not %q. Everything in builtins is a built-in plugin, so the %q prefix is implied.", bare, name, plugin.BuiltinPrefix)
		}
		if !slices.Contains(builtin.Names, name) {
			return fmt.Errorf("builtins: %q isn't a built-in plugin.%s The built-ins are %s.", name, command.DidYouMean(name, builtin.Names), and(builtin.Names))
		}
		if slices.Contains(names[:i], name) {
			return fmt.Errorf("builtins: %q is listed twice. Remove one.", name)
		}
	}

	return nil
}

// or lists options as "a, b or c".
func or(options []string) string {
	return strings.Join(options[:len(options)-1], ", ") + " or " + options[len(options)-1]
}

// and lists items as "a, b and c".
func and(items []string) string {
	if len(items) == 1 {
		return items[0]
	}

	return strings.Join(items[:len(items)-1], ", ") + " and " + items[len(items)-1]
}
