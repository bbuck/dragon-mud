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

			// TODO: Implement serve command
		},
	}

	dragonColors = []string{
		"{b,-W}black{x}",
		"{R}red{x}",
		"{G}green{x}",
		"{c220}gold{x}",
		"{B}blue{x}",
		"{W}white{x}",
		"{w,u}silver{x}",
	}
)

func init() {
	RootCmd.AddCommand(serveCmd)
}

func getDragonColor() string {
	index := random.Intn(len(dragonColors))

	return dragonColors[index]
}
