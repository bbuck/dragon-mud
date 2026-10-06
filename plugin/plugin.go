// Package plugin loads plugins: a directory with a plugin.toml manifest
// and files that each return a table for the engine to register.
// See docs/plugins.md.
package plugin

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"regexp"

	"github.com/BurntSushi/toml"

	"bbuck.dev/dragon-mud/command"
	"bbuck.dev/dragon-mud/scripting"
)

// ModulesDir holds a plugin's own Lua modules, loaded with require.
const ModulesDir = "lua"

// LocalDir is the directory in the game's own plugin that holds the game's
// local plugins: plugins that are part of the game, not installed.
const LocalDir = "plugins"

// BuiltinPrefix is prepended to the names of plugins embedded in the engine.
const BuiltinPrefix = "dragon:"

var nameRx = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)

// Manifest is what a plugin's plugin.toml says about it.
type Manifest struct {
	Name    string `toml:"name"`
	Version string `toml:"version"`
}

// Source is where a plugin's files come from.
type Source struct {
	// Origin describes the source in error messages, such as
	// "built-in plugin chat" or a directory path.
	Origin string

	Files fs.FS

	// Builtin marks plugins embedded in the engine.
	Builtin bool

	// Game marks the game's own plugin, the only one that may wire other
	// plugins' hooks.
	Game bool
}

// Plugin is a loaded plugin.
type Plugin struct {
	// ID identifies the plugin, such as "dragon:chat" or "game".
	ID string

	Manifest Manifest

	files fs.FS

	// scope evaluates the plugin's files, with a require for its lua/
	// modules.
	scope scripting.Scope
}

// ManifestFile is the plugin's manifest. It's data, read without running
// any of the plugin's code, so tools can read what a plugin needs before
// trusting it.
const ManifestFile = "plugin.toml"

// GameID is the game's own plugin's ID. The game needs no manifest; its
// settings are in dragon.toml.
const GameID = "game"

// Open reads src's manifest. Built-in plugins get BuiltinPrefix on their
// ID, and the game's own plugin is GameID.
//
// own, if not nil, returns modules the plugin's scripts can require that
// are theirs alone, given its ID.
func Open(ctx context.Context, engine scripting.Engine, src Source, own func(id string) []scripting.Module) (*Plugin, error) {
	fsys := src.Files
	p := &Plugin{files: fsys}

	if src.Game {
		if p.exists(ManifestFile) || p.exists("plugin.lua") {
			file := ManifestFile
			if !p.exists(file) {
				file = "plugin.lua"
			}
			return nil, fmt.Errorf("game/%s: the game's own plugin has no manifest; its settings are in dragon.toml. Delete game/%s.", file, file)
		}
		p.Manifest = Manifest{Name: GameID}
		p.ID = GameID
	} else {
		m, err := p.readManifest()
		if err != nil {
			return nil, err
		}
		p.Manifest = m
		p.ID = m.Name
		if src.Builtin {
			p.ID = BuiltinPrefix + m.Name
		}
	}

	modules, err := fs.Sub(fsys, ModulesDir)
	if err != nil {
		return nil, err
	}
	var mods []scripting.Module
	if own != nil {
		mods = own(p.ID)
	}
	if p.scope, err = engine.Scope(p.ID+"/"+ModulesDir, modules, mods); err != nil {
		return nil, err
	}

	return p, nil
}

// eval evaluates file from the plugin in its scope. A missing file
// returns nil, nil.
func (p *Plugin) eval(ctx context.Context, file, scriptName string) (any, error) {
	source, err := p.read(file)
	if err != nil || source == "" {
		return nil, err
	}

	return p.scope.Eval(ctx, scriptName, source)
}

// read returns file's contents, or "" if the plugin has no such file.
func (p *Plugin) read(file string) (string, error) {
	source, err := fs.ReadFile(p.files, file)
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}

	return string(source), err
}

// exists reports whether the plugin has file.
func (p *Plugin) exists(file string) bool {
	_, err := fs.Stat(p.files, file)

	return err == nil
}

// readManifest reads and checks plugin.toml.
func (p *Plugin) readManifest() (Manifest, error) {
	source, err := p.read(ManifestFile)
	if err != nil {
		return Manifest{}, err
	}
	if source == "" {
		if p.exists("plugin.lua") {
			return Manifest{}, errors.New(`plugin.lua: manifests are plugin.toml now, so tools can read them without running the plugin. Move its name and version there, like name = "mapping" and version = "0.1.0" on their own lines, and delete plugin.lua.`)
		}
		return Manifest{}, fmt.Errorf(`%s not found. Every plugin needs one, with at least its name: name = "mapping".`, ManifestFile)
	}

	var m Manifest
	meta, err := toml.Decode(source, &m)
	if err != nil {
		return Manifest{}, fmt.Errorf("%s: %w", ManifestFile, err)
	}
	if undecoded := meta.Undecoded(); len(undecoded) > 0 {
		key := undecoded[0].String()
		return Manifest{}, fmt.Errorf("%s: unknown setting %q.%s A manifest has name and version.", ManifestFile, key, command.DidYouMean(key, manifestKeys))
	}

	switch {
	case m.Name == "":
		return Manifest{}, fmt.Errorf(`%s needs the plugin's name, like name = "mapping".`, ManifestFile)
	case !nameRx.MatchString(m.Name):
		return Manifest{}, fmt.Errorf("%s: name %q isn't a valid plugin name. Use lowercase letters, digits, - and _, starting with a letter, like \"mapping\".", ManifestFile, m.Name)
	case m.Name == GameID:
		return Manifest{}, fmt.Errorf("%s: name %q is the game's own plugin. Name this plugin for what it does, like \"combat\".", ManifestFile, m.Name)
	}

	return m, nil
}

var manifestKeys = []string{"name", "version"}
