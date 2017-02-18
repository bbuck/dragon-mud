// Copyright (c) 2016-2017 Brandon Buck

package scripting

import "github.com/bbuck/dragon-mud/scripting/lua"

func newServerEngine() *lua.Engine {
	engine := lua.NewEngine()
	engine.OpenChannel()
	engine.OpenCoroutine()
	engine.OpenMath()
	engine.OpenString()
	engine.OpenTable()

	return engine
}
