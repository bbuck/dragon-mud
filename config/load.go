// Copyright (c) 2016-2017 Brandon Buck

package config

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	Loaded = false

	successfulRead = false
)

// Setup configures Viper and prepares all the default settings. Setting up
// the configuration to load from the environment and from flags.
func Setup(rootCmd *cobra.Command) {
	RegisterDefaults()
	viper.SetConfigType("toml")
	viper.AddConfigPath(".")
	viper.SetConfigName("Dragonfile")
	viper.SetEnvPrefix("dragon_mud")
	bindFlags(rootCmd)
	bindEnvVars()
}

// Load will initiate the loading of the configuration file for use by the
// application. Reading settings from the config file into the configuration
// manager.
func Load() {
	if err := viper.ReadInConfig(); err != nil {
		Loaded = false

		return
	}

	Loaded = true
}

// RegisterDefaults will load the defualt values in for the keys into Viper.
func RegisterDefaults() {
	viper.SetDefault("crypto.cost", 10)

	viper.SetDefault("env", "development")

	// database defaults
	viper.SetDefault("database.development.host", "localhost")
	viper.SetDefault("database.development.username", "neo4j")
	viper.SetDefault("database.development.port", 7687)
}

func bindEnvVars() {
	viper.BindEnv("env")
}

func bindFlags(rootCmd *cobra.Command) {
	if rootCmd == nil {
		return
	}

	viper.BindPFlag("env", rootCmd.PersistentFlags().Lookup("env"))
}
