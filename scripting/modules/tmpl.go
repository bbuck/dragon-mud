package modules

import (
	"github.com/Sirupsen/logrus"
	"github.com/bbuck/dragon-mud/logger"
	"github.com/bbuck/dragon-mud/scripting/lua"
	"github.com/bbuck/dragon-mud/text/tmpl"
)

var tmplLog = logger.LogWithSource("lua(tmpl)")

// Tmpl is the templating module accessible in scripts. This module consists of
// two accessible methods:
//   register(body: string, name: string)
//     register a template with the given name
//   render(name: string, data: table)
//     render the template with the given name using the given data to populate
//     it
var Tmpl = map[string]interface{}{
	"register": func(contents, name string) bool {
		err := tmpl.Register(contents, name)

		if err != nil {
			fields := logrus.Fields{
				"error": err.Error(),
			}
			if len(contents) < 255 {
				fields["tempalte"] = contents
			}
			tmplLog.WithFields(fields).Error("Register failed from script with error")
		}

		return err == nil
	},
	"render": func(engine *lua.Engine) int {
		data := engine.PopTable()
		name := engine.PopString()

		log := tmplLog.WithField("tmpl_name", name)

		t, err := tmpl.Template(name)
		if err != nil {
			log.WithField("error", err.Error()).Error("Failed to fetch template name.")

			engine.PushValue("")
			engine.PushValue(false)

			return 2
		}
		result, err := t.Render(data)
		if err != nil {
			log.WithFields(logrus.Fields{
				"error": err.Error(),
				"data":  data.AsMapStringInterface(),
			}).Error("Failed to render template from requested in script.")
		}

		engine.PushValue(result)
		engine.PushValue(err == nil)

		return 2
	},
}
