package color

import (
	"fmt"
	"regexp"
	"strings"
)

const (
	colorCodeEscape = "\\"
)

// ColorizeFunc is a function that takes a string and returns a string with
// ANSI color escape codes in it.
type ColorizeFunc func(string) string

var colorRx = regexp.MustCompile(`(.|^)\{(-?[lLrRgGyYbBmMcCwWx]|c\d{1,3})\}`)

var (
	colorMap = map[string]string{
		"l": "0",
		"L": "0;1",
		"r": "1",
		"R": "1;1",
		"g": "2",
		"G": "2;1",
		"y": "3",
		"Y": "3;1",
		"b": "4",
		"B": "4;1",
		"m": "5",
		"M": "5;1",
		"c": "6",
		"C": "6;1",
		"w": "7",
		"W": "7;1",
	}
	colorToANSI = map[string]string{
		"x": "\033[0m",
	}
)

func init() {
	for k, val := range colorMap {
		colorToANSI[k] = fmt.Sprintf("\033[3%sm", val)
		colorToANSI["-"+k] = fmt.Sprintf("\033[4%sm", val)
	}
	for i := 0; i < 256; i++ {
		colorToANSI[fmt.Sprintf("c%d", i)] = fmt.Sprintf("\033[38;5;%dm", i)
		colorToANSI[fmt.Sprintf("-c%d", i)] = fmt.Sprintf("\033[48;5;%dm", i)
	}
}

// ColorizeWithCode takes a color code and text string and returns the string
// with the appropriate color codes in it.
func ColorizeWithCode(key, text string) string {
	if code, ok := colorToANSI[key]; ok {
		return fmt.Sprintf("%s%s", code, text)
	}

	return text
}

// Colorize processes all colors in a given text block and returns a new string
// with all readable codes translated to ANSI codes.
func Colorize(text string) string {
	final := colorRx.ReplaceAllStringFunc(text, func(s string) string {
		match := colorRx.FindStringSubmatch(s)
		if len(match[1]) > 0 && match[1] == colorCodeEscape {
			return fmt.Sprintf("{%s}", match[2])
		}

		if code, ok := colorToANSI[match[2]]; ok {
			return fmt.Sprintf("%s%s", match[1], code)
		}

		return s
	})

	return final
}

// Purge will remove color codes from the given string.
func Purge(text string) string {
	final := colorRx.ReplaceAllStringFunc(text, func(s string) string {
		match := colorRx.FindStringSubmatch(s)
		if len(match[1]) > 0 && match[1] == colorCodeEscape {
			return fmt.Sprintf("{%s}", match[2])
		}

		return match[1]
	})

	return final
}

// Escape will replace all ANSI escape codes with text equivalents so strings
// can be printed with color codes.
func Escape(text string) string {
	return strings.Replace(text, "\033", "\\033", -1)
}
