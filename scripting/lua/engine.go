// Package lua implements scripting.Engine with gopher-lua (Lua 5.1).
//
// Scripts run in a sandbox: only the base, table, string, math and coroutine
// libraries are opened, and functions that touch the filesystem (dofile,
// loadfile, require) are removed. Plugin loading is the engine's job; a
// Scope gives a plugin's files a require that reads only its own modules.
package lua

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	glua "github.com/yuin/gopher-lua"

	"bbuck.dev/dragon-mud/scripting"
)

var _ scripting.Engine = (*Engine)(nil)

// libraries are the standard libraries scripts may use.
var libraries = []struct {
	name string
	open glua.LGFunction
}{
	{glua.BaseLibName, glua.OpenBase},
	{glua.TabLibName, glua.OpenTable},
	{glua.StringLibName, glua.OpenString},
	{glua.MathLibName, glua.OpenMath},
	{glua.CoroutineLibName, glua.OpenCoroutine},
}

// removedGlobals are base library functions that reach outside the sandbox.
var removedGlobals = []string{"dofile", "loadfile", "require"}

// Engine runs Lua scripts. It is not safe for concurrent use.
type Engine struct {
	state   *glua.LState
	handles handles

	// modules are the loaded modules by name, for require.
	modules map[string]*glua.LTable
}

// New returns an Engine with a fresh, sandboxed Lua state.
func New() *Engine {
	state := glua.NewState(glua.Options{SkipOpenLibs: true})

	for _, lib := range libraries {
		err := state.CallByParam(glua.P{
			Fn:      state.NewFunction(lib.open),
			Protect: true,
		}, glua.LString(lib.name))
		if err != nil {
			// The standard libraries can't fail to open in a fresh state.
			panic(fmt.Sprintf("lua: opening %q library: %v", lib.name, err))
		}
	}

	for _, name := range removedGlobals {
		state.SetGlobal(name, glua.LNil)
	}

	e := &Engine{state: state, handles: newHandles(), modules: make(map[string]*glua.LTable)}

	// Outside a scope, require reaches only the engine's modules.
	state.SetGlobal("require", state.NewFunction(func(state *glua.LState) int {
		name := state.CheckString(1)
		if t, ok := e.modules[name]; ok {
			state.Push(t)
			return 1
		}
		state.RaiseError("require: there's no module %q. Modules: %s.", name, strings.Join(e.moduleNames(nil), ", "))
		return 0
	}))

	meta := state.NewTable()
	meta.RawSetString("__index", state.NewFunction(func(state *glua.LState) int {
		e.globalHint(state, state.CheckString(2), nil)
		state.Push(glua.LNil)
		return 1
	}))
	state.SetMetatable(state.G.Global, meta)

	return e
}

// Load makes m available to scripts through require(m.Name).
func (e *Engine) Load(m scripting.Module) error {
	if m.Name == "" {
		return errors.New("lua: module has no name")
	}
	if !moduleRx.MatchString(m.Name) {
		return fmt.Errorf("lua: module name %q isn't valid; use dot-separated words like \"dragon.world\"", m.Name)
	}
	if _, ok := e.modules[m.Name]; ok {
		return fmt.Errorf("lua: module %q: name already in use", m.Name)
	}

	table, err := e.moduleTable(m)
	if err != nil {
		return err
	}
	e.modules[m.Name] = table

	return nil
}

// moduleNames lists the engine's modules and extra, sorted.
func (e *Engine) moduleNames(extra map[string]*glua.LTable) []string {
	names := slices.Collect(maps.Keys(e.modules))
	for name := range extra {
		if !slices.Contains(names, name) {
			names = append(names, name)
		}
	}
	slices.Sort(names)

	return names
}

// reserved reports whether name is in a module namespace, such as
// "dragon.wrld" when "dragon.world" is loaded, which require never looks
// for in files.
func (e *Engine) reserved(name string, extra map[string]*glua.LTable) bool {
	first, _, _ := strings.Cut(name, ".")
	for _, m := range e.moduleNames(extra) {
		if ns, _, dotted := strings.Cut(m, "."); dotted && ns == first {
			return true
		}
	}

	return false
}

// globalHint raises an error when a script reads a global named like a
// module, such as world for dragon.world: modules aren't globals.
func (e *Engine) globalHint(state *glua.LState, key string, extra map[string]*glua.LTable) {
	for _, name := range e.moduleNames(extra) {
		short := name[strings.LastIndex(name, ".")+1:]
		if short == key {
			state.RaiseError("%s isn't a global. Add local %s = require(%q) at the top of the file.", key, key, name)
		}
	}
}

// moduleTable builds the table scripts see for m, naming its functions
// m.Name.function in errors.
func (e *Engine) moduleTable(m scripting.Module) (*glua.LTable, error) {
	table := e.state.NewTable()

	for name, value := range m.Values {
		lv, err := e.toLua(value, 0)
		if err != nil {
			return nil, fmt.Errorf("lua: module %q value %q: %w", m.Name, name, err)
		}
		e.state.SetField(table, name, lv)
	}

	for name, fn := range m.Funcs {
		e.state.SetField(table, name, e.wrap(m.Name+"."+name, fn))
	}

	return table, nil
}

// Run executes source as a Lua chunk.
func (e *Engine) Run(ctx context.Context, name, source string) error {
	_, err := e.Eval(ctx, name, source)

	return err
}

// Eval executes source as a Lua chunk and returns its first return value.
func (e *Engine) Eval(ctx context.Context, name, source string) (any, error) {
	fn, err := e.state.Load(strings.NewReader(source), name)
	if err != nil {
		return nil, fmt.Errorf("lua: %w", err)
	}

	return e.eval(ctx, fn)
}

// eval calls the loaded chunk fn and returns its first return value.
func (e *Engine) eval(ctx context.Context, fn *glua.LFunction) (any, error) {
	restore := e.useContext(ctx)
	defer restore()

	e.state.Push(fn)
	if err := e.state.PCall(0, 1, nil); err != nil {
		return nil, e.callError(ctx, err)
	}

	result := e.state.Get(-1)
	e.state.Pop(1)

	return e.fromLua(result, 0)
}

// Close releases the Lua state.
func (e *Engine) Close() {
	e.state.Close()
}

// wrap adapts a scripting.Func into a Lua function. name is used in error
// messages, for example "die.roll".
func (e *Engine) wrap(name string, fn scripting.Func) *glua.LFunction {
	return e.state.NewFunction(func(state *glua.LState) int {
		args := make(scripting.Args, state.GetTop())
		for i := range args {
			value, err := e.fromLua(state.Get(i+1), 0)
			if err != nil {
				state.RaiseError("%s: argument #%d: %v", name, i+1, err)
			}
			args[i] = value
		}

		result, err := fn(args)
		if err != nil {
			state.RaiseError("%s: %v", name, err)
		}

		return e.pushResult(state, name, result)
	})
}

// pushResult pushes what a Func or Method returned, expanding Results into
// separate values, and returns how many values it pushed.
func (e *Engine) pushResult(state *glua.LState, name string, result any) int {
	results, ok := result.(scripting.Results)
	if !ok {
		results = scripting.Results{result}
	}

	for i, value := range results {
		lv, err := e.toLua(value, 0)
		if err != nil {
			state.RaiseError("%s: return value #%d: %v", name, i+1, err)
		}
		state.Push(lv)
	}

	return len(results)
}

// useContext sets ctx on the Lua state so long-running scripts can be
// interrupted. If a script is already running (Go was called from Lua and is
// now calling back into Lua), the outer context stays in effect. The returned
// function restores the previous state.
func (e *Engine) useContext(ctx context.Context) func() {
	if e.state.Context() != nil {
		return func() {}
	}

	e.state.SetContext(ctx)

	return func() {
		e.state.RemoveContext()
	}
}

// callError converts an error from a Lua call, reporting cancellation as the
// context's error so callers can use errors.Is.
func (e *Engine) callError(ctx context.Context, err error) error {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return fmt.Errorf("lua: script interrupted: %w", ctxErr)
	}

	return fmt.Errorf("lua: %w", err)
}
