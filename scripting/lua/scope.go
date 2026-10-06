package lua

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"slices"
	"strings"

	glua "github.com/yuin/gopher-lua"

	"bbuck.dev/dragon-mud/scripting"
)

// moduleRx is a module name: dot-separated parts, as in "items.find".
var moduleRx = regexp.MustCompile(`^[A-Za-z0-9_-]+(\.[A-Za-z0-9_-]+)*$`)

// scope is a plugin's environment: a table of its own globals that falls
// back to the engine's, holding a require for the plugin's modules. Chunks
// evaluated in the scope, and every function they define, use it.
type scope struct {
	e     *Engine
	dir   string
	files fs.FS
	env   *glua.LTable

	// modules are the scope's own modules, which take precedence over the
	// engine's.
	modules map[string]*glua.LTable

	// loaded holds each module's value, so a module runs once per scope.
	loaded map[string]glua.LValue

	// loading is the chain of modules being loaded, to catch cycles.
	loading []string

	// imports resolves require("@name").
	imports scripting.Imports
}

// Scope returns a scope whose require loads modules by name, then "a.b"
// from a/b.lua or a/b/init.lua in files.
func (e *Engine) Scope(dir string, files fs.FS, modules []scripting.Module, imports scripting.Imports) (scripting.Scope, error) {
	s := &scope{e: e, dir: dir, files: files, modules: make(map[string]*glua.LTable), loaded: make(map[string]glua.LValue), imports: imports}
	for _, m := range modules {
		if !moduleRx.MatchString(m.Name) {
			return nil, fmt.Errorf("lua: scope %s: module name %q isn't valid", dir, m.Name)
		}
		table, err := e.moduleTable(m)
		if err != nil {
			return nil, err
		}
		s.modules[m.Name] = table
	}

	s.env = e.state.NewTable()
	meta := e.state.NewTable()
	meta.RawSetString("__index", e.state.NewFunction(func(state *glua.LState) int {
		key := state.CheckString(2)
		e.globalHint(state, key, s.modules)
		state.Push(state.GetField(state.G.Global, key))
		return 1
	}))
	e.state.SetMetatable(s.env, meta)
	s.env.RawSetString("require", e.state.NewFunction(s.require))

	return s, nil
}

// Eval executes source in the scope and returns its first return value.
func (s *scope) Eval(ctx context.Context, name, source string) (any, error) {
	fn, err := s.e.state.Load(strings.NewReader(source), name)
	if err != nil {
		return nil, fmt.Errorf("lua: %w", err)
	}
	fn.Env = s.env

	return s.e.eval(ctx, fn)
}

// require loads a module once and returns what it returned, or true if it
// returned nothing, as Lua's require does. require("@name") imports a
// module from another scope.
func (s *scope) require(state *glua.LState) int {
	name := state.CheckString(1)
	if api, ok := strings.CutPrefix(name, "@"); ok {
		state.Push(s.importModule(state, name, api))
		return 1
	}
	state.Push(s.load(state, name))
	return 1
}

// importModule returns the module imports resolves api to, loading it in
// the scope it belongs to. name is what was required, for messages.
func (s *scope) importModule(state *glua.LState, name, api string) glua.LValue {
	if v, ok := s.loaded[name]; ok {
		return v
	}
	if s.imports == nil {
		state.RaiseError("require(%q): %s can't import modules from elsewhere.", name, s.dir)
	}

	from, module, err := s.imports(api)
	if err != nil {
		state.RaiseError("require(%q): %v", name, err)
	}
	if from == nil {
		return glua.LNil
	}
	other, ok := from.(*scope)
	if !ok || other.e != s.e {
		state.RaiseError("require(%q): the module is in another engine, so it can't be imported here.", name)
	}

	value := other.load(state, module)
	s.loaded[name] = value

	return value
}

// load returns the module name, running its file the first time.
func (s *scope) load(state *glua.LState, name string) glua.LValue {
	if v, ok := s.loaded[name]; ok {
		return v
	}
	if t, ok := s.modules[name]; ok {
		return t
	}
	if t, ok := s.e.modules[name]; ok {
		return t
	}
	if s.e.reserved(name, s.modules) {
		state.RaiseError("require: there's no module %q. Modules: %s.", name, strings.Join(s.e.moduleNames(s.modules), ", "))
	}
	if !moduleRx.MatchString(name) {
		state.RaiseError("require: %q isn't a module name. Name modules by their path under %s with dots, like require(\"items\") for %s/items.lua or require(\"items.find\") for %s/items/find.lua.",
			name, s.dir, s.dir, s.dir)
	}
	if i := slices.Index(s.loading, name); i >= 0 {
		chain := append(slices.Clone(s.loading[i:]), name)
		state.RaiseError("require: modules require each other in a loop: %s. Move what they share into a module both can require.",
			strings.Join(chain, " → "))
	}

	file, source, err := s.find(name)
	if err != nil {
		state.RaiseError("require: %v", err)
	}

	fn, err := state.Load(strings.NewReader(source), s.dir+"/"+file)
	if err != nil {
		state.RaiseError("require: %v", err)
	}
	fn.Env = s.env

	s.loading = append(s.loading, name)
	defer func() { s.loading = s.loading[:len(s.loading)-1] }()

	state.Push(fn)
	state.Call(0, 1)
	value := state.Get(-1)
	state.Pop(1)
	if value == glua.LNil {
		value = glua.LTrue
	}
	s.loaded[name] = value

	return value
}

// find returns the file and source of the module name.
func (s *scope) find(name string) (string, string, error) {
	base := strings.ReplaceAll(name, ".", "/")
	tried := []string{base + ".lua", base + "/init.lua"}
	for _, file := range tried {
		source, err := fs.ReadFile(s.files, file)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return "", "", err
		}
		return file, string(source), nil
	}

	msg := fmt.Sprintf("no module %q: looked for %s/%s and %s/%s.", name, s.dir, tried[0], s.dir, tried[1])
	if names := s.names(); len(names) > 0 {
		return "", "", fmt.Errorf("%s Modules in %s: %s.", msg, s.dir, strings.Join(names, ", "))
	}

	return "", "", fmt.Errorf("%s There are no modules in %s yet; a module is a .lua file there that returns a table.", msg, s.dir)
}

// names lists the modules in the scope, as require would name them.
func (s *scope) names() []string {
	var names []string
	fs.WalkDir(s.files, ".", func(file string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || path.Ext(file) != ".lua" {
			return nil
		}
		name := strings.TrimSuffix(file, ".lua")
		name = strings.TrimSuffix(name, "/init")
		names = append(names, strings.ReplaceAll(name, "/", "."))
		return nil
	})
	slices.Sort(names)

	return names
}
