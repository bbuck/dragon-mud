// Copyright (c) 2016-2017 Brandon Buck

package main

import (
	"os"
	"strings"

	"github.com/bbuck/dragon-mud/cli"
	"github.com/bbuck/dragon-mud/config"
	"github.com/bbuck/dragon-mud/errs"
	"github.com/bbuck/dragon-mud/logger"
	"github.com/bbuck/dragon-mud/plugins"
	"github.com/bbuck/dragon-mud/scripting"
	"github.com/bbuck/dragon-mud/scripting/keys"
	"github.com/bbuck/dragon-mud/scripting/lua"
)

func main() {
	config.Setup(cli.RootCmd)
	config.Load()

	cmdEngine := getCommandEngine()
	defer cmdEngine.Close()

	if err := cli.RootCmd.Execute(); err != nil {
		os.Exit(errs.ErrGeneral)
	}
}

func getCommandEngine() *lua.Engine {
	log := logger.NewWithSource("command_engine")

	eng := lua.NewEngine(lua.EngineOptions{
		FieldNaming:  lua.SnakeCaseNames,
		MethodNaming: lua.SnakeCaseNames,
	})
	eng.OpenLibs()
	scripting.OpenLibs(eng, "*", "-events")
	eng.Meta[keys.RootCmd] = cli.RootCmd

	eng.SecureRequire(plugins.GetScriptLoadPaths())
	err := plugins.LoadCommands(eng)
	if err != nil {
		if !strings.Contains(err.Error(), "commands") {
			log.WithError(err).Fatal("Failed to load commands modules in plugins")
		}
	}

	eng.SetGlobal("print", log.Info)

	return eng
}
