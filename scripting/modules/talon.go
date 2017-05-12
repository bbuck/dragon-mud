package modules

import (
	"github.com/bbuck/dragon-mud/scripting/lua"
	"github.com/bbuck/dragon-mud/talon"
)

type talonResult struct {
	rows talon.Rows
}

// Talon is the core database Lua wrapper, giving the coder access to running
// queries against the database
var Talon = lua.TableMap{
	"exec": func(engine *lua.Engine) int {
		return 0
	},
	"query": func(engine *lua.Engine) int {
		return 0
	},
}
