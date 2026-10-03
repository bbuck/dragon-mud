package message

import (
	"fmt"
	htmltemplate "html/template"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"bbuck.dev/dragon-mud/ansi"
)

// DefaultWidth is the width text is laid out to when none is set.
const DefaultWidth = 80

// columnGap is the space between columns in text.
const columnGap = 2

// Layout helpers lay text out for telnet and become real layout on the web:
//
//	{{columns 3 .list}}         items in up to 3 columns, top to bottom then
//	                            across, using fewer when they don't fit
//	{{table .rows}}             rows (lists) in columns sized to their content;
//	{{table .header .rows}}     the last column wraps to fit the width
//	{{center .text}}            centered in the width
//	{{rule}}, {{rule "="}}      a line across the width
//	{{indent 4 .text}}          wrapped to the width, indented 4 spaces
//	{{pad 10 .name}}            at least 10 columns wide, aligned left
//	{{padleft 6 .gold}}         at least 6 columns wide, aligned right
//
// Items can be entities, which show their names (clickable on the web), or
// anything else, which shows as text.
//
// In text the helpers pad with spaces. A text template rendered for the
// web marks what they made (see textToHTML), which becomes HTML a
// stylesheet lays out, so lines aren't padded in a proportional font. In
// HTML templates they make that HTML directly.
type layout interface {
	item(v any) (string, error)
	columns(n int, items []string) string
	table(header []string, rows [][]string) string
	center(text string) string
	rule(char string) string
	indent(n int, text string) string
	pad(n int, text string, left bool) string
}

// layoutFuncs returns the layout helpers for templates whose output l
// builds. wrap turns l's output into what the template engine expects.
func layoutFuncs[T any](l layout, wrap func(string) T) map[string]any {
	return map[string]any{
		"columns": func(n int, list any) (T, error) {
			var zero T
			if n < 1 {
				return zero, fmt.Errorf("columns needs at least 1 column, not %d", n)
			}
			items, err := itemList(l, "columns", list)
			if err != nil {
				return zero, err
			}
			return wrap(l.columns(n, items)), nil
		},
		"table": func(args ...any) (T, error) {
			var zero T
			var header []string
			switch len(args) {
			case 1:
			case 2:
				var err error
				if header, err = itemList(l, "table's header", args[0]); err != nil {
					return zero, err
				}
				args = args[1:]
			default:
				return zero, fmt.Errorf("table takes rows, or a header and rows, not %d arguments", len(args))
			}

			list, ok := asList(args[0])
			if !ok {
				return zero, fmt.Errorf("table needs a list of rows, each a list, not %v", args[0])
			}
			rows := make([][]string, len(list))
			for i, row := range list {
				cells, err := itemList(l, fmt.Sprintf("table's row %d", i+1), row)
				if err != nil {
					return zero, err
				}
				rows[i] = cells
			}
			return wrap(l.table(header, rows)), nil
		},
		"center": func(v any) (T, error) {
			s, err := l.item(v)
			return wrap(l.center(s)), err
		},
		"rule": func(char ...string) (T, error) {
			var zero T
			c := "-"
			if len(char) > 0 {
				c = char[0]
			}
			if ansi.Width(c) != 1 || len(char) > 1 {
				return zero, fmt.Errorf("rule takes one character to draw the line with, like {{rule \"=\"}}, not %q", strings.Join(char, " "))
			}
			return wrap(l.rule(c)), nil
		},
		"indent": func(n int, v any) (T, error) {
			var zero T
			if n < 0 {
				return zero, fmt.Errorf("indent can't be negative (%d)", n)
			}
			s, err := l.item(v)
			return wrap(l.indent(n, s)), err
		},
		"pad": func(n int, v any) (T, error) {
			s, err := l.item(v)
			return wrap(l.pad(n, s, false)), err
		},
		"padleft": func(n int, v any) (T, error) {
			s, err := l.item(v)
			return wrap(l.pad(n, s, true)), err
		},
	}
}

func asList(v any) ([]any, bool) {
	if list, ok := v.([]any); ok {
		return list, true
	}

	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Slice && rv.Kind() != reflect.Array {
		return nil, false
	}
	list := make([]any, rv.Len())
	for i := range list {
		list[i] = rv.Index(i).Interface()
	}

	return list, true
}

func itemList(l layout, what string, v any) ([]string, error) {
	if v == nil {
		return nil, nil
	}
	list, ok := asList(v)
	if !ok {
		if m, isMap := v.(map[string]any); isMap && len(m) == 0 {
			return nil, nil // an empty script table
		}
		return nil, fmt.Errorf("%s needs a list, not %v", what, v)
	}

	items := make([]string, len(list))
	for i, item := range list {
		s, err := l.item(item)
		if err != nil {
			return nil, err
		}
		items[i] = s
	}

	return items, nil
}

// display is how an item that isn't an entity shows.
func display(v any) string {
	switch v := v.(type) {
	case nil:
		return ""
	case string:
		return v
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	default:
		return fmt.Sprint(v)
	}
}

// textLayout lays text out with spaces, for terminals.
type textLayout struct {
	width func() int
}

func (l textLayout) item(v any) (string, error) {
	if e, ok := v.(Entity); ok {
		return e.Name(), nil
	}

	return display(v), nil
}

func padRight(s string, n int) string {
	return s + strings.Repeat(" ", max(0, n-ansi.Width(s)))
}

func (l textLayout) columns(n int, items []string) string {
	if len(items) == 0 {
		return ""
	}

	// The most columns, up to n, whose widest items fit side by side.
	var cols, rows int
	var widths []int
	for cols = min(n, len(items)); cols >= 1; cols-- {
		rows = (len(items) + cols - 1) / cols
		widths = make([]int, cols)
		for i, item := range items {
			widths[i/rows] = max(widths[i/rows], ansi.Width(item))
		}
		total := columnGap * (cols - 1)
		for _, w := range widths {
			total += w
		}
		if total <= l.width() || cols == 1 {
			break
		}
	}
	// Fewer items than slots can leave the last columns empty.
	cols = (len(items) + rows - 1) / rows

	lines := make([]string, rows)
	for r := range rows {
		var b strings.Builder
		for c := range cols {
			i := c*rows + r
			if i >= len(items) {
				break
			}
			if c > 0 {
				b.WriteString(strings.Repeat(" ", columnGap))
			}
			b.WriteString(padRight(items[i], widths[c]))
		}
		lines[r] = strings.TrimRight(b.String(), " ")
	}

	return strings.Join(lines, "\n")
}

func (l textLayout) table(header []string, rows [][]string) string {
	all := rows
	if header != nil {
		all = append([][]string{header}, rows...)
	}

	var widths []int
	for _, row := range all {
		for i, cell := range row {
			if i >= len(widths) {
				widths = append(widths, 0)
			}
			widths[i] = max(widths[i], ansi.Width(cell))
		}
	}
	if len(widths) == 0 {
		return ""
	}

	// The last column wraps in whatever room the others leave.
	before := 0
	for _, w := range widths[:len(widths)-1] {
		before += w + columnGap
	}
	last := l.width() - before
	if last < 10 {
		last = 0 // too little room; let the line run long instead
	}

	line := func(row []string) string {
		var b strings.Builder
		for i, cell := range row {
			if i == len(widths)-1 {
				cell = ansi.Wrap(cell, last)
				cell = strings.ReplaceAll(cell, "\n", "\n"+strings.Repeat(" ", before))
				b.WriteString(cell)
				break
			}
			b.WriteString(padRight(cell, widths[i]+columnGap))
		}
		return strings.TrimRight(b.String(), " ")
	}

	var lines []string
	if header != nil {
		lines = append(lines, line(header))
		dashes := make([]string, len(widths))
		for i, w := range widths {
			dashes[i] = strings.Repeat("-", w)
		}
		lines = append(lines, strings.Join(dashes, strings.Repeat(" ", columnGap)))
	}
	for _, row := range rows {
		lines = append(lines, line(row))
	}

	return strings.Join(lines, "\n")
}

func (l textLayout) center(text string) string {
	lines := strings.Split(ansi.Wrap(text, l.width()), "\n")
	for i, line := range lines {
		lines[i] = strings.Repeat(" ", max(0, (l.width()-ansi.Width(line))/2)) + line
	}

	return strings.Join(lines, "\n")
}

func (l textLayout) rule(char string) string {
	return strings.Repeat(char, l.width())
}

func (l textLayout) indent(n int, text string) string {
	prefix := strings.Repeat(" ", n)
	lines := strings.Split(ansi.Wrap(text, max(1, l.width()-n)), "\n")
	for i, line := range lines {
		if line != "" {
			lines[i] = prefix + line
		}
	}

	return strings.Join(lines, "\n")
}

func (l textLayout) pad(n int, text string, left bool) string {
	fill := strings.Repeat(" ", max(0, n-ansi.Width(text)))
	if left {
		return fill + text
	}

	return text + fill
}

// Text rendered for the web marks layout with private-use characters,
// which survive escaping, then textToHTML turns the marks into elements.
const (
	markOpen  = ""
	markClose = ""
)

func mark(tag string, arg ...int) string {
	if len(arg) > 0 {
		return markOpen + tag + " " + strconv.Itoa(arg[0]) + markClose
	}

	return markOpen + tag + markClose
}

var layoutMarkRx = regexp.MustCompile(markOpen + `(/?)([a-z]+)(?: ([0-9]+))?` + markClose)

// blockMarkRx matches block-level marks with the line breaks around them,
// which the elements replace.
var blockMarkRx = regexp.MustCompile(`\n?(` + markOpen + `(?:/?(?:columns|table|center|indent)|rule)(?: [0-9]+)?` + markClose + `)\n?`)

// layoutHTML is the HTML each layout mark becomes.
func layoutHTML(closing bool, tag, arg string) string {
	if closing {
		switch tag {
		case "pad", "padleft":
			return "</span>"
		case "table":
			return "</table>"
		case "tr":
			return "</tr>"
		case "td":
			return "</td>"
		case "th":
			return "</th>"
		default:
			return "</div>"
		}
	}

	switch tag {
	case "columns":
		return fmt.Sprintf(`<div class="dragon-columns" style="--columns:%s">`, arg)
	case "col":
		return "<div>"
	case "table":
		return `<table class="dragon-table">`
	case "tr", "td", "th":
		return "<" + tag + ">"
	case "center":
		return `<div class="dragon-center">`
	case "rule":
		return `<hr class="dragon-rule">`
	case "indent":
		return fmt.Sprintf(`<div class="dragon-indent" style="--indent:%sch">`, arg)
	case "pad":
		return fmt.Sprintf(`<span class="dragon-pad" style="--pad:%sch">`, arg)
	case "padleft":
		return fmt.Sprintf(`<span class="dragon-pad dragon-padleft" style="--pad:%sch">`, arg)
	}

	return ""
}

// markedLayout marks layout in text that will be rendered as HTML.
type markedLayout struct{}

func (markedLayout) item(v any) (string, error) {
	if e, ok := v.(Entity); ok {
		return markedEntity(e)
	}

	return display(v), nil
}

func wrapItems(tag string, items []string) string {
	var b strings.Builder
	for _, item := range items {
		b.WriteString(mark(tag) + item + mark("/"+tag))
	}

	return b.String()
}

func (markedLayout) columns(n int, items []string) string {
	return mark("columns", n) + wrapItems("col", items) + mark("/columns")
}

func (markedLayout) table(header []string, rows [][]string) string {
	var b strings.Builder
	b.WriteString(mark("table"))
	if header != nil {
		b.WriteString(mark("tr") + wrapItems("th", header) + mark("/tr"))
	}
	for _, row := range rows {
		b.WriteString(mark("tr") + wrapItems("td", row) + mark("/tr"))
	}
	b.WriteString(mark("/table"))

	return b.String()
}

func (markedLayout) center(text string) string {
	return mark("center") + text + mark("/center")
}

func (markedLayout) rule(string) string {
	return mark("rule")
}

func (markedLayout) indent(n int, text string) string {
	return mark("indent", n) + text + mark("/indent")
}

func (markedLayout) pad(n int, text string, left bool) string {
	tag := "pad"
	if left {
		tag = "padleft"
	}

	return mark(tag, n) + text + mark("/"+tag)
}

// htmlLayout makes HTML for HTML templates. Items are escaped.
type htmlLayout struct{}

func (htmlLayout) item(v any) (string, error) {
	if e, ok := v.(Entity); ok {
		h, err := htmlEntity(e)
		return string(h), err
	}

	return htmltemplate.HTMLEscapeString(display(v)), nil
}

// html builds what markedLayout marks, as HTML.
func (htmlLayout) html(marked string) string {
	return layoutMarkRx.ReplaceAllStringFunc(marked, func(m string) string {
		parts := layoutMarkRx.FindStringSubmatch(m)
		return layoutHTML(parts[1] == "/", parts[2], parts[3])
	})
}

func (l htmlLayout) columns(n int, items []string) string {
	return l.html(markedLayout{}.columns(n, items))
}

func (l htmlLayout) table(header []string, rows [][]string) string {
	return l.html(markedLayout{}.table(header, rows))
}

func (l htmlLayout) center(text string) string {
	return l.html(markedLayout{}.center(text))
}

func (l htmlLayout) rule(char string) string {
	return l.html(markedLayout{}.rule(char))
}

func (l htmlLayout) indent(n int, text string) string {
	return l.html(markedLayout{}.indent(n, text))
}

func (l htmlLayout) pad(n int, text string, left bool) string {
	return l.html(markedLayout{}.pad(n, text, left))
}
