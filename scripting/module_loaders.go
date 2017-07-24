// Copyright (c) 2016-2017 Brandon Buck

package scripting

import (
	"github.com/bbuck/dragon-mud/scripting/lua"
	"github.com/bbuck/dragon-mud/scripting/modules"
)

var simpleModuleMap = map[string]lua.TableMap{
	"tmpl":     modules.Tmpl,
	"password": modules.Password,
	"die":      modules.Die,
	"random":   modules.Random,
	"events":   modules.Events,
	"log":      modules.Log,
	"sutil":    modules.Sutil,
	"cli":      modules.Cli,
	"config":   modules.Config,
	"time":     modules.Time,
	"uuid":     modules.UUID,
}

// ModuleLoader represents a function that takes a lua engine and loads a
// module into it, such that is available for use by any script wishing to
// import the module.
type ModuleLoader func(*lua.Engine)

var complexModuleMap = map[string]ModuleLoader{
	"talon":  modules.TalonLoader,
	"fn":     ScriptLoader("modules/fn.lua"),
	"entity": modules.EntityLoader,
}

// OpenLibs will open all modules given to the function as defined in the
// scripting/modules directory.
func OpenLibs(e *lua.Engine, modules ...string) {
	if len(modules) >= 1 && modules[0] == "*" {
		loadAll(e, modules[1:]...)

		return
	}

	for _, mname := range modules {
		if m, ok := simpleModuleMap[mname]; ok {
			e.RegisterModule(mname, m)

			continue
		}

		if fn, ok := complexModuleMap[mname]; ok {
			fn(e)
		}
	}
}

// modified open libs, executes with open libs input like "*", "-talon", "-time"
// which loads all modules but talon and time into the engine.
func loadAll(e *lua.Engine, modules ...string) {
	ignore := make(map[string]struct{})
	for _, mod := range modules {
		if len(mod) >= 1 && mod[0] == '-' {
			ignore[mod[1:]] = struct{}{}
		}
	}

	for mname, mod := range simpleModuleMap {
		if _, ok := ignore[mname]; !ok {
			e.RegisterModule(mname, mod)

			continue
		}
	}

	for mname, modFn := range complexModuleMap {
		if _, ok := ignore[mname]; !ok {
			modFn(e)
		}
	}
}
