// Copyright (c) 2016-2017 Brandon Buck

package cli

import (
	"github.com/bbuck/dragon-mud/config"
	"github.com/bbuck/dragon-mud/logger"
	"github.com/bbuck/dragon-mud/telnet/server"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the DragonMUD server.",
	Long: `Starts the DragonMUD Game server to listen for new player connections.
All lifecycle scripts will be notified during boot and the configuration
information will be processed.`,
	Run: func(cmd *cobra.Command, args []string) {
		log := logger.NewWithSource("cmd(serve)")

		dragon := getRandomDragonDetails()
		log.WithField("color", dragon.name).Info("A dragon arrives to serve you today.")
		if !config.Loaded {
			log.Fatal("No configuration file detected. Make sure you run {W}dragon init{x} first.")
		}
		log.WithField("env", viper.GetString("env")).Info("Configuration loaded")

		// TODO: Implement serve command
		server.Run()
	},
}

func init() {
	RootCmd.AddCommand(serveCmd)
}
