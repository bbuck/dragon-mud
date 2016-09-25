package cli

import (
	"os"

	"github.com/bbuck/dragon-mud/assets"
	"github.com/bbuck/dragon-mud/config"
	"github.com/bbuck/dragon-mud/data/migrator"
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
		gamefile := assets.MustAsset("Gamefile.toml")
		file, err := os.Create("Gamefile.toml")
		if err != nil && !os.IsExist(err) {
			logger.WithField("error", err.Error()).Fatal("Failed to create a Gamefile.toml in the current directory.")
			return
		}
		defer file.Close()
		// we check error again here in case there is already a file
		if err == nil {
			n, werr := file.Write(gamefile)
			if werr != nil {
				logger.WithField("error", werr.Error()).Fatal("Failed to write the default Gamefile.toml.")
				return
			} else if n != len(gamefile) {
				logger.WithField("percentage", (float64(n) / float64(len(gamefile)) * 100.0)).Fatal("Failed to write the entire file config file.")
				return
			}
		}

		logger.Info("Copied Gamefile.toml into the current directory.")
		config.Load(RootCmd)
		logger.Info("Loaded new configuration")
		err = migrator.MigrateDatabase()
		if err != nil {
			logger.WithField("error", err.Error()).Fatal("Failed to configure and setup database")
			return
		}
		logger.Info("Migrated the database in preperation for execution.")
	},
}

func init() {
	RootCmd.AddCommand(initCmd)
}
