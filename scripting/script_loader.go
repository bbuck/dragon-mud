package scripting

import (
	"github.com/bbuck/dragon-mud/assets"
	"github.com/bbuck/dragon-mud/logger"
	"github.com/bbuck/dragon-mud/scripting/lua"
)

// ScriptLoader takes a string and returns a ModuleLoader capable of reading
// the script file (which is cached so it's read only once) and then loaded
// into the preloaded assets in the engine.
func ScriptLoader(scriptName string) ModuleLoader {
	script := string(assets.MustAsset("modules/fn.lua"))
	return func(eng *lua.Engine) {
		mod, err := eng.LoadString(script)
		if err != nil {
			logger.NewWithSource("script_loader").WithError(err).WithField("file", scriptName).Fatal("Failed to load script file in engine")
		}

		eng.GetEnviron().Get("package").Get("preload").RawSet("fn", mod)
	}
}
