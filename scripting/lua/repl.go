package lua

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	"github.com/chzyer/readline"
	glua "github.com/yuin/gopher-lua"
	"github.com/yuin/gopher-lua/parse"
)

var errExit = errors.New("Exit")

// REPL represent a Read-Eval-Print-Loop
type REPL struct {
	lineNumber   uint
	promptNumFmt string
	promptStrFmt string
	historyPath  string
	engine       *Engine
	input        *readline.Instance
}

const defaultPrompt = "{name} ({n})"

// REPLConfig provides a mean for configuring a lua.REPL value.
// -- HistoryFilePath is the path to the history file for storing the written
//    history of the REPL session (optional)
// -- Prompt is a fmt string to print as the prompt for each REPL input line.
//    There is a special format value {n} here where you want the line number
//    to go, or {name} as a place to inject the name provided (if any).
// -- Name is a name given, really only useful when no prompt is given as this
//    value is injected into the prompt.
type REPLConfig struct {
	Engine          *Engine
	HistoryFilePath string
	Name            string
	Prompt          string
}

// NewREPL creates a REPL struct and seeds it with the necessary values to
// prepare it for use. Uses the default .repl-history file.
func NewREPL(eng *Engine, name string) *REPL {
	return NewREPLWithConfig(REPLConfig{
		Prompt: defaultPrompt,
		Engine: eng,
		Name:   name,
	})
}

// NewREPLWithConfig creates a REPL from the provided configuration.
func NewREPLWithConfig(config REPLConfig) *REPL {
	prompt := defaultPrompt
	if len(config.Prompt) > 0 {
		prompt = config.Prompt
	}
	prompt = strings.Replace(prompt, "{name}", config.Name, -1)

	repl := &REPL{
		promptNumFmt: strings.Replace(prompt, "{n}", "%[1]d", -1),
		promptStrFmt: strings.Replace(prompt, "{n}", "%[1]s", -1),
		engine:       config.Engine,
	}

	if len(config.HistoryFilePath) == 0 {
		repl.historyPath = config.HistoryFilePath
	}

	return repl
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
			switch err.Error() {
			case "Interrupt":
				r.input.SetPrompt(r.NumberPrompt())

				continue
			case "Exit":
				return nil
			}

			return err
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
		var results []*Value
		after := r.engine.StackSize() - before
		for i := 0; i < after; i++ {
			val := r.engine.PopValue()
			results = append([]*Value{val}, results...)
		}

		if len(results) > 0 {
			for i := 0; i < len(results); i++ {
				str := results[i].Inspect("    ")
				fmt.Printf(" => %s\n", str)
			}
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
		if line == ".exit" {
			return "", errExit
		}

		buf.WriteRune('\n')
		buf.WriteString(line)
	}
}
