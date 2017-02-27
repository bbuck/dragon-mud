// Copyright (c) 2016-2017 Brandon Buck

package main

import (
	"os"

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
	eng := lua.NewEngine()
	eng.OpenLibs()
	scripting.OpenLibs(eng, "tmpl", "password", "die", "random", "log",
		"sutil", "cli", "config")
	eng.Meta[keys.RootCmd] = cli.RootCmd

	plugins.RegisterLoadPaths(eng)
	err := plugins.LoadCommands(eng)
	if err != nil {
		logger.New().WithError(err).Fatal("Failed to load commands modules in plugins")
	}

	return eng
}
