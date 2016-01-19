package output

import (
	"fmt"
	"io"
	"os"

	"github.com/bbuck/dragon-mud/color"
)

// Console is an output source, used for printing text to. This can be stdout
// for the server of represent the endpoint of a client.
type Console struct {
	writer io.Writer
}

var (
	stdoutConsole = &Console{os.Stdout}
	stderrConsole = &Console{os.Stderr}
)

// Stdout returns a Console that will print to the servers terminal.
func Stdout() *Console {
	return stdoutConsole
}

// Stderr returns a Console that will print to the servers error outputf
func Stderr() *Console {
	return stderrConsole
}

// NewConsole creates a new console wrapping the given io.Writer
func NewConsole(w io.Writer) *Console {
	return &Console{w}
}

// Println prints the text followed by a trailing newline processing color codes.
func (c *Console) Println(text fmt.Stringer) {
	fmt.Fprintf(c.writer, "%s\n", color.Colorize(text.String()))
}

// Printf will print the string with a format to the console processing
// color codes.
func (c *Console) Printf(format string, params ...interface{}) {
	text := fmt.Sprintf(format, params...)
	fmt.Fprintf(c.writer, "%s", color.Colorize(text))
}

// PlainPrintln will print the string ignoring color codes in the text.
func (c *Console) PlainPrintln(text fmt.Stringer) {
	fmt.Fprintf(c.writer, "%s\n", text.String())
}

// PlainPrintf will print the string ignoring color codes.
func (c *Console) PlainPrintf(format string, params ...interface{}) {
	fmt.Fprintf(c.writer, format, params...)
}
