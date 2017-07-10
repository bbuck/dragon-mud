package modules

import (
	"github.com/bbuck/dragon-mud/logger"
	"github.com/bbuck/dragon-mud/scripting/lua"
	"github.com/bbuck/dragon-mud/text/tmpl"
)

// Tmpl is the templating module accessible in scripts. This module consists of
// two accessible methods:
//   register(name, body)
//     @param name: string = the name to associate with this template after
//     @param body: string = the uncompiled body of the template
//       registration
//     register a template with the given name
//   render(name, data)
//     @param name: string = the name of the compiled template to use for
//       generating output
//     @param data: table = a table of data to provide to the rendering of the
//       named templates
//     render the template with the given name using the given data to populate
//     it
//   render_in_layout(layout, children, data)
//     @param layout: string = the name of the layout template to render.
//     @param children: string or table = the children to render in the layout.
//       if provided as a string, then the name to use in the layout is
//       'content', otherise this is table of field names -> template names to
//       use in generating layout content.
//     @param data: table = a table of data to provide to the rendering of the
//       named templates (used for all views, so must be merged)
//     render the child templates with the provided and building an additional
//     set of data containing the rendered children before rendering the final
//     layout template which can position the child templates via normal
//     rendering means.
var Tmpl = lua.TableMap{
	"register": func(name, contents string) bool {
		err := tmpl.Register(name, contents)

		if err != nil {
			fields := logger.Fields{
				"error": err.Error(),
			}
			if len(contents) < 255 {
				fields["template"] = contents
			}
			log("tmpl").WithFields(fields).Error("Register failed from script with error")
		}

		return err == nil
	},
	"render": func(engine *lua.Engine) int {
		data := engine.PopTable().AsMapStringInterface()
		name := engine.PopString()

		log := log("tmpl").WithField("tmpl_name", name)

		t, err := tmpl.Template(name)
		if err != nil {
			log.WithError(err).Error("Failed to fetch template name.")
			engine.RaiseError(err.Error())

			return 0
		}
		result, err := t.Render(data)
		if err != nil {
			log.WithFields(logger.Fields{
				"error": err.Error(),
				"data":  data,
			}).Error("Failed to render template from requested in script.")
		}

		engine.PushValue(result)

		return 1
	},
	"render_in_layout": func(eng *lua.Engine) int {
		ldata := eng.PopValue()
		children := eng.PopValue()
		parent := eng.PopString()

		pt, err := tmpl.Template(parent)
		if err != nil {
			log("tmpl").WithError(err).WithField("template", parent).Warn("Parent template requested but undefined, returning empty string.")
			eng.PushValue("")

			return 1
		}

		var data map[string]interface{}
		if ldata.IsTable() {
			data = ldata.AsMapStringInterface()
		} else {
			data = make(map[string]interface{})
		}

		switch {
		case children.IsString():
			cs := children.AsString()
			r, err := tmpl.Template(cs)
			// default child name is content in the case of single strings
			if err != nil {
				log("tmpl").WithError(err).WithField("tempalte", cs).Warn("Template requested, but doesn't exit. Using empty string.")
				data["content"] = ""
			} else {
				data["content"], err = r.Render(data)
				if err != nil {
					log("tmpl").WithError(err).WithField("template", cs).Error("Failed to render template")
					data["content"] = ""
				}
			}
		case children.IsTable():
			children.ForEach(func(key, val *lua.Value) {
				if key.IsString() {
					ks := key.AsString()
					if val.IsString() {
						vs := val.AsString()
						r, err := tmpl.Template(vs)
						if err != nil {
							log("tmpl").WithError(err).WithField("tempalte", vs).Warn("Template requested, but doesn't exit. Using empty string.")
							data[ks] = ""

							return
						}
						data[ks], err = r.Render(data)
						if err != nil {
							log("tmpl").WithError(err).WithField("template", ks).Error("Failed to render template.")
							data[ks] = ""
						}
					} else {
						log("tmpl").WithFields(logger.Fields{
							"template": ks,
							"type":     val.String(),
						}).Warn("Non-string value given as name of template, using empty string.")
						data[ks] = ""
					}
				} else {
					log("tmpl").WithField("type", key.String()).Warn("Non-string key provided as key of rendered template, ignoring")
				}
			})
		}

		res, err := pt.Render(data)
		if err != nil {
			log("tmpl").WithError(err).WithField("template", pt).Error("Failed to render parent template, returning empty string.")
			eng.PushValue("")

			return 1
		}

		eng.PushValue(res)

		return 1
	},
}
