// Package lua implements scripting.Engine with gopher-lua (Lua 5.1).
//
// Scripts run in a sandbox: only the base, table, string, math and coroutine
// libraries are opened, and functions that touch the filesystem (dofile,
// loadfile, require) are removed. Plugin loading is the engine's job.
package lua

import (
	"context"
	"errors"
	"fmt"
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
	state *glua.LState
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

	return &Engine{state: state}
}

// Load makes m available to scripts as a global table named m.Name.
func (e *Engine) Load(m scripting.Module) error {
	if m.Name == "" {
		return errors.New("lua: module has no name")
	}

	if e.state.GetGlobal(m.Name) != glua.LNil {
		return fmt.Errorf("lua: module %q: name already in use", m.Name)
	}

	table := e.state.NewTable()

	for name, value := range m.Values {
		lv, err := e.toLua(value, 0)
		if err != nil {
			return fmt.Errorf("lua: module %q value %q: %w", m.Name, name, err)
		}
		e.state.SetField(table, name, lv)
	}

	for name, fn := range m.Funcs {
		e.state.SetField(table, name, e.wrap(m.Name+"."+name, fn))
	}

	e.state.SetGlobal(m.Name, table)

	return nil
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

		lv, err := e.toLua(result, 0)
		if err != nil {
			state.RaiseError("%s: return value: %v", name, err)
		}

		state.Push(lv)

		return 1
	})
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
