package tmpl

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/valyala/fasttemplate"
)

const (
	openTemplateTags  = "%{"
	closeTemplateTags = "}"
)

var compiledTemplates = make(map[string]Renderer)

// Register will compile and register a template using the string given and
// store the compiled template in the map.
func Register(contents, name string) error {
	template, err := fasttemplate.NewTemplate(contents, openTemplateTags, closeTemplateTags)
	if err != nil {
		return err
	}
	compiledTemplates[name] = &fastTemplateRenderer{template}

	return nil
}

// RegisterFile will compile and register a template using the contents of a
// file and storing by name in the compiled map.
func RegisterFile(filename, name string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()
	contents, err := ioutil.ReadAll(file)
	if err != nil {
		return err
	}

	return Register(string(contents), name)
}

// Unregister removes a previously compiled template, normally only used for
// testing purposes but in cases a template is, for some reason, miscompiled
// (although longer term solutions should be much more preferred than
// unregistering)
func Unregister(name string) {
	delete(compiledTemplates, name)
}

// Template returns the Renderer associated with a registered template, if any.
func Template(name string) (Renderer, error) {
	if tmpl, ok := compiledTemplates[name]; ok {
		return tmpl, nil
	}

	return nil, fmt.Errorf("No template has been registered with the name \"%s\"", name)
}
