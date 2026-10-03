// Package scaffold writes the files for a new game directory.
package scaffold

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"text/template"

	"bbuck.dev/dragon-mud/builtin"
)

//go:embed template
var files embed.FS

// renames maps template files to their names in the game directory, for
// files that can't be embedded under their real names.
var renames = map[string]string{
	"gitignore": ".gitignore",
}

// Data is what the caller fills in for templates. Templates can also refer
// to .Builtins, every built-in plugin.
type Data struct {
	Name string
}

// New creates a game in dir, which must not exist yet.
func New(dir string, data Data) error {
	if _, err := os.Stat(dir); err == nil {
		return fmt.Errorf("%s already exists", dir)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	root, err := fs.Sub(files, "template")
	if err != nil {
		return err
	}

	return fs.WalkDir(root, ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		target := filepath.Join(dir, filepath.FromSlash(name))
		if base, ok := renames[path.Base(name)]; ok {
			target = filepath.Join(filepath.Dir(target), base)
		}

		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}

		contents, err := fs.ReadFile(root, name)
		if err != nil {
			return err
		}

		if strings.HasSuffix(name, ".toml") {
			contents, err = render(name, contents, data)
			if err != nil {
				return err
			}
		}

		return os.WriteFile(target, contents, 0o644)
	})
}

func render(name string, contents []byte, data Data) ([]byte, error) {
	tmpl, err := template.New(name).Parse(string(contents))
	if err != nil {
		return nil, err
	}

	// The name lands inside a TOML string; escape what TOML would reject.
	values := struct {
		Name     string
		Builtins []string
	}{
		Name:     strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(data.Name),
		Builtins: builtin.Names,
	}

	var b bytes.Buffer
	if err := tmpl.Execute(&b, values); err != nil {
		return nil, err
	}

	return b.Bytes(), nil
}
