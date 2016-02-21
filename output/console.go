package output

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/bbuck/dragon-mud/ansi"
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
	term                         = len(os.Getenv("TERM")) > 0
	term256                      = strings.Contains(os.Getenv("TERM"), "256")
	stdoutConsole, stderrConsole *Console
)

func getColorSupport() ColorSupport {
	switch {
	case term && term256:
		return Color256
	case term:
		return ColorBasic
	default:
		return ColorMono
	}
}

// Stdout returns a Console that will print to the servers terminal.
func Stdout() *Console {
	if stdoutConsole == nil {
		stdoutConsole = &Console{
			writer:       os.Stdout,
			ColorSupport: getColorSupport(),
		}
	}

	return stdoutConsole
}

// Stderr returns a Console that will print to the servers error outputf
func Stderr() *Console {
	if stderrConsole == nil {
		stderrConsole = &Console{
			writer:       os.Stderr,
			ColorSupport: getColorSupport(),
		}
	}

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

	fmt.Fprintf(c.writer, "%s\n", c.colorize(str))
}

// Write makes Console conform to io.Writer and can therefore be used as a
// logger target.
func (c *Console) Write(p []byte) (n int, err error) {
	text := string(p)
	_, err = c.writer.Write([]byte(c.colorize(text)))

	return len(p), err
}

// Printf will print the string with a format to the console processing
// color codes.
func (c *Console) Printf(format string, params ...interface{}) {
	text := fmt.Sprintf(format, params...)
	fmt.Fprintf(c.writer, "%s", c.colorize(text))
}

// PlainPrintln will print the string ignoring color codes in the text.
func (c *Console) PlainPrintln(text interface{}) {
	fmt.Fprintf(c.writer, "%s\n", text)
}

// PlainPrintf will print the string ignoring color codes.
func (c *Console) PlainPrintf(format string, params ...interface{}) {
	fmt.Fprintf(c.writer, format, params...)
}

func (c *Console) colorize(str string) string {
	switch c.ColorSupport {
	case Color256:
		return ansi.Colorize(str)
	case ColorBasic:
		return ansi.ColorizeWithFallback(str, true)
	default:
		return ansi.Purge(str)
	}
}
