// Copyright (c) 2016-2017 Brandon Buck

package scripting

import (
	"github.com/bbuck/dragon-mud/scripting/lua"
	"github.com/bbuck/dragon-mud/scripting/modules"
)

// OpenTmpl registers the tmpl module with the provided Lua engine.
func OpenTmpl(e *lua.Engine) {
	e.RegisterModule("tmpl", modules.Tmpl)
}

// OpenPassword registers the password module with the provided Lua engine.
func OpenPassword(e *lua.Engine) {
	e.RegisterModule("password", modules.Password)
}

// OpenDie opens the die module, allowing the scripts to simulate die rolls.
func OpenDie(e *lua.Engine) {
	e.RegisterModule("die", modules.Die)
}

// OpenRandom opens the random module, allowing the scripts to generate random
// numbers.
func OpenRandom(e *lua.Engine) {
	e.RegisterModule("random", modules.Random)
}

// OpenEvents opens the events module, making it possible for the engine to emit
// and receive events. This requires the use of a pool though, due to keep
// emissions and handler execution thread safe.
func OpenEvents(e *lua.Engine) {
	e.RegisterModule("events", modules.Events)
}

// OpenLog will register the log module which will enable server scripts to
// log information directly to the user specified log targets.
func OpenLog(e *lua.Engine) {
	e.RegisterModule("log", modules.Log)
}
