package output

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/bbuck/dragon-mud/color"
)

// ColorSupport defines the level of color support, whether it be Mono, Basic
// or Xterm 256 colors.
type ColorSupport int8

const (
	// ColorMono specifies the console supports no color values. This will purge
	// color codes from text.
	ColorMono ColorSupport = iota

	// ColorBasic specifies support for the 8 standard (16 if you count bright)
	// ANSI colors.
	ColorBasic

	// Color256 specifies support for the extend Xterm 256 color codes.
	Color256
)

// Console is an output source, used for printing text to. This can be stdout
// for the server of represent the endpoint of a client.
type Console struct {
	writer io.Writer
	ColorSupport
}

var (
	// do dead simple, dumb detection of 256 color support
	// TODO: Make this way smarter
	term          = len(os.Getenv("TERM")) > 0
	term256       = strings.Contains(os.Getenv("TERM"), "256")
	stdoutConsole = &Console{
		writer: os.Stdout,
	}
	stderrConsole = &Console{
		writer: os.Stderr,
	}
)

func init() {
	var support ColorSupport
	switch {
	case term && term256:
		support = Color256
	case term:
		support = ColorBasic
	default:
		support = ColorMono
	}

	stdoutConsole.ColorSupport = support
	stderrConsole.ColorSupport = support
}

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
		writer:       w,
		ColorSupport: ColorMono,
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
	var colored string
	switch c.ColorSupport {
	case Color256:
		colored = color.Colorize(string(p))
	case ColorBasic:
		colored = color.ColorizeWithFallback(string(p), true)
	default:
		colored = color.Purge(string(p))
	}
	_, err = c.writer.Write([]byte(colored))

	return len(p), err
}

// Printf will print the string with a format to the console processing
// color codes.
func (c *Console) Printf(format string, params ...interface{}) {
	text := fmt.Sprintf(format, params...)
	var colored string
	switch c.ColorSupport {
	case Color256:
		colored = color.Colorize(text)
	case ColorBasic:
		colored = color.ColorizeWithFallback(text, true)
	default:
		colored = color.Purge(text)
	}
	fmt.Fprintf(c.writer, "%s", colored)
}

// PlainPrintln will print the string ignoring color codes in the text.
func (c *Console) PlainPrintln(text interface{}) {
	fmt.Fprintf(c.writer, "%s\n", text)
}

// PlainPrintf will print the string ignoring color codes.
func (c *Console) PlainPrintf(format string, params ...interface{}) {
	fmt.Fprintf(c.writer, format, params...)
}
