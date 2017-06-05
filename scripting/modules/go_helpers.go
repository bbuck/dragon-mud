// Copyright 2016-2017 Brandon Buck

package modules

import (
	"fmt"

	"github.com/bbuck/dragon-mud/logger"
	"github.com/bbuck/dragon-mud/scripting/keys"
	"github.com/bbuck/dragon-mud/scripting/lua"
)

var logCache = make(map[string]logger.Log)

// create or fetch a logger form a cache, based on name. Use for loggers used
// during module functions.
func log(name string) logger.Log {
	if l, ok := logCache[name]; ok {
		return l
	}

	l := logger.NewWithSource(fmt.Sprintf("lua(%s)", name))
	logCache[name] = l

	return l
}

// fetch a log friendly name for the engine along the lines of 'engine(id)'
func nameForEngine(eng *lua.Engine) string {
	if name, ok := eng.Meta[keys.EngineID].(string); ok {
		return fmt.Sprintf("engine(%s)", name)
	}

	return "engine(unknown)"
}

// fetch the EnginePool associated with the given engine
func poolForEngine(eng *lua.Engine) *lua.EnginePool {
	if p, ok := eng.Meta[keys.Pool].(*lua.EnginePool); ok {
		return p
	}

	log("script modules").WithField("engine", nameForEngine(eng)).Fatal("No pool associated with the scripting engine, cannot continue execution.")

	return nil
}
