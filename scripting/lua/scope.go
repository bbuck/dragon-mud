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
	e       *Engine
	dir     string
	modules fs.FS
	env     *glua.LTable

	// loaded holds each module's value, so a module runs once per scope.
	loaded map[string]glua.LValue

	// loading is the chain of modules being loaded, to catch cycles.
	loading []string
}

// Scope returns a scope whose require loads "a.b" from a/b.lua or
// a/b/init.lua in modules, with values as globals of its own.
func (e *Engine) Scope(dir string, modules fs.FS, values map[string]any) (scripting.Scope, error) {
	s := &scope{e: e, dir: dir, modules: modules, loaded: make(map[string]glua.LValue)}

	s.env = e.state.NewTable()
	meta := e.state.NewTable()
	meta.RawSetString("__index", e.state.G.Global)
	e.state.SetMetatable(s.env, meta)
	for name, value := range values {
		lv, err := e.toLua(value, 0)
		if err != nil {
			return nil, fmt.Errorf("lua: scope %s: value %q: %w", dir, name, err)
		}
		s.env.RawSetString(name, lv)
	}
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
// returned nothing, as Lua's require does.
func (s *scope) require(state *glua.LState) int {
	name := state.CheckString(1)
	if v, ok := s.loaded[name]; ok {
		state.Push(v)
		return 1
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

	state.Push(value)
	return 1
}

// find returns the file and source of the module name.
func (s *scope) find(name string) (string, string, error) {
	base := strings.ReplaceAll(name, ".", "/")
	tried := []string{base + ".lua", base + "/init.lua"}
	for _, file := range tried {
		source, err := fs.ReadFile(s.modules, file)
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
	fs.WalkDir(s.modules, ".", func(file string, d fs.DirEntry, err error) error {
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
