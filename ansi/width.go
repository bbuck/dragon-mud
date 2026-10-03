package ansi

import (
	"slices"
	"strings"
	"unicode"
)

// atom is one piece of text with color codes: a code, which takes no room,
// or one rune.
type atom struct {
	text  string
	width int
	space bool
}

// atoms splits text into codes and runes. Escaped codes like [[r]] are the
// text they show, [r].
func atoms(text string) []atom {
	var out []atom
	runes := func(s string) {
		for _, r := range s {
			out = append(out, atom{text: string(r), width: RuneWidth(r), space: r == ' ' || r == '\t'})
		}
	}

	last := 0
	for _, m := range colorRx.FindAllStringSubmatchIndex(text, -1) {
		runes(text[last:m[0]])
		code := text[m[2]:m[3]]
		switch {
		case strings.HasPrefix(code, "[") && strings.HasSuffix(code, "]"):
			runes(code)
		case isCode(code):
			out = append(out, atom{text: text[m[0]:m[1]]})
		default:
			runes(text[m[0]:m[1]])
		}
		last = m[1]
	}
	runes(text[last:])

	return out
}

// isCode reports whether code, from inside brackets, is a color code,
// allowing the single stray bracket colorRx can include.
func isCode(code string) bool {
	code = strings.TrimSuffix(strings.TrimPrefix(code, "["), "]")
	_, ok := colorToANSI[code]

	return ok
}

// Width is how many columns text takes in a terminal: color codes take
// none, and wide characters such as CJK and emoji take two. For text with
// several lines, it's the widest line.
func Width(text string) int {
	widest, w := 0, 0
	for _, a := range atoms(text) {
		if a.text == "\n" {
			widest, w = max(widest, w), 0
			continue
		}
		w += a.width
	}

	return max(widest, w)
}

// RuneWidth is how many columns r takes in a terminal.
func RuneWidth(r rune) int {
	switch {
	case r == 0 || unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Me, r) || unicode.Is(unicode.Cf, r):
		return 0
	case r < 0x1100:
		return 1
	}

	for _, wide := range wideRanges {
		if r >= wide[0] && r <= wide[1] {
			return 2
		}
	}

	return 1
}

// wideRanges are the main East Asian wide and fullwidth blocks, and emoji.
var wideRanges = [][2]rune{
	{0x1100, 0x115F},   // Hangul Jamo
	{0x231A, 0x231B},   // watch, hourglass
	{0x2329, 0x232A},   // angle brackets
	{0x23E9, 0x23EC},   // media controls
	{0x2614, 0x2615},   // umbrella, hot beverage
	{0x2E80, 0x303E},   // CJK radicals, punctuation
	{0x3041, 0x33FF},   // kana, CJK symbols
	{0x3400, 0x4DBF},   // CJK extension A
	{0x4E00, 0x9FFF},   // CJK unified ideographs
	{0xA000, 0xA4CF},   // Yi
	{0xAC00, 0xD7A3},   // Hangul syllables
	{0xF900, 0xFAFF},   // CJK compatibility ideographs
	{0xFE30, 0xFE4F},   // CJK compatibility forms
	{0xFF00, 0xFF60},   // fullwidth forms
	{0xFFE0, 0xFFE6},   // fullwidth signs
	{0x1F300, 0x1F64F}, // pictographs, emoticons
	{0x1F680, 0x1F6FF}, // transport and map
	{0x1F900, 0x1F9FF}, // supplemental pictographs
	{0x20000, 0x3FFFD}, // CJK extensions B and on
}

// Wrap breaks each line of text that's wider than width at spaces, so no
// line is wider than width. Color codes take no room and carry over to
// the next line, as terminals keep colors across lines. A word wider than
// width is split. Continuation lines keep the line's indentation, so lists
// stay lined up. Width 0 or less leaves text as it is.
func Wrap(text string, width int) string {
	if width <= 0 {
		return text
	}

	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if Width(line) > width {
			lines[i] = wrapLine(line, width)
		}
	}

	return strings.Join(lines, "\n")
}

func wrapLine(line string, width int) string {
	all := atoms(line)

	// Continuation lines are indented like the first, unless that would
	// leave too little room.
	var indent []atom
	indentWidth := 0
	for _, a := range all {
		if !a.space {
			break
		}
		indent = append(indent, a)
		indentWidth += a.width
	}
	if indentWidth > width/2 {
		indent, indentWidth = nil, 0
	}

	var out []string
	var cur []atom
	w, lastSpace := 0, -1

	emit := func(atoms []atom) {
		var b strings.Builder
		end := len(atoms)
		for end > 0 && atoms[end-1].space {
			end--
		}
		for _, a := range atoms[:end] {
			b.WriteString(a.text)
		}
		// Codes after trailing spaces still apply.
		for _, a := range atoms[end:] {
			if a.width == 0 && !a.space {
				b.WriteString(a.text)
			}
		}
		out = append(out, b.String())
	}
	newLine := func(rest []atom) {
		cur = slices.Concat(indent, rest)
		w, lastSpace = 0, -1
		for i, a := range cur {
			w += a.width
			if a.space && i >= len(indent) {
				lastSpace = i
			}
		}
	}

	for _, a := range all {
		switch {
		case a.width == 0:
			cur = append(cur, a)
			continue
		case a.space && w == indentWidth && len(out) > 0:
			continue // no leading spaces on a continuation line
		}

		if w+a.width > width {
			switch {
			case a.space:
				emit(cur)
				newLine(nil)
				continue
			case lastSpace >= 0:
				rest := cur[lastSpace+1:]
				emit(cur[:lastSpace])
				newLine(rest)
			default:
				emit(cur)
				newLine(nil)
			}
		}

		cur = append(cur, a)
		w += a.width
		if a.space {
			lastSpace = len(cur) - 1
		}
	}
	emit(cur)

	return strings.Join(out, "\n")
}
