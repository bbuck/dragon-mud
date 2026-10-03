package ansi

import "testing"

func TestWidth(t *testing.T) {
	tests := []struct {
		text string
		want int
	}{
		{"hello", 5},
		{"[R]red[x] and [c123]more[x]", 12},
		{"[[r]] is a code", 13},
		{"[nope]", 6},
		{"龍 dragon", 9},
		{"🐉", 2},
		{"é", 1},
		{"short\nmuch longer", 11},
	}

	for _, tt := range tests {
		if got := Width(tt.text); got != tt.want {
			t.Errorf("Width(%q) = %d, want %d", tt.text, got, tt.want)
		}
	}
}

func TestWrap(t *testing.T) {
	tests := []struct {
		name, text string
		width      int
		want       string
	}{
		{"fits", "a short line", 20, "a short line"},
		{"breaks at spaces", "the quick brown fox jumps", 10, "the quick\nbrown fox\njumps"},
		{"colors take no room", "[R]the[x] quick [G]brown[x] fox", 15, "[R]the[x] quick [G]brown[x]\nfox"},
		{"long words split", "abcdefghijkl", 5, "abcde\nfghij\nkl"},
		{"keeps indentation", "  1. a fairly long choice here", 14, "  1. a fairly\n  long choice\n  here"},
		{"each line separately", "one two three\nfour", 8, "one two\nthree\nfour"},
		{"wide characters", "龍龍 龍龍", 5, "龍龍\n龍龍"},
		{"no width", "the quick brown fox", 0, "the quick brown fox"},
		{"extra spaces at a break", "one   two", 4, "one\ntwo"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Wrap(tt.text, tt.width); got != tt.want {
				t.Errorf("Wrap(%q, %d) = %q, want %q", tt.text, tt.width, got, tt.want)
			}
		})
	}
}
