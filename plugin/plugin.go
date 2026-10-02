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
	// "built-in plugin basics" or a directory path.
	Origin string

	Files fs.FS

	// Builtin marks plugins embedded in the engine.
	Builtin bool
}

// Plugin is a loaded plugin.
type Plugin struct {
	// ID identifies the plugin, such as "dragon:basics" or "game".
	ID string

	Manifest Manifest

	files fs.FS
}

// Open reads the manifest in fsys. Built-in plugins get BuiltinPrefix on
// their ID.
func Open(ctx context.Context, engine scripting.Engine, fsys fs.FS, builtin bool) (*Plugin, error) {
	p := &Plugin{files: fsys}

	value, err := p.eval(ctx, engine, "plugin.lua", "plugin.lua")
	if err != nil {
		return nil, err
	}
	if value == nil {
		return nil, errors.New("plugin.lua not found")
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

	return p, nil
}

// eval evaluates file from the plugin. A missing file returns nil, nil.
func (p *Plugin) eval(ctx context.Context, engine scripting.Engine, file, scriptName string) (any, error) {
	source, err := fs.ReadFile(p.files, file)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return engine.Eval(ctx, scriptName, string(source))
}
