// Package plugin loads plugins: a directory with a plugin.toml manifest
// and files that each return a table for the engine to register.
// See docs/plugins.md.
package plugin

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"strings"

	"github.com/BurntSushi/toml"

	"bbuck.dev/dragon-mud/command"
	"bbuck.dev/dragon-mud/scripting"
)

// ModulesDir holds a plugin's Lua modules, which init.lua and the modules
// themselves load with require, as Neovim's lua/ does.
const ModulesDir = "lua"

// InitFile is the plugin's entry point. It returns a table of what the
// plugin provides, built from its other files with require.
const InitFile = "init.lua"

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
	// plugins' events.
	Game bool
}

// Plugin is a loaded plugin.
type Plugin struct {
	// ID identifies the plugin, such as "dragon:chat" or "game".
	ID string

	Manifest Manifest

	files fs.FS
	game  bool

	// scope evaluates the plugin's files, with a require for its modules.
	scope scripting.Scope

	// exports is what init.lua returned.
	exports map[string]any
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
	p := &Plugin{files: fsys, game: src.Game}

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

// Load runs init.lua and checks the shape of what it returns. A plugin
// without one provides nothing from Lua, such as one that only has views.
//
//	return {
//	  commands = require("commands"),
//	  slots = require("slots"),
//	  modes = require("modes"),
//	  events = {
//	    declare = require("events"),
//	    handlers = require("handlers"),
//	    wiring = require("wiring"), -- the game's own plugin only
//	  },
//	}
func (p *Plugin) Load(ctx context.Context) error {
	if stray := p.rootScripts(); len(stray) > 0 {
		moved := make([]string, len(stray))
		for i, name := range stray {
			moved[i] = ModulesDir + "/" + name
		}
		return fmt.Errorf("%s %s next to %s, where require doesn't look. Move %s to %s, and require %s from %s.",
			andList(stray), isAre(stray), InitFile, them(stray), andList(moved), them(stray), InitFile)
	}

	source, err := p.read(InitFile)
	if err != nil {
		return err
	}
	if source == "" {
		p.exports = map[string]any{}
		if unloaded := p.modules(); len(unloaded) > 0 {
			return fmt.Errorf("there's no %s, so nothing loads %s. A plugin's %s returns what it provides, built from its modules in %s/, like return { commands = require(\"commands\") } for %s/commands.lua.",
				InitFile, andList(unloaded), InitFile, ModulesDir, ModulesDir)
		}
		return nil
	}

	value, err := p.scope.Eval(ctx, p.ID+"/"+InitFile, source)
	if err != nil {
		return err
	}
	exports, err := asTable(InitFile, value, `return { commands = require("commands") }`)
	if err != nil {
		return err
	}
	if exports == nil {
		return fmt.Errorf("%s returns nothing. It returns a table of what the plugin provides, like return { commands = require(\"commands\") }.", InitFile)
	}
	if err := checkKeys(InitFile, exports, exportKeys, map[string]string{
		"hooks":  `hooks are part of events now: events = { handlers = require("handlers") }.`,
		"wiring": `wiring is part of events now: events = { wiring = require("wiring") }.`,
	}); err != nil {
		return err
	}

	events, err := asTable("events", exports["events"], `events = { handlers = require("handlers") }`)
	if err != nil {
		return err
	}
	if err := checkKeys("events", events, eventsKeys, nil); err != nil {
		return err
	}
	if _, ok := events["wiring"]; ok && !p.game {
		return errors.New(`events.wiring: only the game's own plugin can wire events. A plugin orders its handlers with before and after, like ["dragon:before_say"] = { after = { "dragon:chat" }, handler = function(event) ... end }.`)
	}

	p.exports = exports
	return nil
}

// export returns the table at path in what init.lua returned, such as
// "commands" or "events.handlers", or nil if there's none. shape is an
// example of what belongs there, for messages.
func (p *Plugin) export(path, shape string) (map[string]any, error) {
	var value any = p.exports
	for part := range strings.SplitSeq(path, ".") {
		table, _ := value.(map[string]any)
		value = table[part]
	}

	return asTable(path, value, shape)
}

// asTable returns value as a table keyed by name, or nil if it's nil. An
// empty Lua table converts to an empty list, so that's a table too.
func asTable(where string, value any, shape string) (map[string]any, error) {
	switch v := value.(type) {
	case nil:
		return nil, nil
	case map[string]any:
		return v, nil
	case []any:
		if len(v) == 0 {
			return map[string]any{}, nil
		}
		return nil, fmt.Errorf("%s is a list, but it must be a table keyed by name, like %s.", where, shape)
	case bool:
		return nil, fmt.Errorf("%s is a boolean, but it must be a table keyed by name, like %s. A file loaded with require must return its table; one that returns nothing gives true.", where, shape)
	default:
		return nil, fmt.Errorf("%s must be a table keyed by name, like %s, not a %s.", where, shape, scripting.TypeName(value))
	}
}

// rootScripts lists the .lua files at the top of the plugin other than
// init.lua, which require can't reach.
func (p *Plugin) rootScripts() []string {
	entries, _ := fs.ReadDir(p.files, ".")
	var names []string
	for _, e := range entries {
		if !e.IsDir() && path.Ext(e.Name()) == ".lua" && e.Name() != InitFile {
			names = append(names, e.Name())
		}
	}

	return names
}

// modules lists the .lua files in the plugin's lua/ directory.
func (p *Plugin) modules() []string {
	var names []string
	fs.WalkDir(p.files, ModulesDir, func(file string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && path.Ext(file) == ".lua" {
			names = append(names, file)
		}
		return nil
	})

	return names
}

// field is the path to key in the table at base, written as Lua would:
// commands.look, or events.handlers["dragon:before_say"].
func field(base, key string) string {
	if !identRx.MatchString(key) {
		return fmt.Sprintf("%s[%q]", base, key)
	}
	if base == "" {
		return key
	}

	return base + "." + key
}

var identRx = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// exportKeys are the fields init.lua's table may have, and eventsKeys
// those of its events.
var (
	exportKeys = []string{"commands", "slots", "modes", "events"}
	eventsKeys = []string{"declare", "handlers", "wiring"}
)

func andList(items []string) string {
	if len(items) == 1 {
		return items[0]
	}

	return strings.Join(items[:len(items)-1], ", ") + " and " + items[len(items)-1]
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

func isAre(items []string) string {
	if len(items) == 1 {
		return "is"
	}

	return "are"
}

func them(items []string) string {
	if len(items) == 1 {
		return "it"
	}

	return "them"
}
