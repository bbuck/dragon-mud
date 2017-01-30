package modules

import (
	"github.com/Sirupsen/logrus"
	"github.com/bbuck/dragon-mud/logger"
	"github.com/bbuck/dragon-mud/scripting/engine"
	"github.com/bbuck/dragon-mud/text/tmpl"
)

// Tmpl is the templating module accessible in scripts. This module consists of
// two accessible methods:
//   Register(body: string, name: string)
//     register a template with the given name
//   Render(name: string, data: table)
//     render the template with the given name using the given data to populate
//     it
var Tmpl = map[string]interface{}{
	"Register": func(contents, name string) bool {
		err := tmpl.Register(contents, name)

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
	"Render": func(engine *engine.Lua) int {
		data := engine.PopTable()
		name := engine.PopString()

		log := logger.WithField("name", name)

		t, err := tmpl.Template(name)
		if err != nil {
			log.WithField("error", err.Error()).Error("Failed to fetch template name.")

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
