// Copyright (c) 2016-2017 Brandon Buck

package scripting

import "github.com/bbuck/dragon-mud/scripting/engine"

func newServerEngine() *engine.Lua {
	engine := engine.NewLua()
	engine.OpenChannel()
	engine.OpenCoroutine()
	engine.OpenMath()
	engine.OpenString()
	engine.OpenTable()

	return engine
}
