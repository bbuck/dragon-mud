// Package plugin loads plugins: a directory with a plugin.lua manifest and
// files that each return a table for the engine to register.
// See docs/plugins.md.
package plugin

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"regexp"

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

// Manifest is what a plugin's plugin.lua returns.
type Manifest struct {
	Name    string
	Version string
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

// Open reads the manifest in fsys. Built-in plugins get BuiltinPrefix on
// their ID.
func Open(ctx context.Context, engine scripting.Engine, fsys fs.FS, builtin bool) (*Plugin, error) {
	p := &Plugin{files: fsys}

	source, err := p.read("plugin.lua")
	if err != nil {
		return nil, err
	}
	var value any
	if source != "" {
		if value, err = engine.Eval(ctx, "plugin.lua", source); err != nil {
			return nil, err
		}
	}
	if err != nil {
		return nil, err
	}
	if value == nil {
		return nil, errors.New(`plugin.lua not found. Every plugin needs one, returning at least its name: return { name = "myplugin" }`)
	}

	table, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("plugin.lua must return a table, got %s", scripting.TypeName(value))
	}

	name, _ := table["name"].(string)
	if !nameRx.MatchString(name) {
		return nil, fmt.Errorf("plugin.lua: invalid name %q (use lowercase letters, digits, - and _)", name)
	}
	version, _ := table["version"].(string)

	p.Manifest = Manifest{Name: name, Version: version}
	p.ID = name
	if builtin {
		p.ID = BuiltinPrefix + name
	}

	modules, err := fs.Sub(fsys, ModulesDir)
	if err != nil {
		return nil, err
	}
	p.scope = engine.Scope(p.ID+"/"+ModulesDir, modules)

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
