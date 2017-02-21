package modules

import (
	"github.com/bbuck/dragon-mud/events"
	"github.com/bbuck/dragon-mud/logger"
	"github.com/bbuck/dragon-mud/scripting/keys"
	"github.com/bbuck/dragon-mud/scripting/lua"
)

// Log is the definition of the Lua logging module.
//   error(msg: string[, data: table])
//     log message with data on the error level, data can be omitted or nil
//   warn(msg: string[, data: table])
//     log message with data on the warn level, data can be omitted or nil
//   info(msg: string[, data: table])
//     log message with data on the info level, data can be omitted or nil
//   debug(msg: string[, data: table])
//     log message with data on the debug level, data can be omitted or nil
var Log = map[string]interface{}{
	"error": func(eng *lua.Engine) int {
		performLog(eng, func(l logger.Log, msg string) {
			l.Error(msg)
		})

		return 0
	},
	"warn": func(eng *lua.Engine) int {
		performLog(eng, func(l logger.Log, msg string) {
			l.Warn(msg)
		})

		return 0
	},
	"info": func(eng *lua.Engine) int {
		performLog(eng, func(l logger.Log, msg string) {
			l.Info(msg)
		})

		return 0
	},
	"debug": func(eng *lua.Engine) int {
		performLog(eng, func(l logger.Log, msg string) {
			l.Debug(msg)
		})

		return 0
	},
}

func loggerForEngine(eng *lua.Engine) logger.Log {
	if log, ok := eng.Meta[keys.Logger].(logger.Log); ok {
		return log
	}

	if em, ok := eng.Meta[keys.Emitter].(*events.Emitter); ok {
		return em.Log
	}

	name := "Unknown Engine"
	if n, ok := eng.Meta[keys.EngineID].(string); ok {
		name = n
	}

	l := logger.NewLogWithSource(name)
	eng.Meta[keys.Logger] = l

	return l
}

func performLog(eng *lua.Engine, fn func(logger.Log, string)) {
	data := eng.Nil()
	if eng.StackSize() >= 2 {
		data = eng.PopTable()
	}
	msg := eng.PopString()

	log := loggerForEngine(eng)

	if !data.IsNil() && data.IsTable() {
		m := data.AsMapStringInterface()
		log = log.WithFields(logger.Fields(m))
	}

	fn(log, msg)
}
