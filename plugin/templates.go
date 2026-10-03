package plugin

import (
	"errors"
	"fmt"
	"io/fs"
	"regexp"
	"strings"

	"bbuck.dev/dragon-mud/view"
)

// Template directories in a plugin.
const (
	// ViewsDir holds views: say.txt.tmpl is the view "say".
	ViewsDir = "views"

	// TemplatesDir holds the other templates the engine renders, such as
	// entity_tooltip.html.tmpl.
	TemplatesDir = "templates"
)

var templateFileRx = regexp.MustCompile(`^([a-z][a-z0-9_]*)\.(txt|html)\.tmpl$`)

// Templates reads the template files in dir, one of MessagesDir or
// TemplatesDir. A plugin without dir has none.
func (p *Plugin) Templates(dir string) ([]view.File, error) {
	entries, err := fs.ReadDir(p.files, dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var files []view.File
	for _, entry := range entries {
		path := p.ID + "/" + dir + "/" + entry.Name()
		if entry.IsDir() {
			return nil, fmt.Errorf("%s is a directory, but %s/ only holds template files like say.txt.tmpl.", path, dir)
		}
		if strings.HasPrefix(entry.Name(), ".") {
			continue // editor and OS droppings, such as .DS_Store
		}

		m := templateFileRx.FindStringSubmatch(entry.Name())
		if m == nil {
			return nil, fmt.Errorf("%s isn't a template name. Name templates <name>.txt.tmpl or <name>.html.tmpl, where the name is lowercase letters, digits and underscores, like say.txt.tmpl.", path)
		}

		source, err := fs.ReadFile(p.files, dir+"/"+entry.Name())
		if err != nil {
			return nil, err
		}

		files = append(files, view.File{
			Name:   m[1],
			Format: m[2],
			Path:   path,
			Plugin: p.ID,
			Source: string(source),
		})
	}

	return files, nil
}
