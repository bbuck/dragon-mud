// Copyright 2016-2017 Brandon Buck

package plugins

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bbuck/dragon-mud/errs"
	"github.com/bbuck/dragon-mud/logger"
	"github.com/bbuck/dragon-mud/scripting/lua"
)

var (
	// Paths lists the filepaths for all accessible plugins in the current
	// project directory.
	Paths []string

	// Root is the root path of the project directory
	Root string

	// PluginRoot is the root for all plugins, essnetially just
	// Root + "/plugins"
	PluginRoot string

	// GameRoot is the root for all player written game files, essentially just
	// Root + "/game"
	GameRoot string

	// Names is a list of names for each of the plugins loaded.
	Names []string
)

// paths to use to search for lua modules to load.
var loadPaths []string

// set some values to prepare for execution and use
func init() {
	wd, err := os.Getwd()
	if err != nil {
		logger.NewWithSource("plugins").WithError(err).Error("Could not determine current working directory.")
		os.Exit(errs.ErrPluginLoad)
	}
	Root = wd
	PluginRoot = filepath.Join(Root, "plugins")
	GameRoot = filepath.Join(GameRoot, "game")
	Paths, err = filepath.Glob(filepath.Join(PluginRoot, "*"))
	if err != nil {
		logger.NewWithSource("plugins").WithError(err).Error("Could not read the 'plugins' directory.")
		os.Exit(errs.ErrPluginLoad)
	}
	for _, p := range Paths {
		Names = append(Names, strings.Replace(p, PluginRoot+string(filepath.Separator), "", 1))
	}
}

// GetScriptLoadPaths returns the paths used for loading scripts via Lua. This
// is converting all plugin paths into ?.lua and ?/init.lua paths.
func GetScriptLoadPaths() []string {
	if loadPaths == nil {
		loadPaths = []string{
			filepath.Join(Root, "?.lua"),
			filepath.Join(Root, "?", "init.lua"),
			filepath.Join(PluginRoot, "?.lua"),
			filepath.Join(PluginRoot, "?", "init.lua"),
		}
	}

	return loadPaths
}

// LoadCommands runs all the init.lua files for commands in the users codebase
// and with all plugins.
func LoadCommands(eng *lua.Engine) error {
	return loadPluginCode("commands", eng)
}

// LoadServer runs all the init.lua files for server in the users codebase
// and with all plugins.
func LoadServer(eng *lua.Engine) error {
	return loadPluginCode("server", eng)
}

// LoadClient runs all the init.lua files for client in the users codebase
// and with all plugins.
func LoadClient(eng *lua.Engine) error {
	return loadPluginCode("client", eng)
}

// run require command for the user's code and all plugins to initiate a plugin
// level. Like "commands" or "server", etc...
func loadPluginCode(kind string, eng *lua.Engine) error {
	msgs := make([]string, 0)
	if _, err := eng.Call("require", 0, kind); err != nil && !isModNotFoundError(err, kind) {
		msgs = append(msgs, err.Error())
	}
	for _, plugin := range Names {
		reqStr := fmt.Sprintf("%s.%s", plugin, kind)
		if _, err := eng.Call("require", 0, reqStr); err != nil && !isModNotFoundError(err, reqStr) {
			msgs = append(msgs, err.Error())
		}
	}

	if len(msgs) > 0 {
		return errors.New(strings.Join(msgs, "; "))
	}

	return nil
}

func isModNotFoundError(err error, mod string) bool {
	msg := fmt.Sprintf("module %s not found", mod)

	return strings.Contains(err.Error(), msg)
}
