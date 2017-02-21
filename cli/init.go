// Copyright (c) 2016-2017 Brandon Buck

package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bbuck/dragon-mud/assets"
	"github.com/bbuck/dragon-mud/logger"
	"github.com/bbuck/dragon-mud/text/tmpl"
	"github.com/spf13/cobra"
)

var projectStructure = dir{
	"Dragonfile.toml": file("Dragonfile.toml"),
	"Plugins":         dir{},
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Configure the current directory to prepare for running a DragonMUD server.",
	Long: `Configure and setup the current directory for use as the base directory
for a DragonMUD game server. This will build the necessary folder structure and
copy in required configuration files with defaults set ready for you to get
started.`,
	Run: func(_ *cobra.Command, _ []string) {
		var gameName string
		fmt.Print("Enter the name of your game >> ")
		fmt.Scanf("%s", &gameName)
		tmplData := map[string]interface{}{
			"game_title": strings.Title(gameName),
			"game_name":  strings.ToLower(gameName),
		}

		writeStructure("", projectStructure, tmplData)
	},
}

func init() {
	RootCmd.AddCommand(initCmd)
}

func writeStructure(basePath string, structure dir, tmplData map[string]interface{}) {
	for name, itemInfo := range structure {
		switch itemInfo.Type() {
		case itemTypeFile:
			writeFile(filepath.Join(basePath, name), itemInfo.(file), tmplData)
		case itemTypeDir:
			writeDir(filepath.Join(basePath, name), itemInfo.(dir), tmplData)
		}
	}
}

func writeFile(name string, f file, tmplData map[string]interface{}) {
	flog := logger.NewLog().WithField("filename", name)

	assetName := string(f)
	fdata := assets.MustAsset(assetName)
	fdata = []byte(tmpl.MustRenderOnce(string(fdata), tmplData))

	flog.Info("Creating file     ")
	file, err := os.Create(name)
	if err != nil && !os.IsExist(err) {
		flog.WithField("error", err.Error()).Fatal("Failed to create file in the current directory.")

		return
	}
	defer file.Close()
	// we check error again here in case there is already a file
	if err == nil {
		n, werr := file.Write(fdata)
		if werr != nil {
			flog.WithField("error", werr.Error()).Fatal("Failed to write a default file.")
		} else if n != len(fdata) {
			flog.WithField("percentage", (float64(n) / float64(len(fdata)) * 100.0)).Fatal("Failed to write the entire file.")
		}
	}
}

func writeDir(name string, d dir, tmplData map[string]interface{}) {
	dlog := logger.NewLog().WithField("dirname", name)

	dlog.Info("Creating directory")
	err := os.Mkdir(name, os.ModePerm)
	if err != nil {
		dlog.WithField("error", err.Error()).Fatal("Failed to create directory.")
	}

	writeStructure(name, d, tmplData)
}

// filesystem types

type itemType uint8

const (
	itemTypeFile itemType = 1 + iota
	itemTypeDir
)

type item interface {
	Type() itemType
}

// A string to the asset name to copy into this location.
type file string

func (file) Type() itemType {
	return itemTypeFile
}

// a mapping of names to values to store in that location
type dir map[string]item

func (dir) Type() itemType {
	return itemTypeDir
}
