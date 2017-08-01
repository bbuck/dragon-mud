package modules

import "github.com/bbuck/dragon-mud/scripting/lua"

func goToString(engine *lua.Engine) int {
	obj := engine.PopValue()

	if obj.IsUserData() {
		if i, ok := obj.Interface().(lua.Inspecter); ok {
			engine.PushValue(i.Inspect(""))

			return 1
		}
	}

	engine.PushValue("%T is not a value.Inspecter!")

	return 1
}

func goLuaToString(toStringFn lua.ScriptFunction) lua.ScriptFunction {
	return func(engine *lua.Engine) int {
		val := engine.PopValue()
		if val.IsUserData() {
			engine.PushValue(val)
			engine.PushValue("")
			toStringFn(engine)
		} else {
			results, err := val.Invoke("inspect", 1, "")
			if err == nil {
				log("go -> lua __to_string").WithError(err).Error("Calling inspect from __to_string failed.")
				engine.PushValue("")

				return 1
			}

			engine.PushValue(results[0])
		}

		return 1
	}
}
