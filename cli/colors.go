// Copyright (c) 2016-2017 Brandon Buck

package cli

import (
	"fmt"

	"github.com/bbuck/dragon-mud/ansi"
	"github.com/bbuck/dragon-mud/output"
	"github.com/spf13/cobra"
)

var (
	colorCmd = &cobra.Command{
		Use:   "colors",
		Short: "Display colors in a fancy way for viewing.",
		Long: `Provide display of the color code results in the console to provide a visual
	way to decide which color codes to use in the game or with scripts where you
	plan to log data to the console (make your logs rich, if you wish!).`,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintf(output.Stdout(), colorDisplayOutput)
		},
	}

	colorDisplayOutput = `
{C}--------------------------<[ {r}Dragon{R}MUD  {R}C {Y}O {G}L {B}O {M}R {R}S {C}]>--------------------------{x}

Please use this guide to visualize what each color code maps to which colors.
Remember, white and black (or any other color for that matter) may not show up
depending on the color settings of your terminal. Color codes are wrapped in
braces to provide them a simple yet easily distinguishable format. Bracketed
text that doesn't match a color code will not be replaced, but if you want to
display a color code then double the brackets, {{{ and }}}, like {{{r}}}

 {{L}} {L}black{x} {{R}} {R}red{x} {{G}} {G}green{x} {{Y}} {Y}yellow{x} {{B}} {B}blue{x} {{M}} {M}magenta{x} {{C}} {C}cyan{x} {{W}} {W}white{x}
 {{l}} {l}black{x} {{r}} {r}red{x} {{g}} {g}green{x} {{y}} {y}yellow{x} {{b}} {b}blue{x} {{m}} {m}magenta{x} {{c}} {c}cyan{x} {{w}} {w}white{x}

If you would prefer to highlight the background, then simply add a '-' before
the code (inside of the braces). For example: {{l}}{{-Y}} {l}{-Y}Hello, World!{x}. Don't
forget to use the reset code {{x}} after your done colorizing your text!

It's best practice to end colored lines or sections with {{x}} which is a special
code used to reset the foreground and background colors. If you don't do this
colors will bleed across lines.

Another special code is {{u}} which {u}underlines{x} text. It's important to remember to
include {{x}} after you underline text otherwise all text following will be
underlined.

`
)

func init() {
	RootCmd.AddCommand(colorCmd)

	for i := 0; i < 256; i++ {
		code := fmt.Sprintf("c%03d", i)
		fallback := ansi.FallbackColor(code)
		colorDisplayOutput += fmt.Sprintf("   {{%s}} {%s}This is xterm color %03d{x}     {{%s}} {%s}The ANSI fallback color{x}\n", code, code, i, fallback, fallback)
	}
}
