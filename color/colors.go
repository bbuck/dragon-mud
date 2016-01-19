package color

import (
	"bytes"
	"strings"

	"github.com/mgutz/ansi"
)

const (
	// ColorCodeOpen is the rune that denotes the beginning of the color code.
	ColorCodeOpen = '{'

	// ColorCodeClose is the rune that denotes the end of the color code.
	ColorCodeClose = '}'

	// ColorCodeEscape is the rune that denotes an escaped ColorCodeOpen
	ColorCodeEscape = '\\'
)

// ColorizeFunc is a function that takes a string and returns a string with
// ANSI color escape codes in it.
type ColorizeFunc func(string) string

var (
	noopColorFunc = ColorizeFunc(func(text string) string {
		return text
	})
	colorFuncMap   = make(map[string]ColorizeFunc)
	commonPatterns = []string{
		"black", "black+h",
		"red", "red+h",
		"green", "green+h",
		"yellow", "yellow+h",
		"blue", "blue+h",
		"magenta", "magenta+h",
		"cyan", "cyan+h",
		"white", "white+h",
		"reset",
	}
)

func init() {
	// we preload common color patterns to avoid processing these color codes
	// during runtime
	for _, pattern := range commonPatterns {
		getColorFunction(pattern)
	}
}

func getColorFunction(code string) ColorizeFunc {
	var (
		colorFunc ColorizeFunc
		ok        bool
	)

	if colorFunc, ok = colorFuncMap[code]; ok {
		return colorFunc
	}

	colorFunc = ColorizeFunc(ansi.ColorFunc(code))
	colorFuncMap[code] = colorFunc

	return colorFunc
}

// ColorizeWithCode takes a color code and text string and returns the string
// with the appropriate color codes in it.
func ColorizeWithCode(code, text string) string {
	return getColorFunction(code)(text)
}

// Colorize processes all colors in a given text block and returns a new string
// with all readable codes translated to ANSI codes.
func Colorize(text string) string {
	final := new(bytes.Buffer)
	toColor := new(bytes.Buffer)
	prevColorFunc := noopColorFunc
	for len(text) > 0 {
		startIndex := strings.Index(text, string(ColorCodeOpen))
		switch {
		// if it's escaped, skip it.
		case startIndex > 0 && rune(text[startIndex-1]) == ColorCodeEscape:
			toColor.WriteString(text[:startIndex+1])
			text = text[startIndex+1:]
			continue
		case startIndex < 0:
			toColor.WriteString(text)
			final.WriteString(prevColorFunc(toColor.String()))
			text = ""
		default:
			toColor.WriteString(text[:startIndex])
			final.WriteString(prevColorFunc(toColor.String()))
			toColor = new(bytes.Buffer)
			text = text[startIndex+1:]
			endIndex := strings.Index(text, string(ColorCodeClose))
			prevColorFunc = getColorFunction(text[:endIndex])
			text = text[endIndex+1:]
		}
	}

	return final.String()
}

// Purge will remove color codes from the given string.
func Purge(text string) string {
	return text
}

// Escape will replace all ANSI escape codes with text equivalents so strings
// can be printed with color codes.
func Escape(text string) string {
	return strings.Replace(text, "\033", "\\033", -1)
}
