package scripting

import "github.com/yuin/gopher-lua"

// Nil represents the nil Lua value.
var Nil = newValue(lua.LNil)

// Number converts a float to a Value representing a number in Lua.
func Number(f float64) *LuaValue {
	return newValue(lua.LNumber(f))
}

// String returns a Value representing a string in Lua.
func String(s string) *LuaValue {
	return newValue(lua.LString(s))
}

// Bool converts a Go bool into a Lua bool type.
func Bool(b bool) *LuaValue {
	if b {
		return newValue(lua.LTrue)
	}

	return newValue(lua.LFalse)
}
