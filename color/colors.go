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

var colorRx = regexp.MustCompile(`(?m)(.|^)\{(.+?)\}`)

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
	fallbackColors = make(map[string]string)
)

func init() {
	for k, val := range colorMap {
		colorToANSI[k] = fmt.Sprintf("\033[3%sm", val)
		colorToANSI["-"+k] = fmt.Sprintf("\033[4%sm", val)
	}
	for i := 0; i < 256; i++ {
		colorToANSI[fmt.Sprintf("c%03d", i)] = fmt.Sprintf("\033[38;5;%dm", i)
		colorToANSI[fmt.Sprintf("-c%03d", i)] = fmt.Sprintf("\033[48;5;%dm", i)
	}

	assignFallbacks("l", fallbackRange{0, 0})
	assignFallbacks("r", fallbackRange{1, 1}, fallbackRange{52, 52},
		fallbackRange{88, 88}, fallbackRange{124, 125})
	assignFallbacks("g", fallbackRange{2, 2}, fallbackRange{22, 22},
		fallbackRange{28, 29}, fallbackRange{34, 35}, fallbackRange{40, 42},
		fallbackRange{71, 71})
	assignFallbacks("y", fallbackRange{3, 3}, fallbackRange{58, 58},
		fallbackRange{64, 65}, fallbackRange{94, 95}, fallbackRange{100, 101},
		fallbackRange{106, 107}, fallbackRange{130, 131}, fallbackRange{136, 137},
		fallbackRange{142, 143}, fallbackRange{148, 150}, fallbackRange{166, 166},
		fallbackRange{172, 173}, fallbackRange{178, 179}, fallbackRange{186, 187},
		fallbackRange{220, 222})
	assignFallbacks("b", fallbackRange{4, 4}, fallbackRange{17, 19},
		fallbackRange{60, 60})
	assignFallbacks("m", fallbackRange{5, 5}, fallbackRange{53, 56},
		fallbackRange{89, 93}, fallbackRange{96, 97}, fallbackRange{126, 129},
		fallbackRange{132, 135}, fallbackRange{139, 141}, fallbackRange{162, 163},
		fallbackRange{167, 170}, fallbackRange{174, 177})
	assignFallbacks("c", fallbackRange{6, 6}, fallbackRange{23, 24},
		fallbackRange{30, 31}, fallbackRange{36, 39}, fallbackRange{43, 45},
		fallbackRange{66, 67}, fallbackRange{72, 75}, fallbackRange{115, 116})
	assignFallbacks("w", fallbackRange{7, 7}, fallbackRange{59, 59},
		fallbackRange{108, 109}, fallbackRange{138, 138}, fallbackRange{144, 147},
		fallbackRange{151, 152}, fallbackRange{180, 183}, fallbackRange{188, 189},
		fallbackRange{192, 195}, fallbackRange{218, 219}, fallbackRange{223, 225},
		fallbackRange{241, 250})
	assignFallbacks("L", fallbackRange{8, 8}, fallbackRange{16, 16},
		fallbackRange{232, 240})
	assignFallbacks("R", fallbackRange{9, 9}, fallbackRange{160, 161},
		fallbackRange{196, 199}, fallbackRange{202, 203}, fallbackRange{208, 211})
	assignFallbacks("G", fallbackRange{10, 10}, fallbackRange{46, 48},
		fallbackRange{70, 70}, fallbackRange{76, 78}, fallbackRange{82, 84},
		fallbackRange{112, 114}, fallbackRange{118, 121}, fallbackRange{154, 157},
		fallbackRange{190, 191})
	assignFallbacks("Y", fallbackRange{11, 11}, fallbackRange{184, 185},
		fallbackRange{214, 217}, fallbackRange{226, 229})
	assignFallbacks("B", fallbackRange{12, 12}, fallbackRange{20, 21},
		fallbackRange{25, 27}, fallbackRange{32, 33}, fallbackRange{57, 57},
		fallbackRange{61, 63}, fallbackRange{68, 69}, fallbackRange{98, 99},
		fallbackRange{103, 105}, fallbackRange{110, 111})
	assignFallbacks("M", fallbackRange{13, 13}, fallbackRange{164, 165},
		fallbackRange{171, 171}, fallbackRange{200, 201}, fallbackRange{204, 207},
		fallbackRange{212, 213})
	assignFallbacks("C", fallbackRange{14, 14}, fallbackRange{49, 51},
		fallbackRange{79, 81}, fallbackRange{85, 87}, fallbackRange{117, 117},
		fallbackRange{122, 123}, fallbackRange{153, 153}, fallbackRange{158, 159})
	assignFallbacks("W", fallbackRange{15, 15}, fallbackRange{102, 102},
		fallbackRange{230, 231}, fallbackRange{251, 255})
}

// ColorizeWithCode takes a color code and text string and returns the string
// with the appropriate color codes in it.
func ColorizeWithCode(key, text string) string {
	return ColorizeWithFallbackCode(key, text, false)
}

// ColorizeWithFallbackCode takes a color code and text string and returns the string
// with the appropriate color codes in it.
func ColorizeWithFallbackCode(key, text string, fallback bool) string {
	if fallback {
		key = FallbackColor(key)
	}

	if code, ok := colorToANSI[key]; ok {
		return fmt.Sprintf("%s%s", code, text)
	}

	return text
}

// Colorize processes all colors in a given text block and returns a new string
// with all readable codes translated to ANSI codes.
func Colorize(text string) string {
	return ColorizeWithFallback(text, false)
}

// ColorizeWithFallback will replace xterm color choices with their fallback
// colors if false is passed in place of fallback
func ColorizeWithFallback(text string, fallback bool) string {
	final := colorRx.ReplaceAllStringFunc(text, func(s string) string {
		match := colorRx.FindStringSubmatch(s)
		if len(match[1]) > 0 && match[1] == colorCodeEscape {
			return fmt.Sprintf("{%s}", match[2])
		}

		codes := strings.Split(match[2], ",")
		var (
			colors string
			valid  = true
		)
		for _, code := range codes {
			if fallback {
				code = FallbackColor(code)
			}

			if color, ok := colorToANSI[code]; ok {
				colors += color
			} else {
				valid = false
				break
			}
		}

		if valid {
			return fmt.Sprintf("%s%s", match[1], colors)
		}

		return s
	})

	return final
}

// FallbackColor takes a code for a given xterm value and then returns the best
// ANSI match for it.
func FallbackColor(code string) string {
	if fallback, ok := fallbackColors[code]; ok {
		return fallback
	}

	return code
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

type fallbackRange struct {
	start, end int
}

func assignFallbacks(code string, ranges ...fallbackRange) {
	for _, rng := range ranges {
		for i := rng.start; i < rng.end+1; i++ {
			fallbackColors[fmt.Sprintf("c%03d", i)] = code
		}
	}
}
