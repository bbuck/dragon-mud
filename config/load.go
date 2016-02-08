package config

import (
	"fmt"
	"os"

	"github.com/bbuck/dragon-mud/cli"
	"github.com/spf13/viper"
)

// Load will initiate the loading of the configuration file for use by the
// application. Reading settings from the config file into the configuration
// manager.
func Load() {
	registerDefaults()
	viper.SetConfigType("toml")
	viper.AddConfigPath(".")
	viper.SetConfigName("Gamefile")
	viper.SetEnvPrefix("dragon_mud")
	if err := viper.ReadInConfig(); err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, " WARN: Config file not found, using defaults. It's wise to run `dragon init`\n")
		} else {
			fmt.Fprintf(os.Stderr, "ERROR: Error loading configuration: %s\n", err)
			os.Exit(1)
		}
	}
	bindFlags()
	bindEnvVars()
}

func registerDefaults() {}

func bindEnvVars() {
	viper.BindEnv("env")
}

func bindFlags() {
	viper.BindPFlag("env", cli.RootCmd.PersistentFlags().Lookup("env"))
}
