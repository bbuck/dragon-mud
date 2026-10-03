// Package builtin embeds the plugins that ship with the engine. They're
// ordinary plugins that use only the public plugin API.
package builtin

import (
	"embed"
	"io/fs"
)

//go:embed chat help presence characters character-creation
var files embed.FS

// Names lists the built-in plugins in load order.
var Names = []string{"chat", "help", "presence", "characters", "character-creation"}

// FS returns the files of the built-in plugin name.
func FS(name string) (fs.FS, error) {
	return fs.Sub(files, name)
}
