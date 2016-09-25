package tmpl

import (
	"io"

	"github.com/valyala/fasttemplate"
)

type fastTemplateRenderer struct {
	template *fasttemplate.Template
}

// Render will use the fasttemplates ExecuteString method to produce a string
// from the template and data provided.
func (f *fastTemplateRenderer) Render(data map[string]interface{}) (string, error) {
	result := f.template.ExecuteString(data)

	return result, nil
}

// RenderTo will use the fasttemplates Execute method to render a template to
// and io.Writer using the data provided.
func (f *fastTemplateRenderer) RenderTo(w io.Writer, data map[string]interface{}) error {
	_, err := f.template.Execute(w, data)

	return err
}
