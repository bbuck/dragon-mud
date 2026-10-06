// Package builtin embeds the plugins that ship with the engine. They're
// ordinary plugins that use only the public plugin API.
package builtin

import (
	"embed"
	"fmt"
	"io/fs"
	"maps"
	"slices"

	"bbuck.dev/dragon-mud/plugin"
)

//go:embed chat help presence characters
var files embed.FS

// Names lists the built-in plugins in load order.
var Names = []string{"chat", "help", "presence", "characters"}

// FS returns the files of the built-in plugin name.
func FS(name string) (fs.FS, error) {
	return fs.Sub(files, name)
}

// APIs lists the APIs the built-in plugins provide, sorted: the engine's
// dragon: APIs, which other plugins may provide in a built-in's place.
func APIs() ([]string, error) {
	var apis []string
	for _, name := range Names {
		fsys, err := FS(name)
		if err != nil {
			return nil, err
		}
		m, err := plugin.ReadManifest(fsys)
		if err != nil {
			return nil, fmt.Errorf("built-in plugin %s: %w", name, err)
		}
		apis = append(apis, slices.Collect(maps.Keys(m.Provides))...)
	}
	slices.Sort(apis)

	return apis, nil
}
