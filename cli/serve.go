package cli

import (
	"github.com/bbuck/dragon-mud/color"
	"github.com/bbuck/dragon-mud/log"
	"github.com/bbuck/dragon-mud/random"
	"github.com/spf13/cobra"
)

var (
	serveCmd = &cobra.Command{
		Use:   "serve",
		Short: "Start the DragonMUD server.",
		Long: `Starts the DragonMUD Game server to listen for new player connections.
All lifecycle scripts will be notified during boot and the configuration
information will be processed.`,
		Run: func(cmd *cobra.Command, args []string) {
			dragonColor := getDragonColor()
			log.Logger().Infof("A %s dragon arrives to serve you today.\n", dragonColor)
		},
	}

	dragonColors = []string{
		color.ColorizeWithCode("black+h:white+h", "black"),
		color.ColorizeWithCode("red+h", "red"),
		color.ColorizeWithCode("green+h", "green"),
		color.ColorizeWithCode("yellow+h", "gold"),
		color.ColorizeWithCode("blue+h", "blue"),
		color.ColorizeWithCode("white+h", "white"),
		color.ColorizeWithCode("white+u", "silver"),
	}
)

func init() {
	RootCmd.AddCommand(serveCmd)
}

func getDragonColor() string {
	index := random.Int(len(dragonColors))

	return dragonColors[index]
}
