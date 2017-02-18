package modules_test

import "github.com/bbuck/dragon-mud/scripting/lua"

func testReturn(eng *lua.Engine, script string) *lua.Value {
	return eng.Nil()
}
