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

var (
	templateFileRx = regexp.MustCompile(`^([a-z][a-z0-9_]*)\.(txt|html)\.tmpl$`)
	templateDirRx  = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
)

// Templates reads the template files in dir, one of ViewsDir or
// TemplatesDir, and the directories inside it. A file's name is its path
// under dir without the extension: views/chat/say.txt.tmpl is the view
// "chat/say". A plugin without dir has none.
func (p *Plugin) Templates(dir string) ([]view.File, error) {
	if _, err := fs.Stat(p.files, dir); errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}

	var files []view.File
	err := fs.WalkDir(p.files, dir, func(file string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if file == dir {
			return nil
		}
		path := p.ID + "/" + file
		if strings.HasPrefix(entry.Name(), ".") {
			if entry.IsDir() {
				return fs.SkipDir
			}
			return nil // editor and OS droppings, such as .DS_Store
		}
		if entry.IsDir() {
			if !templateDirRx.MatchString(entry.Name()) {
				return fmt.Errorf("%s isn't a template directory name. Name directories with lowercase letters, digits and underscores, like %s/chat/, so their templates have names like chat/say.", path, dir)
			}
			return nil
		}

		m := templateFileRx.FindStringSubmatch(entry.Name())
		if m == nil {
			return fmt.Errorf("%s isn't a template name. Name templates <name>.txt.tmpl or <name>.html.tmpl, where the name is lowercase letters, digits and underscores, like say.txt.tmpl.", path)
		}

		source, err := fs.ReadFile(p.files, file)
		if err != nil {
			return err
		}

		name := strings.TrimPrefix(file, dir+"/")
		name = strings.TrimSuffix(name, entry.Name()) + m[1]
		files = append(files, view.File{
			Name:   name,
			Format: m[2],
			Path:   path,
			Plugin: p.ID,
			Source: string(source),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}

	return files, nil
}
