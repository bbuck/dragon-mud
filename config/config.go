// Package config loads and validates a game's dragon.toml.
// See docs/design.md §6.
package config

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// FileName is the config file at the root of a game directory.
const FileName = "dragon.toml"

// Config is a game's configuration.
type Config struct {
	Name   string `toml:"name"`
	Telnet Telnet `toml:"telnet"`
	Web    Web    `toml:"web"`
}

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
		Name:   "A DragonMUD Game",
		Telnet: Telnet{Enabled: true, Address: ":4000"},
		Web:    Web{Address: ":8080", Client: Toggle{Enabled: true}},
	}
}

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

	return nil
}
