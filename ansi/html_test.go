package ansi

import "testing"

func TestHTML(t *testing.T) {
	tests := []struct {
		name, in, want string
	}{
		{"plain", "hello", "hello"},
		{"escapes html", "<b>&</b>", "&lt;b&gt;&amp;&lt;/b&gt;"},
		{"basic color", "[r]red[x] plain", `<span class="ansi-fg-1">red</span> plain`},
		{"bright color", "[R]red", `<span class="ansi-fg-1 ansi-bold">red</span>`},
		{"color change", "[r]a[g]b[x]", `<span class="ansi-fg-1">a</span><span class="ansi-fg-2">b</span>`},
		{"background", "[-b]x[x]", `<span class="ansi-bg-4">x</span>`},
		{"xterm", "[c196]x[x]", `<span style="color:#ff0000">x</span>`},
		{"underline", "[u]x[x]", `<span class="ansi-under">x</span>`},
		{"escaped code", "[[r]]", "[r]"},
		{"unclosed", "[g]open", `<span class="ansi-fg-2">open</span>`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HTML(tt.in); got != tt.want {
				t.Errorf("HTML(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestXtermHex(t *testing.T) {
	tests := map[int]string{
		1:   "#800000",
		16:  "#000000",
		196: "#ff0000",
		231: "#ffffff",
		232: "#080808",
		255: "#eeeeee",
	}

	for n, want := range tests {
		if got := xtermHex(n); got != want {
			t.Errorf("xtermHex(%d) = %s, want %s", n, got, want)
		}
	}
}
