package tmpl

import (
	"fmt"
	"html/template"
	"io"

	"github.com/bbuck/dragon-mud/ansi"
	"github.com/fatih/structs"
	"github.com/gobuffalo/velvet"
)

// define velvet Helpers
func init() {
	velvet.Helpers.Add("purge", func(s string) template.HTML {
		s = ansi.Purge(s)

		return template.HTML(s)
	})
}

// InvalidDataError represents that a data value was provided of an unexpected
// type.
type InvalidDataError string

// Error returns an error message that represents the type of data that was
// received and what is expected.
func (i InvalidDataError) Error() string {
	return fmt.Sprintf("invalid data given to template, got %q expected map[string]interface{}", string(i))
}

// renderer for the velvet implementation of handlebars
type velvetRenderer struct {
	template *velvet.Template
}

// Render will render the velvet handlebar template with the given data as it's
// context.
func (vr *velvetRenderer) Render(data interface{}) (string, error) {
	var ctx *velvet.Context
	switch t := data.(type) {
	case map[string]interface{}:
		ctx = velvet.NewContextWith(t)
	case *velvet.Context:
		ctx = t
	}

	if ctx == nil && data == nil {
		ctx = velvet.NewContext()
	}

	if ctx == nil {
		if structs.IsStruct(data) {
			ctx = velvet.NewContextWith(structs.Map(data))
		} else {
			return "", InvalidDataError(fmt.Sprintf("%T", data))
		}
	}

	res, err := vr.template.Exec(ctx)
	if err != nil {
		return "", err
	}

	return res, nil
}

// RenderTo will render the template to the writer provided.
func (vr *velvetRenderer) RenderTo(w io.Writer, data interface{}) error {
	res, err := vr.Render(data)
	if err != nil {
		return err
	}

	fmt.Fprint(w, res)

	return nil
}
