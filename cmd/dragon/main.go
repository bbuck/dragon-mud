// Copyright (c) 2016-2017 Brandon Buck

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bbuck/dragon-mud/cli"
	"github.com/bbuck/dragon-mud/config"
	"github.com/bbuck/dragon-mud/logger"
	"github.com/bbuck/dragon-mud/scripting"
	"github.com/bbuck/dragon-mud/scripting/keys"
	"github.com/bbuck/dragon-mud/scripting/lua"
)

var cmdEngine *lua.Engine

func main() {
	config.Setup(cli.RootCmd)
	config.Load()

	cmdEngine = getCommandEngine()

	if err := cli.RootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func getCommandEngine() *lua.Engine {
	eng := lua.NewEngine()
	eng.OpenLibs()
	scripting.OpenLibs(eng, "tmpl", "password", "die", "random", "log",
		"sutil", "cli")
	eng.Meta[keys.RootCmd] = cli.RootCmd

	// Set import directories
	cmdDirs := getCommandDirectories()
	pkg := eng.GetEnviron().Get("package")
	paths := pkg.Get("paths").AsString()
	newPaths := []string{paths}
	for _, cmdDir := range cmdDirs {
		dir := filepath.Dir(cmdDir)
		if strings.Contains(dir, "plugins") {
			dir = filepath.Dir(dir)
		}
		newPaths = append(newPaths, filepath.Join(dir, "?.lua"))
	}
	pkg.Set("paths", strings.Join(newPaths, ";"))
	fmt.Println(strings.Join(newPaths, ";"))

	loadCommandFiles(eng, cmdDirs)

	return eng
}

func loadCommandFiles(eng *lua.Engine, paths []string) {
	for _, path := range paths {
		mlua := filepath.Join(path, "main.lua")
		if fi, err := os.Lstat(mlua); err == nil && !fi.IsDir() {
			lerr := eng.DoFile(mlua)
			if lerr != nil {
				logger.NewWithSource("engine(commands)").WithError(lerr).WithField("filepath", mlua).Warn("failed to load commands.")

				continue
			}
		}
	}
}

func getCommandDirectories() []string {
	wd, err := os.Getwd()
	if err != nil {
		logger.NewWithSource("engine(commands)").WithError(err).Fatal("Failed to fetch working directory.")
	}

	paths := []string{filepath.Join(wd, "commands")}
	pdirPath := filepath.Join(wd, "plugins")
	if pluginDir, err := os.Open(pdirPath); err == nil {
		if pnames, derr := pluginDir.Readdirnames(-1); derr == nil {
			for _, pname := range pnames {
				paths = append(paths, filepath.Join(pdirPath, pname, "commands"))
			}
		}
	}

	return paths
}
