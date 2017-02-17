// Copyright (c) 2016-2017 Brandon Buck

package engine

import (
	"reflect"

	"github.com/layeh/gopher-luar"
	"github.com/yuin/gopher-lua"
)

// Lua struct stores a pointer to a gluaLState providing a simplified API.
type Lua struct {
	state *lua.LState
}

// ScriptFunction is a type alias for a function that receives an Engine and
// returns an int.
type ScriptFunction func(*Lua) int

// LuaTableMap interface to speed along the creation of table defining maps
// when creating Go modueles for use in Lua.
type LuaTableMap map[string]interface{}

// NewLua creates a new engine containing a new lua.LState.
func NewLua() *Lua {
	eng := &Lua{
		state: lua.NewState(lua.Options{
			SkipOpenLibs:        true,
			IncludeGoStackTrace: true,
		}),
	}
	eng.OpenBase()
	eng.OpenPackage()

	return eng
}

// Close will perform a close on the Lua state.
func (e *Lua) Close() {
	e.state.Close()
}

// OpenBase allows the Lua engine to open the base library up for use in
// scripts.
func (e *Lua) OpenBase() int {
	return lua.OpenBase(e.state)
}

// OpenChannel allows the Lua module for Go channel support to be accessible
// to scripts.
func (e *Lua) OpenChannel() int {
	return lua.OpenChannel(e.state)
}

// OpenCoroutine allows the Lua module for goroutine suppor tto be accessible
// to scripts.
func (e *Lua) OpenCoroutine() int {
	return lua.OpenCoroutine(e.state)
}

// OpenDebug allows the Lua module support debug features to be accissible
// in scripts.
func (e *Lua) OpenDebug() int {
	return lua.OpenDebug(e.state)
}

// OpenIO allows the input/output Lua module to be accessbile in scripts.
func (e *Lua) OpenIO() int {
	return lua.OpenIo(e.state)
}

// OpenMath allows the Lua math moduled to be accessible in scripts.
func (e *Lua) OpenMath() int {
	return lua.OpenMath(e.state)
}

// OpenOS allows the OS Lua module to be accessible in scripts.
func (e *Lua) OpenOS() int {
	return lua.OpenOs(e.state)
}

// OpenPackage allows the Lua module for packages to be used in scripts.
// TODO: Find out what this does/means.
func (e *Lua) OpenPackage() int {
	return lua.OpenPackage(e.state)
}

// OpenString allows the Lua module for string operations to be used in
// scripts.
func (e *Lua) OpenString() int {
	return lua.OpenString(e.state)
}

// OpenTable allows the Lua module for table operations to be used in scripts.
func (e *Lua) OpenTable() int {
	return lua.OpenTable(e.state)
}

// OpenLibs seeds the engine with some basic library access. This should only
// be used if security isn't necessarily a major concern.
func (e *Lua) OpenLibs() {
	e.state.OpenLibs()
}

// DoFile runs the file through the Lua interpreter.
func (e *Lua) DoFile(fn string) error {
	return e.state.DoFile(fn)
}

// DoString runs the given string through the Lua interpreter.
func (e *Lua) DoString(src string) error {
	return e.state.DoString(src)
}

// SetGlobal allows for setting global variables in the loaded code.
func (e *Lua) SetGlobal(name string, val interface{}) {
	v := e.ValueFor(val)

	e.state.SetGlobal(name, v.lval)
}

// GetGlobal returns the value associated with the given name, or LuaNil
func (e *Lua) GetGlobal(name string) *LuaValue {
	lv := e.state.GetGlobal(name)

	return e.newValue(lv)
}

// SetField applies the value to the given table associated with the given
// key.
func (e *Lua) SetField(tbl *LuaValue, key string, val interface{}) {
	v := e.ValueFor(val)
	e.state.SetField(tbl.lval, key, v.lval)
}

// RegisterFunc registers a Go function with the script. Using this method makes
// Go functions accessible through Lua scripts.
func (e *Lua) RegisterFunc(name string, fn interface{}) {
	var lfn lua.LValue
	if sf, ok := fn.(func(*Lua) int); ok {
		lfn = e.genScriptFunc(sf)
	} else {
		v := e.ValueFor(fn)
		lfn = v.lval
	}
	e.state.SetGlobal(name, lfn)
}

// RegisterModule takes the values given, maps them to a LuaTable and then
// preloads the module with the given name to be consumed in Lua code.
func (e *Lua) RegisterModule(name string, fields map[string]interface{}) *LuaValue {
	table := e.NewTable()
	for key, val := range fields {
		if sf, ok := val.(func(*Lua) int); ok {
			table.RawSet(key, e.genScriptFunc(sf))
		} else {
			table.RawSet(key, e.ValueFor(val).lval)
		}
	}

	loader := func(l *lua.LState) int {
		l.Push(table.lval)

		return 1
	}
	e.state.PreloadModule(name, loader)

	return table
}

func (e *Lua) Get(n int) *LuaValue {
	lv := e.state.Get(1)
	return e.newValue(lv)
}

// PopArg returns the top value on the Lua stack.
// This method is used to get arguments given to a Go function from a Lua script.
// This method will return a Value pointer that can then be converted into
// an appropriate type.
func (e *Lua) PopArg() *LuaValue {
	lv := e.state.Get(-1)
	e.state.Pop(1)
	val := e.newValue(lv)
	if val.IsTable() {
		val.owner = e
	}

	return val
}

// PushValue pushes the given Value onto the Lua stack.
// Use this method when 'returning' values from a Go function called from a
// Lua script.
func (e *Lua) PushValue(val interface{}) {
	v := e.ValueFor(val)
	e.state.Push(v.lval)
}

// StackSize returns the maximum value currently remaining on the stack.
func (e *Lua) StackSize() int {
	return e.state.GetTop()
}

// PopBool returns the top of the stack as an actual Go bool.
func (e *Lua) PopBool() bool {
	v := e.PopArg()

	return v.AsBool()
}

// PopFunction is an alias for PopArg, provided for readability when specifying
// the desired value from the top of the stack.
func (e *Lua) PopFunction() *LuaValue {
	return e.PopArg()
}

// PopInt returns the top of the stack as an actual Go int.
func (e *Lua) PopInt() int {
	v := e.PopArg()
	i := int(v.AsNumber())

	return i
}

// PopInt64 returns the top of the stack as an actual Go int64.
func (e *Lua) PopInt64() int64 {
	v := e.PopArg()
	i := int64(v.AsNumber())

	return i
}

// PopFloat returns the top of the stack as an actual Go float.
func (e *Lua) PopFloat() float64 {
	v := e.PopArg()

	return v.AsFloat()
}

// PopNumber is an alias for PopArg, provided for readability when specifying
// the desired value from the top of the stack.
func (e *Lua) PopNumber() *LuaValue {
	return e.PopArg()
}

// PopString returns the top of the stack as an actual Go string value.
func (e *Lua) PopString() string {
	v := e.PopArg()

	return v.AsString()
}

// PopTable is an alias for PopArg, provided for readability when specifying
// the desired value from the top of the stack.
func (e *Lua) PopTable() *LuaValue {
	tbl := e.PopArg()
	tbl.owner = e

	return tbl
}

// PopInterface returns the top of the stack as an actual Go interface.
func (e *Lua) PopInterface() interface{} {
	v := e.PopArg()

	return v.Interface()
}

// True returns a value for the constant 'true' in Lua.
func (e *Lua) True() *LuaValue {
	return e.newValue(lua.LTrue)
}

// False returns a value for the constant 'false' in Lua.
func (e *Lua) False() *LuaValue {
	return e.newValue(lua.LFalse)
}

// Nil returns a value for the constant 'nil' in Lua.
func (e *Lua) Nil() *LuaValue {
	return e.newValue(lua.LNil)
}

// Call allows for calling a method by name.
// The second parameter is the number of return values the function being
// called should return. These values will be returned in a slice of Value
// pointers.
func (e *Lua) Call(name string, retCount int, params ...interface{}) ([]*LuaValue, error) {
	luaParams := make([]lua.LValue, len(params))
	for i, iface := range params {
		v := e.ValueFor(iface)
		luaParams[i] = v.lval
	}

	err := e.state.CallByParam(lua.P{
		Fn:      e.state.GetGlobal(name),
		NRet:    retCount,
		Protect: true,
	}, luaParams...)

	if err != nil {
		return nil, err
	}

	retVals := make([]*LuaValue, retCount)
	for i := retCount - 1; i >= 0; i-- {
		retVals[i] = e.ValueFor(e.state.Get(-1))
		e.state.Pop(1)
	}

	return retVals, nil
}

// RegisterType creates a construtor with the given name that will generate the
// given type.
func (e *Lua) RegisterType(name string, val interface{}) {
	cons := luar.NewType(e.state, val)
	e.state.SetGlobal(name, cons)
}

// RegisterClass assigns a new type, but instead of creating it via "TypeName()"
// it provides a more OO way of creating the object "TypeName.new()" otherwise
// it's functionally equivalent to RegisterType.
func (e *Lua) RegisterClass(name string, val interface{}) {
	cons := luar.NewType(e.state, val)
	table := e.NewTable()
	table.RawSet("new", cons)
	e.state.SetGlobal(name, table.lval)
}

// RegisterClassWithCtor does the same thing as RegisterClass excep the new
// function is mapped to the constructor passed in.
func (e *Lua) RegisterClassWithCtor(name string, typ interface{}, cons interface{}) {
	luar.NewType(e.state, typ)
	lcons := e.ValueFor(cons)
	table := e.NewTable()
	table.RawSet("new", lcons)

	e.state.SetGlobal(name, table.lval)
}

// ValueFor takes a Go type and creates a lua equivalent Value for it.
func (e *Lua) ValueFor(val interface{}) *LuaValue {
	switch v := val.(type) {
	case ScriptableObject:
		return e.newValue(luar.New(e.state, v.ScriptObject()))
	case *LuaValue:
		return v
	default:
		return e.newValue(luar.New(e.state, val))
	}
}

// WhitelistFor will mark the given method names whitelisted on the metatable
// for the given interface{} value.
func (e *Lua) WhitelistFor(i interface{}, names ...string) {
	mt := luar.MT(e.state, i)
	if mt != nil {
		mt.Whitelist(names...)
	}
}

// BlacklistFor will mark the given method names blacklisted on the metatable
// for the given interface{} value.
func (e *Lua) BlacklistFor(i interface{}, names ...string) {
	mt := luar.MT(e.state, i)
	if mt != nil {
		mt.Blacklist(names...)
	}
}

// TableFromMap takes a map of go values and generates a Lua table representing
// the value.
func (e *Lua) TableFromMap(i interface{}) *LuaValue {
	t := e.NewTable()
	m := reflect.ValueOf(i)
	if m.Kind() == reflect.Map {
		for _, k := range m.MapKeys() {
			t.Set(k.Interface(), m.MapIndex(k).Interface())
		}
	}

	return t
}

// TableFromSlice converts the given slice into a table ready for use in Lua.
func (e *Lua) TableFromSlice(i interface{}) *LuaValue {
	t := e.NewTable()
	s := reflect.ValueOf(i)
	if s.Kind() == reflect.Slice {
		for i := 0; i < s.Len(); i++ {
			t.Append(s.Index(i).Interface())
		}
	}

	return t
}

// newValue constructs a new value from an LValue.
func (e *Lua) newValue(val lua.LValue) *LuaValue {
	return &LuaValue{
		lval:  val,
		owner: e,
	}
}

// NewTable creates and returns a new NewTable.
func (e *Lua) NewTable() *LuaValue {
	tbl := e.newValue(e.state.NewTable())
	tbl.owner = e

	return tbl
}

// wrapScriptFunction turns a ScriptFunction into a lua.LGFunction
func (e *Lua) wrapScriptFunction(fn ScriptFunction) lua.LGFunction {
	return func(l *lua.LState) int {
		e := &Lua{state: l}

		return fn(e)
	}
}

// genScriptFunc will wrap a ScriptFunction with a function that gopher-lua
// expects to see when calling method from Lua.
func (e *Lua) genScriptFunc(fn ScriptFunction) *lua.LFunction {
	return e.state.NewFunction(e.wrapScriptFunction(fn))
}
