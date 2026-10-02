package lua

import (
	"errors"
	"reflect"

	glua "github.com/yuin/gopher-lua"

	"bbuck.dev/dragon-mud/scripting"
)

// handles presents scripting.Handles as userdata. Each handle gets one
// userdata per engine, so equal handles are equal in Lua and work as the
// same table key.
type handles struct {
	userdata   map[scripting.Handle]*glua.LUserData
	metatables map[*scripting.Type]*glua.LTable
}

func newHandles() handles {
	return handles{
		userdata:   make(map[scripting.Handle]*glua.LUserData),
		metatables: make(map[*scripting.Type]*glua.LTable),
	}
}

// handleToLua returns the userdata for h.
func (e *Engine) handleToLua(h scripting.Handle) (glua.LValue, error) {
	if h.Type == nil {
		return nil, errors.New("handle has no type")
	}
	if h.Key == nil || !reflect.TypeOf(h.Key).Comparable() {
		return nil, errors.New("handle key must be comparable")
	}

	if ud, ok := e.handles.userdata[h]; ok {
		return ud, nil
	}

	ud := e.state.NewUserData()
	ud.Value = h
	ud.Metatable = e.metatable(h.Type)
	e.handles.userdata[h] = ud

	return ud, nil
}

// metatable returns the metatable shared by every handle of type t.
func (e *Engine) metatable(t *scripting.Type) *glua.LTable {
	if mt, ok := e.handles.metatables[t]; ok {
		return mt
	}

	methods := make(map[string]*glua.LFunction, len(t.Methods))
	for name, method := range t.Methods {
		methods[name] = e.method(t, name, method)
	}

	mt := e.state.NewTable()

	e.state.SetField(mt, "__index", e.state.NewFunction(func(state *glua.LState) int {
		h := state.CheckUserData(1).Value.(scripting.Handle)
		key := state.CheckString(2)

		if field, ok := t.Fields[key]; ok {
			value, err := field(h.Key)
			if err != nil {
				state.RaiseError("%s.%s: %v", t.Name, key, err)
			}
			lv, err := e.toLua(value, 0)
			if err != nil {
				state.RaiseError("%s.%s: %v", t.Name, key, err)
			}
			state.Push(lv)
			return 1
		}

		if fn, ok := methods[key]; ok {
			state.Push(fn)
			return 1
		}

		state.RaiseError("%s has no field or method %q", t.Name, key)
		return 0
	}))

	e.state.SetField(mt, "__newindex", e.state.NewFunction(func(state *glua.LState) int {
		state.RaiseError("%s fields can't be assigned; use its methods", t.Name)
		return 0
	}))

	e.state.SetField(mt, "__tostring", e.state.NewFunction(func(state *glua.LState) int {
		h := state.CheckUserData(1).Value.(scripting.Handle)
		state.Push(glua.LString(h.Describe()))
		return 1
	}))

	// Scripts can't reach the real metatable to change how handles behave.
	e.state.SetField(mt, "__metatable", glua.LString("locked"))

	e.handles.metatables[t] = mt

	return mt
}

// method adapts a scripting.Method into a Lua function called as h:name().
func (e *Engine) method(t *scripting.Type, name string, method scripting.Method) *glua.LFunction {
	full := t.Name + ":" + name

	return e.state.NewFunction(func(state *glua.LState) int {
		h, ok := handleFromLua(state.Get(1))
		if !ok || h.Type != t {
			state.RaiseError("%s: call it with a colon, as value:%s(...)", full, name)
		}

		args := make(scripting.Args, state.GetTop()-1)
		for i := range args {
			value, err := e.fromLua(state.Get(i+2), 0)
			if err != nil {
				state.RaiseError("%s: argument #%d: %v", full, i+1, err)
			}
			args[i] = value
		}

		result, err := method(h.Key, args)
		if err != nil {
			state.RaiseError("%s: %v", full, err)
		}

		lv, err := e.toLua(result, 0)
		if err != nil {
			state.RaiseError("%s: return value: %v", full, err)
		}
		state.Push(lv)

		return 1
	})
}

func handleFromLua(lv glua.LValue) (scripting.Handle, bool) {
	ud, ok := lv.(*glua.LUserData)
	if !ok {
		return scripting.Handle{}, false
	}
	h, ok := ud.Value.(scripting.Handle)

	return h, ok
}
