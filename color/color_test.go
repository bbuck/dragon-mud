package color_test

import (
	"strings"

	. "github.com/bbuck/dragon-mud/color"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Color", func() {
	var str = "this is a string"
	Describe("ColorizeWithCode", func() {
		var (
			code = "R"
		)

		It("performs the same action as ANSI codes", func() {
			Ω(ColorizeWithCode(code, str)).Should(Equal("\033[31;1mthis is a string"))
		})
	})

	Describe("Colorize", func() {
		var (
			colored       = "{r}this is {g}a colored{x} string"
			result        = ColorizeWithCode("r", "this is ") + ColorizeWithCode("g", "a colored") + ColorizeWithCode("x", " string")
			escaped       = "sample code \\{r}"
			escapedResult = "sample code {r}"
		)

		It("processes all color codes in a string", func() {
			Ω(Colorize(colored)).Should(Equal(result))
		})

		It("does nothing for plain strings", func() {
			Ω(Colorize(str)).Should(Equal(str))
		})

		It("does nothing for pre-colored strings", func() {
			Ω(Colorize(Colorize(colored))).Should(Equal(result))
		})

		It("does not replace escaped color codes", func() {
			Ω(Colorize(escaped)).Should(Equal(escapedResult))
		})
	})

	Describe("Purge", func() {
		var (
			colored       = "{r}this is {g}a colored{x} string"
			result        = "this is a colored string"
			escaped       = "sample code \\{r}"
			escapedResult = "sample code {r}"
		)

		It("removes all color codes from a string", func() {
			Ω(Purge(colored)).Should(Equal(result))
		})

		It("does not purge escaped color codes", func() {
			Ω(Purge(escaped)).Should(Equal(escapedResult))
		})
	})

	Describe("Escape", func() {
		var (
			str    = "{r}red{x}"
			result = strings.Replace("\033[31mred\033[0m", "\033", "\\033", -1)
		)

		It("escapes the ANSI escape sequence", func() {
			Ω(Escape(Colorize(str))).Should(Equal(result))
		})
	})
})
