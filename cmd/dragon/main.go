package main

import (
	"os"

	"github.com/bbuck/dragon-mud/cli"
	_ "github.com/bbuck/dragon-mud/config"
)

func main() {
	if err := cli.RootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
