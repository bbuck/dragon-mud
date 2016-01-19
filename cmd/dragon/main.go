package main

import (
	"os"

	"github.com/bbuck/dragon-mud/cli"
	"github.com/bbuck/dragon-mud/output"
)

func main() {
	stderr := output.Stderr()
	if err := cli.RootCmd.Execute(); err != nil {
		stderr.Printf("[red+h]ERROR:[white+h] %s[reset]", err)
		os.Exit(1)
	}
}
