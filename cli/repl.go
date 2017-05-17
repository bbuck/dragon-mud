package cli

import (
	"bytes"
	"fmt"
	"os"

	"github.com/chzyer/readline"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	glua "github.com/yuin/gopher-lua"
	"github.com/yuin/gopher-lua/parse"

	"strings"

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
			log := logger.NewWithSource("repl")
			log.Info("Starting read-eval-print-loop")

			// TODO: Add security level specic engine creation here
			eng := lua.NewEngine(lua.EngineOptions{
				FieldNaming:  lua.SnakeCaseNames,
				MethodNaming: lua.SnakeCaseNames,
			})
			scripting.OpenLibs(eng, "talon")

			name := strings.ToLower(viper.GetString("name"))
			repl := &REPL{
				promptNumFmt: fmt.Sprintf("%s (%%d)> ", name),
				promptStrFmt: fmt.Sprintf("%s (%%s)> ", name),
				engine:       eng,
				log:          log,
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
	lineNumber   uint
	promptNumFmt string
	promptStrFmt string
	engine       *lua.Engine
	log          logger.Log
	input        *readline.Instance
}

// Run begins the execution fo the read-eval-print-loop. Executing the REPL
// only ends when an input line matches `.exit` or if an error is encountered.
func (r *REPL) Run() error {
	var err error
	r.input, err = readline.NewEx(&readline.Config{
		Prompt:      r.NumberPrompt(),
		HistoryFile: ".repl-history",
	})
	if err != nil {
		return err
	}

	for {
		line, err := r.read()
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

		r.Execute(line)

		r.lineNumber++
		r.input.SetPrompt(r.NumberPrompt())
	}
}

// Execute will take a source string and attempt to execute it in the given
// engine context.
func (r *REPL) Execute(src string) {
	retSrc := fmt.Sprintf("return (%s)", src)

	before := r.engine.StackSize()

	// try to run code that forces a return value
	err := r.engine.DoString(retSrc)
	if err != nil {
		// if the customized return injection caused failure, we double check
		// by executing the code without it.
		err = r.engine.DoString(src)
	}

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
				strs = append(strs, results[i].Inspect())
			}

			fmt.Printf(" => %s\n", strings.Join(strs, ", "))
		} else {
			fmt.Println(" => nil")
		}
	}
}

// NumberPrompt returns a formatted prompt to use as the Readline prompt.
func (r *REPL) NumberPrompt() string {
	return fmt.Sprintf(r.promptNumFmt, r.lineNumber)
}

// StarPrompt generates a similar prompt to the font with the line number in
// it, but instead of the line number it uses a * character.
func (r *REPL) StarPrompt() string {
	n := r.lineNumber
	count := 0
	for ; n > 0; n /= 10 {
		count++
	}
	if count == 0 {
		count = 1
	}

	return fmt.Sprintf(r.promptStrFmt, strings.Repeat("*", count))
}

// determines if the error means that more code can follow (i.e. multi-line
// input.
func (r *REPL) isIncompleteLine(err error) bool {
	if lerr, ok := err.(*glua.ApiError); ok {
		if perr, ok := lerr.Cause.(*parse.Error); ok {
			return perr.Pos.Line == parse.EOF
		}
	}

	return false
}

func (r *REPL) read() (string, error) {
	line, err := r.input.Readline()
	if err != nil {
		return "", err
	}

	_, err = r.engine.LoadString("return " + line)
	if err == nil {
		return line, nil
	}

	return r.readMulti(line)
}

// read multiline input
func (r *REPL) readMulti(line string) (string, error) {
	buf := new(bytes.Buffer)
	buf.WriteString(line)

	for {
		_, err := r.engine.LoadString(buf.String())
		if err == nil || !r.isIncompleteLine(err) {
			return buf.String(), nil
		}

		r.input.SetPrompt(r.StarPrompt())
		line, err = r.input.Readline()
		if err != nil {
			return "", err
		}
		buf.WriteRune('\n')
		buf.WriteString(line)
	}
}
