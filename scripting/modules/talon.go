package modules

import (
	"github.com/bbuck/dragon-mud/scripting/lua"
	"github.com/bbuck/dragon-mud/talon"
)

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

type talonRows struct {
	rows talon.Rows
}

func (tr *talonRows) Next() (*lua.Value, error) {
	return nil, nil
}
