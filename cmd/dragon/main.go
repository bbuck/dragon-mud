package main

import (
	"os"

	"github.com/bbuck/dragon-mud/cli"
	"github.com/bbuck/dragon-mud/config"
)

func main() {
	config.Load()
	if err := cli.RootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
