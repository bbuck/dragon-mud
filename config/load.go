package config

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var successfulRead = false

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
func Load() bool {
	if err := viper.ReadInConfig(); err != nil {
		if !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "ERROR: Error loading configuration: %s\n", err)
			os.Exit(1)
		}

		return false
	}

	return true
}

// RegisterDefaults will load the defualt values in for the keys into Viper.
func RegisterDefaults() {
	viper.SetDefault("crypto.password_memory_size", 4096)
	viper.SetDefault("crypto.password_length", 32)
	viper.SetDefault("crypto.min_iterations", 3)
	viper.SetDefault("crypto.max_iterations", 8)
}

func bindEnvVars() {
	viper.BindEnv("env")
}

func bindFlags(rootCmd *cobra.Command) {
	viper.BindPFlag("env", rootCmd.PersistentFlags().Lookup("env"))
}
