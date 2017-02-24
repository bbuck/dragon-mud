// Copyright (c) 2016-2017 Brandon Buck

package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/bbuck/dragon-mud/fs"
	"github.com/bbuck/dragon-mud/logger"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Configure the current directory to prepare for running a DragonMUD server.",
	Long: `Configure and setup the current directory for use as the base directory
for a DragonMUD game server. This will build the necessary folder structure and
copy in required configuration files with defaults set ready for you to get
started.`,
	Run: func(_ *cobra.Command, _ []string) {
		log := logger.NewWithSource("cmd(init)")

		var gameName string
		fmt.Print("Enter the name of your game >> ")
		fmt.Scanf("%s", &gameName)
		tmplData := map[string]interface{}{
			"game_title": strings.Title(gameName),
			"game_name":  strings.ToLower(gameName),
		}

		wd, err := os.Getwd()
		if err != nil {
			log.WithError(err).Fatal("Failed to fetch current working directory, cannot initialize project.")
		}

		fs.CreateFromStructure(fs.CreateStructureParams{
			Log:          log,
			Structure:    fs.ProjectStructure,
			BaseName:     wd,
			TemplateData: tmplData,
		})
	},
}

func init() {
	RootCmd.AddCommand(initCmd)
}
