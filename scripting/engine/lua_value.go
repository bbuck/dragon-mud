// Copyright (c) 2016-2017 Brandon Buck

package engine

import (
	"github.com/layeh/gopher-luar"
	"github.com/yuin/gopher-lua"
)

// LuaValue is a utility wrapper for lua.LValue that provies conveinient methods
// for casting.
type LuaValue struct {
	lval  lua.LValue
	owner *Lua
}

// Nil represents the Lua nil value.
var Nil = &LuaValue{lval: lua.LNil}

// String makes Value conform to Stringer
func (v *LuaValue) String() string {
	return v.lval.String()
}

// AsRaw returns the best associated Go type, ingoring functions and any other
// odd types. Only concerns itself with string, bool, nil, number and user data
// types. Tables are again, ignored.
func (v *LuaValue) AsRaw() interface{} {
	switch v.lval.Type() {
	case lua.LTString:
		return v.AsString()
	case lua.LTBool:
		return v.AsBool()
	case lua.LTNil:
		return nil
	case lua.LTNumber:
		return v.AsNumber()
	case lua.LTUserData:
		return v.Interface()
	}

	return nil
}

// AsString returns the LValue as a Go string
func (v *LuaValue) AsString() string {
	return lua.LVAsString(v.lval)
}

// AsFloat returns the LValue as a Go float64.
// This method will try to convert the Lua value to a number if possible, if
// not then LuaNumber(0) is returned.
func (v *LuaValue) AsFloat() float64 {
	return float64(lua.LVAsNumber(v.lval))
}

// AsNumber is an alias for AsFloat (Lua calls them "numbers")
func (v *LuaValue) AsNumber() float64 {
	return v.AsFloat()
}

// AsBool returns the Lua boolean representation for an object (this works for
// non bool Values)
func (v *LuaValue) AsBool() bool {
	return lua.LVAsBool(v.lval)
}

// AsMapStringInterface will work on a Lua Table to convert it into a go
// map[string]interface.
func (v *LuaValue) AsMapStringInterface() map[string]interface{} {
	if v.IsTable() {
		result := make(map[string]interface{})
		v.ForEach(func(key, value *LuaValue) {
			result[key.AsString()] = value.AsRaw()
		})

		return result
	}

	return nil
}

// IsNil will only return true if the Value wraps LNil.
func (v *LuaValue) IsNil() bool {
	return v.lval.Type() == lua.LTNil
}

// IsFalse is similar to AsBool except it returns if the Lua value would be
// considered false in Lua.
func (v *LuaValue) IsFalse() bool {
	return lua.LVIsFalse(v.lval)
}

// IsTrue returns whether or not this is a truthy value or not.
func (v *LuaValue) IsTrue() bool {
	return !v.IsFalse()
}

// The following methods allow for type detection

// IsNumber returns true if the stored value is a numeric value.
func (v *LuaValue) IsNumber() bool {
	return v.lval.Type() == lua.LTNumber
}

// IsBool returns true if the stored value is a boolean value.
func (v *LuaValue) IsBool() bool {
	return v.lval.Type() == lua.LTBool
}

// IsFunction returns true if the stored value is a function.
func (v *LuaValue) IsFunction() bool {
	return v.lval.Type() == lua.LTFunction
}

// IsString returns true if the stored value is a string.
func (v *LuaValue) IsString() bool {
	return v.lval.Type() == lua.LTString
}

// IsTable returns true if the stored value is a table.
func (v *LuaValue) IsTable() bool {
	return v.lval.Type() == lua.LTTable
}

// The following methods allow LTable values to be modified through Go.

// asTable converts the Value into an LTable.
func (v *LuaValue) asTable() (t *lua.LTable) {
	t, _ = v.lval.(*lua.LTable)

	return
}

// isUserData returns a bool if the Value is an LUserData
func (v *LuaValue) isUserData() bool {
	return v.lval.Type() == lua.LTUserData
}

// asUserData converts the Value into an LUserData
func (v *LuaValue) asUserData() (t *lua.LUserData) {
	t, _ = v.lval.(*lua.LUserData)

	return
}

// Append maps to lua.LTable.Append
func (v *LuaValue) Append(value interface{}) {
	if v.IsTable() {
		val := getLValue(v.owner, value)

		t := v.asTable()
		t.Append(val)
	}
}

// ForEach maps to lua.LTable.ForEach
func (v *LuaValue) ForEach(cb func(*LuaValue, *LuaValue)) {
	if v.IsTable() {
		actualCb := func(key lua.LValue, val lua.LValue) {
			cb(v.owner.newValue(key), v.owner.newValue(val))
		}
		t := v.asTable()
		t.ForEach(actualCb)
	}
}

// Insert maps to lua.LTable.Insert
func (v *LuaValue) Insert(i int, value interface{}) {
	if v.IsTable() {
		val := getLValue(v.owner, value)

		t := v.asTable()
		t.Insert(i, val)
	}
}

// Len maps to lua.LTable.Len
func (v *LuaValue) Len() int {
	if v.IsTable() {
		t := v.asTable()

		return t.Len()
	}

	return -1
}

// MaxN maps to lua.LTable.MaxN
func (v *LuaValue) MaxN() int {
	if v.IsTable() {
		t := v.asTable()

		return t.MaxN()
	}

	return 0
}

// Next maps to lua.LTable.Next
func (v *LuaValue) Next(key interface{}) (*LuaValue, *LuaValue) {
	if v.IsTable() {
		val := getLValue(v.owner, key)

		t := v.asTable()
		v1, v2 := t.Next(val)

		return v.owner.newValue(v1), v.owner.newValue(v2)
	}

	return Nil, Nil
}

// Remove maps to lua.LTable.Remove
func (v *LuaValue) Remove(pos int) *LuaValue {
	if v.IsTable() {
		t := v.asTable()
		ret := t.Remove(pos)

		return v.owner.newValue(ret)
	}

	return Nil
}

// Helper method for Set and RawSet
func getLValue(e *Lua, item interface{}) lua.LValue {
	switch val := item.(type) {
	case (*LuaValue):
		return val.lval
	case lua.LValue:
		return val
	}

	if e != nil {
		return luar.New(e.state, item)
	}

	return lua.LNil
}

// Get returns the value associated with the key given if the LuaValue wraps
// a table.
func (v *LuaValue) Get(key interface{}) *LuaValue {
	if v.IsTable() {
		k := getLValue(v.owner, key)
		val := v.owner.state.GetTable(v.lval, k)

		return v.owner.ValueFor(val)
	}

	return nil
}

// Set sets the value of a given key on the table, this method checks for
// validity of array keys and handles them accordingly.
func (v *LuaValue) Set(goKey interface{}, val interface{}) {
	if v.IsTable() {
		key := getLValue(v.owner, goKey)
		lval := getLValue(v.owner, val)

		v.asTable().RawSet(key, lval)
	}
}

// RawSet bypasses any checks for key existence and sets the value onto the
// table with the given key.
func (v *LuaValue) RawSet(goKey interface{}, val interface{}) {
	if v.IsTable() {
		key := getLValue(v.owner, goKey)
		lval := getLValue(v.owner, val)

		v.asTable().RawSetH(key, lval)
	}
}

// The following provde methods for LUserData

// Interface returns the value of the LUserData
func (v *LuaValue) Interface() interface{} {
	if v.isUserData() {
		t := v.asUserData()

		return t.Value
	}

	return nil
}

// The following provide LFunction methods on Value

// FuncLocalName is a function that returns the local name of a LFunction type
// if this Value objects holds an LFunction.
func (v *LuaValue) FuncLocalName(regno, pc int) (string, bool) {
	if f, ok := v.lval.(*lua.LFunction); ok {
		return f.LocalName(regno, pc)
	}

	return "", false
}

// Call invokes the LuaValue as a function (if it is one) with similar behavior
// to engine.Call
func (v *LuaValue) Call(retCount int, argList ...interface{}) ([]*LuaValue, error) {
	if v.IsFunction() && v.owner != nil {
		p := lua.P{
			Fn:      v.lval,
			NRet:    retCount,
			Protect: true,
		}
		args := make([]lua.LValue, len(argList))
		for i, iface := range argList {
			args[i] = getLValue(v.owner, iface)
		}

		err := v.owner.state.CallByParam(p, args...)
		if err != nil {
			return nil, err
		}

		retVals := make([]*LuaValue, retCount)
		for i := 0; i < retCount; i++ {
			retVals[i] = v.owner.ValueFor(v.owner.state.Get(-1))
		}

		return retVals, nil
	}

	return make([]*LuaValue, 0), nil
}

// The following are Lua -> Go advanced transformations

// ToMap will convert the given value (if it's a Lua table) into a
// map[string]interface{}. It will coerce all keys into strings and attempt
// to extract the Go value of each value in the table, but will preserve
// LuaValue references for tables.
func (v *LuaValue) ToMap() map[string]interface{} {
	m := make(map[string]interface{})
	if v.IsTable() {
		v.ForEach(func(k, lv *LuaValue) {
			var val interface{} = lv.AsRaw()
			if lv.IsTable() {
				val = lv
			}
			m[k.AsString()] = val
		})
	}

	return m
}

// ToSlice will convert the Lua table value to a []interface{}, extracting
// Go values were possible and preserving references to tables.
func (v *LuaValue) ToSlice() []interface{} {
	var s []interface{}
	if v.IsTable() {
		len := v.Len()
		for i := 1; i <= len; i++ {
			lv := v.Get(i)
			var val interface{} = lv.AsRaw()
			if lv.IsTable() {
				val = lv
			}
			s = append(s, val)
		}
	}

	return s
}
