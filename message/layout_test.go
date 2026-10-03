package message

import (
	"strings"
	"testing"
)

var layoutData = map[string]any{
	"short":  []any{"warrior", "mage", "rogue", "cleric", "bard"},
	"long":   []any{"a really long item name", "another long one", "a third long item"},
	"mixed":  []any{Entity{"id": "b0b", "name": "Bob"}, "The Dragon's Rest"},
	"rows":   []any{[]any{"Bob", "warrior", 12.0}, []any{"Alexandra", "mage", 3.5}},
	"header": []any{"Name", "Class", "Level"},
	"wide": []any{
		[]any{"1.", "A long description that will need to wrap."},
		[]any{"2.", "Short."},
	},
	"title": "The Dragon's Rest",
	"text":  "A fire crackles in a hearth carved like a sleeping dragon.",
}

// renderLayout renders source as a template of format, laid out 30 wide.
func renderLayout(t *testing.T, format, source string) string {
	t.Helper()

	ts := NewTemplates()
	ts.SetWidth(30)
	if err := ts.Add(File{Name: "x", Format: format, Path: "x." + format + ".tmpl", Source: source}); err != nil {
		t.Fatal(err)
	}

	return render(t, ts, "x", FormatHTML, "", layoutData)
}

func TestLayoutText(t *testing.T) {
	tests := []struct{ name, source, want string }{
		{"columns go down then across", `{{columns 3 .short}}`, "warrior  rogue   bard\nmage     cleric"},
		{"columns use fewer when items don't fit", `{{columns 3 .long}}`, "a really long item name\nanother long one\na third long item"},
		{"columns show entities by name", `{{columns 2 .mixed}}`, "Bob  The Dragon's Rest"},
		{"table", `{{table .rows}}`, "Bob        warrior  12\nAlexandra  mage     3.5"},
		{"table with a header", `{{table .header .rows}}`, "Name       Class    Level\n---------  -------  -----\nBob        warrior  12\nAlexandra  mage     3.5"},
		{"table wraps its last column", `{{table .wide}}`, "1.  A long description that\n    will need to wrap.\n2.  Short."},
		{"center", `{{center .title}}`, "      The Dragon's Rest"},
		{"rule", `{{rule}}|{{rule "="}}`, strings.Repeat("-", 30) + "|" + strings.Repeat("=", 30)},
		{"indent", `{{indent 4 .text}}`, "    A fire crackles in a\n    hearth carved like a\n    sleeping dragon."},
		{"pad", `[{{pad 6 "ab"}}][{{padleft 6 "ab"}}][{{pad 1 "abc"}}]`, "[ab    ][    ab][abc]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := NewTemplates()
			ts.SetWidth(30)
			if err := ts.Add(File{Name: "x", Format: FormatText, Path: "x.txt.tmpl", Source: tt.source}); err != nil {
				t.Fatal(err)
			}
			if got := render(t, ts, "x", FormatText, "", layoutData); got != tt.want {
				t.Errorf("got\n%s\nwant\n%s", got, tt.want)
			}
		})
	}
}

func TestLayoutHTML(t *testing.T) {
	tests := []struct{ name, source, want string }{
		{
			"columns", "Classes:\n{{columns 3 .mixed}}\nPick one.",
			`Classes:<div class="dragon-columns" style="--columns:3"><div><dragon-entity ref="b0b">Bob</dragon-entity></div><div>The Dragon&#39;s Rest</div></div>Pick one.`,
		},
		{
			"table with a header", `{{table .header .rows}}`,
			`<table class="dragon-table"><tr><th>Name</th><th>Class</th><th>Level</th></tr><tr><td>Bob</td><td>warrior</td><td>12</td></tr><tr><td>Alexandra</td><td>mage</td><td>3.5</td></tr></table>`,
		},
		{"rule", "above\n{{rule}}\nbelow", `above<hr class="dragon-rule">below`},
		{"center", `{{center .title}}`, `<div class="dragon-center">The Dragon&#39;s Rest</div>`},
		{"indent", `{{indent 4 "hi"}}`, `<div class="dragon-indent" style="--indent:4ch">hi</div>`},
		{"pad", `{{pad 6 "ab"}}{{padleft 6 "<b>"}}`, `<span class="dragon-pad" style="--pad:6ch">ab</span><span class="dragon-pad dragon-padleft" style="--pad:6ch">&lt;b&gt;</span>`},
	}

	for _, tt := range tests {
		t.Run("text "+tt.name, func(t *testing.T) {
			if got := renderLayout(t, FormatText, tt.source); got != tt.want {
				t.Errorf("got\n%s\nwant\n%s", got, tt.want)
			}
		})
		t.Run("html "+tt.name, func(t *testing.T) {
			// HTML templates don't turn line breaks into <br>, so leave
			// them out.
			source := strings.ReplaceAll(tt.source, "\n", "")
			if got := renderLayout(t, FormatHTML, source); got != tt.want {
				t.Errorf("got\n%s\nwant\n%s", got, tt.want)
			}
		})
	}
}

func TestLayoutErrors(t *testing.T) {
	tests := []struct{ source, want string }{
		{`{{columns 0 .short}}`, "columns needs at least 1 column, not 0"},
		{`{{columns 2 .title}}`, "columns needs a list, not The Dragon's Rest"},
		{`{{table .title}}`, "table needs a list of rows, each a list"},
		{`{{table .short}}`, "table's row 1 needs a list, not warrior"},
		{`{{rule "=="}}`, `rule takes one character to draw the line with, like {{rule "="}}, not "=="`},
	}

	for _, tt := range tests {
		ts := NewTemplates()
		if err := ts.Add(File{Name: "x", Format: FormatText, Path: "x.txt.tmpl", Source: tt.source}); err != nil {
			t.Fatal(err)
		}
		_, _, err := ts.Render("x", FormatText, "", layoutData)
		if err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%s: error = %v, want it to contain %q", tt.source, err, tt.want)
		}
	}
}
