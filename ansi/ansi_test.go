package ansi

import (
	"strings"
	"testing"
)

const plain = "this is a string"

func TestColorizeWithCode(t *testing.T) {
	tests := []struct {
		name, code, want string
	}{
		{"ansi code", "R", "\033[31;1m" + plain},
		{"invalid code", "invalid", plain},
		{"xterm code", "c001", "\033[38;5;1m" + plain},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ColorizeWithCode(tt.code, plain); got != tt.want {
				t.Errorf("ColorizeWithCode(%q) = %q, want %q", tt.code, got, tt.want)
			}
		})
	}
}

func TestColorizeWithFallbackCode(t *testing.T) {
	want := "\033[31;22m" + plain
	if got := ColorizeWithFallbackCode("c001", plain, true); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestColorize(t *testing.T) {
	colored := "[r]this is [g]a colored[x] string"
	coloredResult := "\033[31;22mthis is \033[32;22ma colored\033[0m string"

	tests := []struct {
		name, in, want string
	}{
		{"all codes", colored, coloredResult},
		{"plain string", plain, plain},
		{"already colored", coloredResult, coloredResult},
		{"escaped code", "sample code [[r]]", "sample code [r]"},
		{"background", "[-r]The background should be red![x]", "\033[41;22mThe background should be red!\033[0m"},
		{"xterm", "[c001]This is Xterm colored[x]", "\033[38;5;1mThis is Xterm colored\033[0m"},
		{"flip", "[r][-b]This is [~]Flipped[x]", "\033[31;22m\033[44;22mThis is \033[7mFlipped\033[0m"},
		{"extra leading bracket", "[[r]red[x]", "[\033[31;22mred\033[0m"},
		{"extra trailing bracket", "[r]]red[x]", "\033[31;22m]red\033[0m"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Colorize(tt.in); got != tt.want {
				t.Errorf("Colorize(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestColorizeWithFallback(t *testing.T) {
	in := "[c001]This is Xterm colored[x]"
	want := "\033[31;22mThis is Xterm colored\033[0m"
	if got := ColorizeWithFallback(in, true); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestPurge(t *testing.T) {
	tests := []struct {
		name, in, want string
	}{
		{"removes codes", "[r]this is [g]a colored[x] string", "this is a colored string"},
		{"keeps escaped codes", "sample code [[r]]", "sample code [r]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Purge(tt.in); got != tt.want {
				t.Errorf("Purge(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestEscape(t *testing.T) {
	want := strings.ReplaceAll("\033[31;22mred\033[0m", "\033", "\\033")
	if got := Escape(Colorize("[r]red[x]")); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
