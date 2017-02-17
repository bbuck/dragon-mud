// Copyright (c) 2016-2017 Brandon Buck

package tmpl

import (
	"io"
	"reflect"

	"github.com/bbuck/dragon-mud/scripting/engine"
	"github.com/fatih/structs"
	"github.com/valyala/fasttemplate"
)

type fastTemplateRenderer struct {
	template *fasttemplate.Template
}

// Render will use the fasttemplates ExecuteString method to produce a string
// from the template and data provided.
func (f *fastTemplateRenderer) Render(data interface{}) (string, error) {
	result := f.template.ExecuteString(ifaceToMap(data))

	return result, nil
}

// RenderTo will use the fasttemplates Execute method to render a template to
// and io.Writer using the data provided.
func (f *fastTemplateRenderer) RenderTo(w io.Writer, data interface{}) error {
	_, err := f.template.Execute(w, ifaceToMap(data))

	return err
}

func ifaceToMap(i interface{}) map[string]interface{} {
	if m, ok := i.(map[string]interface{}); ok {
		return m
	}

	if reflect.ValueOf(i).Kind() == reflect.Struct {
		st := structs.New(i)

		return st.Map()
	}

	if lv, ok := i.(*engine.LuaValue); ok {
		if lv.IsTable() {
			return lv.AsMapStringInterface()
		}
	}

	return nil
}
