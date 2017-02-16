// Copyright (c) 2016-2017 Brandon Buck

package cli

import "github.com/spf13/cobra"

// RootCmd is the root command for the command line interface. An entry point
// for the remaind of the CLI.
var RootCmd = &cobra.Command{
	Use:   "dragon",
	Short: "DragonMUD is a Go based MUD server library.",
	Long: `An extensible and scriptable MUD server library for building and running your
dream MUD. Write scripts for several server events in Lua and once in game you can
script all of your in game scripts are also written in Lua.`,
}

func init() {
	RootCmd.PersistentFlags().StringP("env", "E", "development", "Specify the execution environment for the game server, default is 'development'")
}
