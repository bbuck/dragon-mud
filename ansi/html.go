package ansi

import (
	"fmt"
	"html"
	"regexp"
	"strconv"
	"strings"
)

// sgrRx matches the SGR escape sequences Colorize produces.
var sgrRx = regexp.MustCompile("\033\\[([0-9;]*)m")

// htmlState is the styling in effect at a point in the text.
type htmlState struct {
	fg, bg              string // "1".."7" for basic colors, "x123" for xterm
	bold, under, invert bool
}

func (s htmlState) empty() bool {
	return s == htmlState{}
}

// open returns the opening span for the state.
func (s htmlState) open() string {
	var classes []string
	var styles []string

	addColor := func(prefix, color string, css string) {
		if color == "" {
			return
		}
		if strings.HasPrefix(color, "x") {
			n, _ := strconv.Atoi(color[1:])
			styles = append(styles, fmt.Sprintf("%s:%s", css, xtermHex(n)))
			return
		}
		classes = append(classes, prefix+color)
	}

	addColor("ansi-fg-", s.fg, "color")
	addColor("ansi-bg-", s.bg, "background-color")
	if s.bold {
		classes = append(classes, "ansi-bold")
	}
	if s.under {
		classes = append(classes, "ansi-under")
	}
	if s.invert {
		classes = append(classes, "ansi-invert")
	}

	var b strings.Builder
	b.WriteString("<span")
	if len(classes) > 0 {
		fmt.Fprintf(&b, ` class="%s"`, strings.Join(classes, " "))
	}
	if len(styles) > 0 {
		fmt.Fprintf(&b, ` style="%s"`, strings.Join(styles, ";"))
	}
	b.WriteString(">")

	return b.String()
}

// apply updates the state from one SGR parameter list.
func (s *htmlState) apply(params string) {
	parts := strings.Split(params, ";")
	for i := 0; i < len(parts); i++ {
		p := parts[i]
		switch {
		case p == "0" || p == "":
			*s = htmlState{}
		case p == "1":
			s.bold = true
		case p == "22":
			s.bold = false
		case p == "4":
			s.under = true
		case p == "7":
			s.invert = true
		case (p == "38" || p == "48") && i+2 < len(parts) && parts[i+1] == "5":
			color := "x" + parts[i+2]
			if p == "38" {
				s.fg = color
			} else {
				s.bg = color
			}
			i += 2
		case len(p) == 2 && p[0] == '3' && p[1] >= '0' && p[1] <= '7':
			s.fg = p[1:]
		case len(p) == 2 && p[0] == '4' && p[1] >= '0' && p[1] <= '7':
			s.bg = p[1:]
		}
	}
}

// HTML converts text containing color codes such as [r] and [c123] into
// HTML-escaped text with spans. Basic colors become classes (ansi-fg-1,
// ansi-bg-4, ansi-bold, ansi-under, ansi-invert) so a stylesheet can theme
// them; xterm 256 colors become inline styles.
func HTML(text string) string {
	colored := Colorize(text)

	var b strings.Builder
	var state htmlState
	spanOpen := false
	last := 0

	for _, loc := range sgrRx.FindAllStringSubmatchIndex(colored, -1) {
		b.WriteString(html.EscapeString(colored[last:loc[0]]))
		last = loc[1]

		state.apply(colored[loc[2]:loc[3]])

		if spanOpen {
			b.WriteString("</span>")
			spanOpen = false
		}
		if !state.empty() {
			b.WriteString(state.open())
			spanOpen = true
		}
	}

	b.WriteString(html.EscapeString(colored[last:]))
	if spanOpen {
		b.WriteString("</span>")
	}

	return b.String()
}

// xtermHex returns the hex color for an xterm 256-color index.
func xtermHex(n int) string {
	basic := [16]string{
		"#000000", "#800000", "#008000", "#808000", "#000080", "#800080", "#008080", "#c0c0c0",
		"#808080", "#ff0000", "#00ff00", "#ffff00", "#0000ff", "#ff00ff", "#00ffff", "#ffffff",
	}

	switch {
	case n < 0 || n > 255:
		return "inherit"
	case n < 16:
		return basic[n]
	case n < 232:
		n -= 16
		level := func(v int) int {
			if v == 0 {
				return 0
			}
			return 55 + v*40
		}
		return fmt.Sprintf("#%02x%02x%02x", level(n/36), level(n/6%6), level(n%6))
	default:
		gray := 8 + (n-232)*10
		return fmt.Sprintf("#%02x%02x%02x", gray, gray, gray)
	}
}
