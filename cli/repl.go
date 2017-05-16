package cli

import (
	"bufio"
	"fmt"
	"os"

	"github.com/chzyer/readline"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	glua "github.com/yuin/gopher-lua"
	"github.com/yuin/gopher-lua/parse"

	"strings"

	"github.com/bbuck/dragon-mud/logger"
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
			log := logger.NewWithSource("repl")
			log.Info("Starting read-eval-print-loop")

			// TODO: Add security level specic engine creation here
			eng := lua.NewEngine(lua.EngineOptions{
				FieldNaming:  lua.SnakeCaseNames,
				MethodNaming: lua.SnakeCaseNames,
			})

			repl := &REPL{
				promptFmt: fmt.Sprintf("%s (%%d)> ", strings.ToLower(viper.GetString("name"))),
				engine:    eng,
				log:       log,
			}

			err := repl.Run()
			if err != nil {
				log.WithError(err).Error("Encountered error running Console.")
			}
		},
	}
)

func init() {
	replCmd.Flags().StringVarP(&level, "level", "l", "server", "Specify the security level of requested engine, server/client/entity")

	RootCmd.AddCommand(replCmd)
}

// REPL represent a Read-Eval-Print-Loop
type REPL struct {
	lineNumber uint
	promptFmt  string
	engine     *lua.Engine
	log        logger.Log
	input      *readline.Instance
}

// Run begins the execution fo the read-eval-print-loop. Executing the REPL
// only ends when an input line matches `.exit` or if an error is encountered.
func (r *REPL) Run() error {
	var err error
	r.input, err = readline.New(r.Prompt())
	if err != nil {
		return err
	}

	for {
		line, err := r.input.Readline()
		if err != nil {
			if err.Error() == "Interrupt" {
				fmt.Print("Please use '.exit' to exit console.\n\n")

				continue
			}

			return err
		}

		if line == ".exit" {
			os.Exit(0)
		}

		before := r.engine.StackSize()
		err = r.engine.DoString(line)
		if err != nil {
			fmt.Printf("\n <=> %s\n", err.Error())

		} else {
			var results []*lua.Value
			after := r.engine.StackSize() - before
			for i := 0; i < after; i++ {
				val := r.engine.PopValue()
				results = append([]*lua.Value{val}, results...)
			}

			if len(results) > 0 {
				var strs []string
				for i := 0; i < len(results); i++ {
					strs = append(strs, results[i].AsString())
				}

				fmt.Printf(" => %s\n", strings.Join(strs, ", "))
			} else {
				fmt.Println(" => nil")
			}
		}

		r.lineNumber++
		r.input.SetPrompt(r.Prompt())
	}
}

// Prompt returns a formatted prompt to use as the Readline prompt.
func (r *REPL) Prompt() string {
	return fmt.Sprintf(r.promptFmt, r.lineNumber)
}

func runConsole(log logger.Log, engine *lua.Engine) {
	in := bufio.NewReader(os.Stdin)
	for {
		str, err := readLine(in, engine)
		if err != nil {
			log.WithError(err).Fatal("Failed to read line of script.")

			return
		}

		if err := engine.DoString(str); err != nil {
			fmt.Printf(" ==> %s\n", err)
		}
	}
}

func isIncompleteLineError(err error) bool {
	if lerr, ok := err.(*glua.ApiError); ok {
		if perr, ok := lerr.Cause.(*parse.Error); ok {
			return perr.Pos.Line == parse.EOF
		}
	}

	return false
}

func readLine(reader *bufio.Reader, engine *lua.Engine) (string, error) {
	fmt.Print("> ")
	line, err := reader.ReadString('\n')
	if err == nil {
		// try add return <...> then compile
		if _, err := engine.LoadString("return " + line); err == nil {
			return line, nil
		}

		return multiline(line, reader, engine)
	}

	return "", err
}

func multiline(ml string, reader *bufio.Reader, engine *lua.Engine) (string, error) {
	for {
		if _, err := engine.LoadString(ml); err == nil {
			return ml, nil
		} else if !isIncompleteLineError(err) {
			return ml, nil
		} else {
			fmt.Print(">> ")
			if line, err := reader.ReadString('\n'); err == nil {
				ml = ml + "\n" + line
			} else {
				return "", err
			}
		}
	}
}
