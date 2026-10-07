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

//go:embed plugin
var pluginFiles embed.FS

// Plugin creates a local plugin called name in dir, which must not exist
// yet: a manifest, an init.lua with a command and handlers, and a test
// for the command. Every file is a template filled in with the name.
func Plugin(dir, name string) error {
	if _, err := os.Stat(dir); err == nil {
		return fmt.Errorf("%s already exists", dir)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	root, err := fs.Sub(pluginFiles, "plugin")
	if err != nil {
		return err
	}

	return fs.WalkDir(root, ".", func(file string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		target := filepath.Join(dir, filepath.FromSlash(file))
		if file == "tests/plugin_test.lua" {
			target = filepath.Join(dir, "tests", strings.ReplaceAll(name, "-", "_")+"_test.lua")
		}
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}

		source, err := fs.ReadFile(root, file)
		if err != nil {
			return err
		}
		tmpl, err := template.New(file).Parse(string(source))
		if err != nil {
			return err
		}
		var b bytes.Buffer
		if err := tmpl.Execute(&b, struct{ Name string }{name}); err != nil {
			return err
		}

		return os.WriteFile(target, b.Bytes(), 0o644)
	})
}

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
