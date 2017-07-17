package modules

import (
	"github.com/bbuck/dragon-mud/assets"
	"github.com/bbuck/dragon-mud/scripting/lua"
)

func ScriptLoader(scriptName string) func(*lua.Engine) {
	script := string(assets.MustAsset("modules/fn.lua"))
	return func(eng *lua.Engine) {
		mod, err := eng.LoadString(script)
		if err != nil {
			log("script_loader").WithError(err).WithField("file", scriptName).Fatal("Failed to load script file in engine")
		}

		eng.GetEnviron().Get("package").Get("preload").RawSet("fn", mod)
	}
}
