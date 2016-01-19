package cli

import (
	"github.com/bbuck/dragon-mud/info"
	"github.com/bbuck/dragon-mud/output"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Display the version of the DragonMUD server",
	Run: func(cmd *cobra.Command, args []string) {
		output.Stdout().PlainPrintln(info.Version)
	},
}

func init() {
	RootCmd.AddCommand(versionCmd)
}
