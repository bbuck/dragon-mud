package cli

import (
	"github.com/bbuck/dragon-mud/data/models"
	"github.com/bbuck/dragon-mud/logger"
	"github.com/bbuck/dragon-mud/random"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	serveCmd = &cobra.Command{
		Use:   "serve",
		Short: "Start the DragonMUD server.",
		Long: `Starts the DragonMUD Game server to listen for new player connections.
All lifecycle scripts will be notified during boot and the configuration
information will be processed.`,
		Run: func(cmd *cobra.Command, args []string) {
			logger.Infof("A %s dragon arrives to serve you today.", getDragonColor())
			logger.WithField("env", viper.GetString("env")).Info("Configuration loaded")

			err := models.MigrateDatabase()
			if err != nil {
				logger.WithField("err", err.Error()).Fatal("Failed to configure and setup database")
			}

			player := models.FindPlayerByUsername("izuriel")
			logger.WithField("match", player.IsValidPassword("password")).Info("Testing password validity")

			if player != nil {
				logger.WithField("player", player.DisplayName).Info("found player")
			} else {
				logger.WithField("username", "izuriel").Error("Failed to find the player with username")
			}
		},
	}

	dragonColors = []string{
		"{black+h:white+h}black{reset}",
		"{red+h}red{reset}",
		"{green+h}green{reset}",
		"{yellow+h}gold{reset}",
		"{blue+h}blue{reset}",
		"{white+h}white{reset}",
		"{white+u}silver{reset}",
	}
)

func init() {
	RootCmd.AddCommand(serveCmd)
}

func getDragonColor() string {
	index := random.Intn(len(dragonColors))

	return dragonColors[index]
}
