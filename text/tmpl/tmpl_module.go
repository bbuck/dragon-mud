package tmpl

import (
	"github.com/Sirupsen/logrus"
	"github.com/bbuck/dragon-mud/logger"
	"github.com/bbuck/dragon-mud/scripting"
)

var scriptModule = map[string]interface{}{
	"Register": func(contents, name string) bool {
		err := Register(contents, name)

		if err != nil {
			fields := logrus.Fields{
				"error": err.Error(),
			}
			if len(contents) < 255 {
				fields["tempalte"] = contents
			}
			logger.WithFields(fields).Error("Register failed from script with error")
		}

		return err == nil
	},
	"Render": func(engine *scripting.LuaEngine) int {
		data := engine.PopTable()
		name := engine.PopString()

		log := logger.WithField("name", name)

		t, err := Template(name)
		if err != nil {
			log.WithField("error", err.Error()).Error("Failed to fetch tempalte name.")

			engine.PushValue("")
			engine.PushValue(false)

			return 2
		}
		mapData := data.AsMapStringInterface()
		result, err := t.Render(mapData)
		if err != nil {
			log.WithFields(logrus.Fields{
				"error": err.Error(),
				"data":  mapData,
			}).Error("Failed to render template from requested in script.")
		}

		engine.PushValue(result)
		engine.PushValue(err == nil)

		return 2
	},
}

func RegisterScriptModules(engine *scripting.LuaEngine) {
	engine.RegisterModule("tmpl", scriptModule)
}
