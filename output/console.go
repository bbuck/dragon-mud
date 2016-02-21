package output

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/bbuck/dragon-mud/color"
)

// Console is an output source, used for printing text to. This can be stdout
// for the server of represent the endpoint of a client.
type Console struct {
	writer io.Writer
	colors bool
}

var (
	// do dead simple, dumb detection of 256 color support
	term256       = strings.Contains(os.Getenv("TERM"), "256")
	stdoutConsole = &Console{
		writer: os.Stdout,
		colors: term256,
	}
	stderrConsole = &Console{
		writer: os.Stderr,
		colors: term256,
	}
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
	return &Console{
		writer: w,
		colors: false,
	}
}

// Println prints the text followed by a trailing newline processing color codes.
func (c *Console) Println(text interface{}) {
	var str string
	switch value := text.(type) {
	case string:
		str = value
	case fmt.Stringer:
		str = value.String()
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		str = fmt.Sprintf("%d", text)
	case float32, float64:
		str = fmt.Sprintf("%f", text)
	default:
		str = fmt.Sprintf("%v", text)
	}

	fmt.Fprintf(c.writer, "%s\n", color.Colorize(str))
}

// Write makes Console conform to io.Writer and can therefore be used as a
// logger target.
func (c *Console) Write(p []byte) (n int, err error) {
	str := color.Colorize(string(p))
	_, err = c.writer.Write([]byte(str))

	return len(p), err
}

// Printf will print the string with a format to the console processing
// color codes.
func (c *Console) Printf(format string, params ...interface{}) {
	text := fmt.Sprintf(format, params...)
	fmt.Fprintf(c.writer, "%s", color.Colorize(text))
}

// PlainPrintln will print the string ignoring color codes in the text.
func (c *Console) PlainPrintln(text interface{}) {
	fmt.Fprintf(c.writer, "%s\n", text)
}

// PlainPrintf will print the string ignoring color codes.
func (c *Console) PlainPrintf(format string, params ...interface{}) {
	fmt.Fprintf(c.writer, format, params...)
}
