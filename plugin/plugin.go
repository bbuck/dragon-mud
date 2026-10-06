// Package plugin loads plugins: a directory with a plugin.toml manifest
// and files that each return a table for the engine to register.
// See docs/plugins.md.
package plugin

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"path"
	"regexp"
	"slices"
	"strconv"
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

// apiRx is an API's name: a plugin name, optionally after the namespace
// of whoever owns the API's contract, as in "dragon:rooms".
var apiRx = regexp.MustCompile(`^([a-z][a-z0-9_-]*:)?[a-z][a-z0-9_-]*$`)

// Manifest is what a plugin's plugin.toml says about it.
type Manifest struct {
	Name    string `toml:"name"`
	Version string `toml:"version"`

	// Provides lists the APIs the plugin provides, with their versions,
	// which are independent of the plugin's own.
	Provides map[string]Version `toml:"-"`

	// Depends lists the APIs the plugin uses, and which versions.
	Depends map[string]Dependency `toml:"-"`

	// Capabilities are the system features the plugin may use, such as
	// tasks. Game features (the world, events, messages) need none.
	Capabilities []string `toml:"-"`
}

// Capabilities a plugin can declare. Each grants a system feature: one
// that reaches past the game, into the command line, the database, the
// network or the web server. Code loaded from disk gets only what its
// manifest declares, and world scripts never get any (docs/design.md
// §11). The game's own plugin has every capability, since the game's
// owner wrote it.
const (
	CapTasks        = "tasks"         // export tasks run from the command line
	CapLiveTasks    = "live_tasks"    // tasks that run inside the running game
	CapStore        = "store"         // plugin-scoped storage
	CapSQL          = "sql"           // the database directly
	CapWebClient    = "web_client"    // JavaScript and CSS in the game client
	CapClientEvents = "client_events" // handle what the web client sends
	CapWebRoutes    = "web_routes"    // HTTP routes
	CapAdminUI      = "admin_ui"      // builder UI extensions
)

// AllCapabilities lists every capability, in the order docs give them.
var AllCapabilities = []string{CapTasks, CapLiveTasks, CapStore, CapSQL, CapWebClient, CapClientEvents, CapWebRoutes, CapAdminUI}

// Can reports whether the plugin may use the capability: its manifest
// declares it, or it's the game's own plugin.
func (p *Plugin) Can(capability string) bool {
	return p.game || slices.Contains(p.Manifest.Capabilities, capability)
}

// need returns an error unless the plugin has capability, saying where it
// was needed and what using it is, like "exporting tasks".
func (p *Plugin) need(where, capability, using string) error {
	if p.Can(capability) {
		return nil
	}

	have := "It declares no capabilities yet."
	if len(p.Manifest.Capabilities) > 0 {
		have = "It declares " + andList(p.Manifest.Capabilities) + "."
	}
	return fmt.Errorf("%s: %s needs the %s capability, which %s doesn't declare. %s Add it to %s:\n\ncapabilities = [%s]",
		where, using, capability, ManifestFile, have, ManifestFile, quoteList(append(slices.Clone(p.Manifest.Capabilities), capability)))
}

func quoteList(items []string) string {
	quoted := make([]string, len(items))
	for i, item := range items {
		quoted[i] = strconv.Quote(item)
	}

	return strings.Join(quoted, ", ")
}

// Dependency is an API a plugin uses: rooms = "^1.2", or
// weather = { version = "^1.0", optional = true }.
type Dependency struct {
	Version Constraint

	// Optional dependencies may be missing, and then require gives nil.
	Optional bool
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

	// Origin is where the plugin came from, for messages.
	Origin string

	files fs.FS
	game  bool

	// scope evaluates the plugin's files, with a require for its modules.
	scope scripting.Scope

	// exports is what init.lua returned, or nil before it has run.
	exports map[string]any

	// apis maps each API the plugin provides to the module that has it.
	apis map[string]string
}

// Imports finds the plugin that provides api for from, already loaded, as
// require("@api") in from's scripts asks for it. It returns nil if api is
// an optional dependency no plugin provides.
type Imports func(from *Plugin, api string) (*Plugin, error)

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
// are theirs alone, given its ID. imports, if not nil, finds the plugins
// that provide the APIs its scripts import.
func Open(ctx context.Context, engine scripting.Engine, src Source, own func(id string) []scripting.Module, imports Imports) (*Plugin, error) {
	fsys := src.Files
	p := &Plugin{files: fsys, game: src.Game, Origin: src.Origin}

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
		m, err := ReadManifest(fsys)
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
	var resolve scripting.Imports
	if imports != nil {
		resolve = func(api string) (scripting.Scope, string, error) {
			provider, err := imports(p, api)
			if err != nil || provider == nil {
				return nil, "", err
			}
			module, ok := provider.apis[api]
			if !ok {
				return nil, "", fmt.Errorf("%s hasn't loaded yet, so its %s API isn't ready.", provider.ID, api)
			}
			return provider.scope, module, nil
		}
	}
	if p.scope, err = engine.Scope(p.ID+"/"+ModulesDir, modules, mods, resolve); err != nil {
		return nil, err
	}

	return p, nil
}

// Eval runs source in the plugin's scope, where it can require the
// plugin's modules, and returns what it returns. name identifies it in
// errors.
func (p *Plugin) Eval(ctx context.Context, name, source string) (any, error) {
	return p.scope.Eval(ctx, name, source)
}

// Loaded reports whether init.lua has run.
func (p *Plugin) Loaded() bool {
	return p.exports != nil
}

// CheckAPIs loads the module of each API the plugin provides and checks
// that it's a table, so a broken API fails when the game starts rather
// than when something first imports it.
func (p *Plugin) CheckAPIs(ctx context.Context) error {
	for _, api := range slices.Sorted(maps.Keys(p.apis)) {
		module := p.apis[api]
		value, err := p.scope.Eval(ctx, p.ID+"/"+InitFile, fmt.Sprintf("return require(%q)", module))
		if err != nil {
			return err
		}
		if _, err := asTable(fmt.Sprintf("the %s API (%s/%s.lua)", api, ModulesDir, strings.ReplaceAll(module, ".", "/")), value, "{ say = function(actor, message) ... end }"); err != nil {
			return err
		}
	}

	return nil
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
		if unloaded := p.modules(); len(unloaded) > 0 {
			return fmt.Errorf("there's no %s, so nothing loads %s. A plugin's %s returns what it provides, built from its modules in %s/, like return { commands = require(\"commands\") } for %s/commands.lua.",
				InitFile, andList(unloaded), InitFile, ModulesDir, ModulesDir)
		}
		if _, err := p.readAPIs(nil); err != nil {
			return err
		}
		p.exports = map[string]any{}
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

	apis, err := p.readAPIs(exports["api"])
	if err != nil {
		return err
	}

	p.apis = apis
	p.exports = exports
	return nil
}

// readAPIs checks init.lua's api against the manifest's [provides] and
// returns the module of each API: api = "api" when the plugin provides
// one API, or api = { rooms = "rooms", exits = "exits" } for several.
func (p *Plugin) readAPIs(value any) (map[string]string, error) {
	provides := slices.Sorted(maps.Keys(p.Manifest.Provides))
	if value == nil {
		if len(provides) == 0 {
			return nil, nil
		}
		return nil, fmt.Errorf("%s provides %s, but %s has no api. Add api = %q to its table, naming the module in %s/ that returns the API's functions.",
			ManifestFile, andList(provides), InitFile, "api", ModulesDir)
	}
	if p.game {
		return nil, fmt.Errorf("api: the game's own plugin can't provide an API. Move it to a plugin in game/%s/, whose %s says what it provides.", LocalDir, ManifestFile)
	}
	if len(provides) == 0 {
		return nil, fmt.Errorf("api: %s provides no API. Add one to it, with its version, like:\n\n[provides]\n%s = \"0.1\"", ManifestFile, p.Manifest.Name)
	}

	if module, ok := value.(string); ok {
		if len(provides) > 1 {
			return nil, fmt.Errorf("api: %s provides %s, so api names a module for each, like api = { %s = %q, ... }.", ManifestFile, andList(provides), field("", provides[0]), module)
		}
		return map[string]string{provides[0]: module}, nil
	}

	table, err := asTable("api", value, `api = "api"`)
	if err != nil {
		return nil, fmt.Errorf("%w It names the module in %s/ that returns the API.", err, ModulesDir)
	}
	apis := make(map[string]string, len(table))
	for api, v := range table {
		if _, ok := p.Manifest.Provides[api]; !ok {
			return nil, fmt.Errorf("%s: %s doesn't provide %s.%s It provides %s.", field("api", api), ManifestFile, api, command.DidYouMean(api, provides), andList(provides))
		}
		module, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("%s names the module in %s/ that returns the API, like %q, not a %s.", field("api", api), ModulesDir, "api", scripting.TypeName(v))
		}
		apis[api] = module
	}
	for _, api := range provides {
		if _, ok := apis[api]; !ok {
			return nil, fmt.Errorf("api: %s provides %s, but api has no module for it. Add %s = \"...\" to api.", ManifestFile, api, field("", api))
		}
	}

	return apis, nil
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
	exportKeys = []string{"commands", "slots", "modes", "events", "api", "tasks", "schema"}
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

// ReadManifest reads and checks the plugin.toml in a plugin's files.
func ReadManifest(fsys fs.FS) (Manifest, error) {
	p := &Plugin{files: fsys}
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

	var raw struct {
		Name         string            `toml:"name"`
		Version      string            `toml:"version"`
		Provides     map[string]string `toml:"provides"`
		Depends      map[string]any    `toml:"depends"`
		Capabilities []any             `toml:"capabilities"`
	}
	meta, err := toml.Decode(source, &raw)
	if err != nil {
		return Manifest{}, fmt.Errorf("%s: %w", ManifestFile, err)
	}
	// A dependency's own settings are checked by readDepends.
	undecoded := slices.DeleteFunc(meta.Undecoded(), func(k toml.Key) bool { return len(k) > 2 && k[0] == "depends" })
	if len(undecoded) > 0 {
		key := undecoded[0].String()
		return Manifest{}, fmt.Errorf("%s: unknown setting %q.%s A manifest has %s.", ManifestFile, key, command.DidYouMean(key, manifestKeys), andList(manifestKeys))
	}
	m := Manifest{Name: raw.Name, Version: raw.Version}

	switch {
	case m.Name == "":
		return Manifest{}, fmt.Errorf(`%s needs the plugin's name, like name = "mapping".`, ManifestFile)
	case !nameRx.MatchString(m.Name):
		return Manifest{}, fmt.Errorf("%s: name %q isn't a valid plugin name. Use lowercase letters, digits, - and _, starting with a letter, like \"mapping\".", ManifestFile, m.Name)
	case m.Name == GameID:
		return Manifest{}, fmt.Errorf("%s: name %q is the game's own plugin. Name this plugin for what it does, like \"combat\".", ManifestFile, m.Name)
	}

	if m.Provides, err = readProvides(raw.Provides); err != nil {
		return Manifest{}, fmt.Errorf("%s: %w", ManifestFile, err)
	}
	if m.Depends, err = readDepends(raw.Depends); err != nil {
		return Manifest{}, fmt.Errorf("%s: %w", ManifestFile, err)
	}
	if m.Capabilities, err = readCapabilities(raw.Capabilities); err != nil {
		return Manifest{}, fmt.Errorf("%s: %w", ManifestFile, err)
	}

	return m, nil
}

var manifestKeys = []string{"name", "version", "provides", "depends", "capabilities"}

// readCapabilities checks capabilities: a list of known capability names,
// each once.
func readCapabilities(raw []any) ([]string, error) {
	var caps []string
	for _, v := range raw {
		name, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("capabilities lists capability names, like capabilities = [%q], not %v.", CapTasks, v)
		}
		if !slices.Contains(AllCapabilities, name) {
			return nil, fmt.Errorf("capabilities: there's no capability %q.%s Capabilities: %s.", name, command.DidYouMean(name, AllCapabilities), andList(AllCapabilities))
		}
		if slices.Contains(caps, name) {
			return nil, fmt.Errorf("capabilities lists %q twice. Remove one.", name)
		}
		caps = append(caps, name)
	}

	return caps, nil
}

// readProvides checks [provides]: API names and their versions.
func readProvides(raw map[string]string) (map[string]Version, error) {
	provides := make(map[string]Version, len(raw))
	for api, text := range raw {
		if !apiRx.MatchString(api) {
			return nil, fmt.Errorf("provides: %q isn't a valid API name. Use lowercase letters, digits, - and _, starting with a letter, after a namespace for whoever owns the API, like \"johns:rooms\" = \"1.0\".", api)
		}
		v, err := ParseVersion(text)
		if err != nil {
			return nil, fmt.Errorf("provides.%s: %v", api, err)
		}
		provides[api] = v
	}

	return provides, nil
}

// readDepends checks [depends]: each API with a version constraint, or a
// table of version and optional.
func readDepends(raw map[string]any) (map[string]Dependency, error) {
	const shape = `"johns:rooms" = "^1.2", or "johns:rooms" = { version = "^1.2", optional = true }`

	depends := make(map[string]Dependency, len(raw))
	for api, value := range raw {
		where := "depends." + api
		if !apiRx.MatchString(api) {
			return nil, fmt.Errorf("depends: %q isn't a valid API name. Use lowercase letters, digits, - and _, starting with a letter, after a namespace for whoever owns the API, like %s.", api, shape)
		}

		var dep Dependency
		text, ok := value.(string)
		if table, isTable := value.(map[string]any); isTable {
			for key := range table {
				if key != "version" && key != "optional" {
					return nil, fmt.Errorf("%s: unknown setting %q.%s A dependency has version and optional.", where, key, command.DidYouMean(key, []string{"version", "optional"}))
				}
			}
			if text, ok = table["version"].(string); !ok {
				return nil, fmt.Errorf("%s needs a version, like %s.", where, shape)
			}
			if opt, set := table["optional"]; set {
				if dep.Optional, ok = opt.(bool); !ok {
					return nil, fmt.Errorf("%s: optional is true or false.", where)
				}
			}
		} else if !ok {
			return nil, fmt.Errorf("%s must be a version constraint or a table, like %s.", where, shape)
		}

		var err error
		if dep.Version, err = ParseConstraint(text); err != nil {
			return nil, fmt.Errorf("%s: %v", where, err)
		}
		depends[api] = dep
	}

	return depends, nil
}

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
