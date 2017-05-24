package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"strings"

	"github.com/bbuck/dragon-mud/ansi"
	"github.com/bbuck/dragon-mud/logger"
	"github.com/bbuck/dragon-mud/scripting"
	"github.com/bbuck/dragon-mud/scripting/lua"
)

var (
	level   string
	replCmd = &cobra.Command{
		Use:   "console",
		Short: "Run a REPL at the requested security level allowing for access to Lua code.",
		Long: `Provide real time access to a Lua engine via a Read-Eval-Print-Loop method
	giving access to the plugins at the given security level and the various built
	in libraries for quick testing.`,
		Aliases: []string{"repl", "c"},
		Run: func(*cobra.Command, []string) {
			log := logger.NewWithSource("cmd(repl)")
			log.Debug("Starting read-eval-print-loop")

			dragon := getRandomDragonDetails()
			log = log.WithField("color", dragon.name)
			log.Debug("A dragon has appeared to handle your Lua code.")

			// TODO: Add security level specic engine creation here
			eng := lua.NewEngine(lua.EngineOptions{
				FieldNaming:  lua.SnakeCaseNames,
				MethodNaming: lua.SnakeCaseNames,
			})
			scripting.OpenLibs(eng, "*")

			name := strings.ToLower(viper.GetString("name"))
			repl := lua.NewREPLWithConfig(lua.REPLConfig{
				Engine:          eng,
				Name:            name,
				Prompt:          ansi.Colorize(dragon.color + "\\{name}{x} {L}(\\{n}) {W}> {x}"),
				HistoryFilePath: ".repl-history",
			})

			fmt.Printf("\n  type '.exit' to quit.\n\n")
			err := repl.Run()
			if err != nil {
				log.WithError(err).Error("Encountered error running Console.")
			}

			fmt.Println()
			log.Debug("The dragon bids you farewell")
		},
	}
)

func init() {
	replCmd.Flags().StringVarP(&level, "level", "l", "server", "Specify the security level of requested engine, server/client/entity")

	RootCmd.AddCommand(replCmd)
}
