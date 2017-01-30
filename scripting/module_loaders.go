package scripting

import (
	"github.com/bbuck/dragon-mud/scripting/engine"
	"github.com/bbuck/dragon-mud/scripting/modules"
)

// OpenTmpl registers the tmpl module with the provided Lua engine.
func OpenTmpl(e *engine.Lua) {
	e.RegisterModule("tmpl", modules.Tmpl)
}
