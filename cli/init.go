package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/bbuck/dragon-mud/assets"
	"github.com/bbuck/dragon-mud/config"
	"github.com/bbuck/dragon-mud/logger"
	"github.com/bbuck/dragon-mud/text/tmpl"
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
		log := logger.LogWithSource("init cmd")
		gamefile := assets.MustAsset("Dragonfile.toml")
		var gameName string
		fmt.Print("Enter the name of your game >> ")
		fmt.Scanf("%s", &gameName)
		gamefile = []byte(tmpl.MustRenderOnce(string(gamefile), map[string]interface{}{
			"game_title": strings.Title(gameName),
			"game_name":  strings.ToLower(gameName),
		}))

		file, err := os.Create("Dragonfile.toml")
		if err != nil && !os.IsExist(err) {
			log.WithField("error", err.Error()).Fatal("Failed to create a Dragonfile.toml in the current directory.")
			return
		}
		defer file.Close()
		// we check error again here in case there is already a file
		if err == nil {
			n, werr := file.Write(gamefile)
			if werr != nil {
				log.WithField("error", werr.Error()).Fatal("Failed to write the default Dragonfile.toml.")
				return
			} else if n != len(gamefile) {
				log.WithField("percentage", (float64(n) / float64(len(gamefile)) * 100.0)).Fatal("Failed to write the entire file config file.")
				return
			}
		}

		log.Info("Copied Dragonfile.toml into the current directory.")
		config.Load()
		log.Info("Loaded new configuration")
		if err != nil {
			log.WithField("error", err.Error()).Fatal("Failed to configure and setup database")
			return
		}
		log.Info("Migrated the database in preperation for execution.")
	},
}

func init() {
	RootCmd.AddCommand(initCmd)
}
