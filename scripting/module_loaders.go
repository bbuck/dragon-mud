// Copyright (c) 2016-2017 Brandon Buck

package scripting

import (
	"github.com/bbuck/dragon-mud/scripting/lua"
	"github.com/bbuck/dragon-mud/scripting/modules"
)

var moduleMap = map[string]lua.TableMap{
	"tmpl":     modules.Tmpl,
	"password": modules.Password,
	"die":      modules.Die,
	"random":   modules.Random,
	"events":   modules.Events,
	"log":      modules.Log,
	"sutil":    modules.Sutil,
	"cli":      modules.Cli,
	"config":   modules.Config,
}

// OpenLibs will open all modules given to the function as defined in the
// scripting/modules directory.
func OpenLibs(e *lua.Engine, modules ...string) {
	for _, mname := range modules {
		if m, ok := moduleMap[mname]; ok {
			e.RegisterModule(mname, m)
		}
	}
}
