package cli

import (
	"github.com/bbuck/dragon-mud/data/migrator"
	"github.com/bbuck/dragon-mud/logger"
	"github.com/bbuck/dragon-mud/random"
	"github.com/bbuck/dragon-mud/server"
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

			err := migrator.MigrateDatabase()
			if err != nil {
				logger.WithField("err", err.Error()).Fatal("Failed to configure and setup database")
			}

			// TODO: Implement serve command
			server.Run()
		},
	}

	dragonColors = []string{
		"{l,-W}black{x}",
		"{c220}brass{x}",
		"{R}red{x}",
		"{c208}bronze{x}",
		"{G}green{x}",
		"{Y}gold{x}",
		"{B}blue{x}",
		"{c202}copper{x}",
		"{W}white{x}",
		"{c250,u}silver{x}",
	}
)

func init() {
	RootCmd.AddCommand(serveCmd)
}

func getDragonColor() string {
	index := random.Intn(len(dragonColors))

	return dragonColors[index]
}
