// Copyright (c) 2016-2017 Brandon Buck

package scripting

import (
	"github.com/bbuck/dragon-mud/scripting/engine"
	"github.com/bbuck/dragon-mud/scripting/modules"
)

// OpenTmpl registers the tmpl module with the provided Lua engine.
func OpenTmpl(e *engine.Lua) {
	e.RegisterModule("tmpl", modules.Tmpl)
}

// OpenPassword registers the password module with the provided Lua engine.
func OpenPassword(e *engine.Lua) {
	e.RegisterModule("password", modules.Password)
}

// OpenDie opens the die module, allowing the scripts to simulate die rolls.
func OpenDie(e *engine.Lua) {
	e.RegisterModule("die", modules.Die)
}

// OpenRandom opens the random module, allowing the scripts to generate random
// numbers.
func OpenRandom(e *engine.Lua) {
	e.RegisterModule("random", modules.Random)
}
