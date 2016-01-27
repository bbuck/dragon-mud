package config

import (
	"fmt"
	"os"

	"github.com/bbuck/dragon-mud/cli"
	"github.com/spf13/viper"
)

func init() {
	registerDefaults()
	viper.SetConfigType("toml")
	viper.SetConfigName("DragonDetails")
	viper.SetEnvPrefix("dragon_mud")
	viper.AddConfigPath(".")
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

func registerDefaults() {

}

func bindEnvVars() {
	viper.BindEnv("env")
}

func bindFlags() {
	viper.BindPFlag("env", cli.RootCmd.PersistentFlags().Lookup("env"))
}
