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

	"bbuck.dev/dragon-mud/hook"
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

// Commands loads the commands the plugin's commands.lua returns. A plugin
// without commands.lua has no commands.
func (p *Plugin) Commands(ctx context.Context, engine scripting.Engine) ([]hook.Command, error) {
	value, err := p.eval(ctx, engine, "commands.lua", p.ID+"/commands.lua")
	if err != nil || value == nil {
		return nil, err
	}

	table, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s/commands.lua must return a table, got %s", p.ID, scripting.TypeName(value))
	}

	var commands []hook.Command
	for name, raw := range table {
		def, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%s/commands.lua: %q must be a table", p.ID, name)
		}

		execute, ok := def["execute"].(scripting.Function)
		if !ok {
			return nil, fmt.Errorf("%s/commands.lua: %q needs an execute function", p.ID, name)
		}

		desc, _ := def["desc"].(string)
		override, _ := def["override"].(bool)

		commands = append(commands, hook.Command{
			Name:     name,
			Desc:     desc,
			Plugin:   p.ID,
			Execute:  execute,
			Override: override,
		})
	}

	return commands, nil
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
