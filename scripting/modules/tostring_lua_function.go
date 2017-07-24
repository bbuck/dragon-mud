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
