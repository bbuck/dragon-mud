package ansi_test

import (
	"strings"

	. "github.com/bbuck/dragon-mud/ansi"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Color", func() {
	var str = "this is a string"

	Describe("ColorizeWithCode", func() {
		var (
			code        = "R"
			result      = "\033[31;1m" + str
			xterm       = "c001"
			xtermResult = "\033[38;5;1m" + str
		)

		It("performs the same action as ANSI codes", func() {
			Ω(ColorizeWithCode(code, str)).To(Equal(result))
		})

		It("returns the string if an invalid code is given", func() {
			Ω(ColorizeWithCode("invalid", "testing")).To(Equal("testing"))
		})

		It("colorizes xterm codes as well", func() {
			Ω(ColorizeWithCode(xterm, str)).To(Equal(xtermResult))
		})
	})

	Describe("ColorizeWithFallbackCode", func() {
		var (
			code   = "c001"
			result = "\033[31m" + str
		)

		It("colorizes with fallback values", func() {
			Ω(ColorizeWithFallbackCode(code, str, true)).To(Equal(result))
		})
	})

	Describe("Colorize", func() {
		var (
			colored             = "{r}this is {g}a colored{x} string"
			result              = "\033[31mthis is \033[32ma colored\033[0m string"
			escaped             = "sample code \\{r}"
			escapedResult       = "sample code {r}"
			withBackground      = "{-r}The background should be red!{x}"
			bgResult            = "\033[41mThe background should be red!\033[0m"
			withXterm           = "{c001}This is Xterm colored{x}"
			xtermResult         = "\033[38;5;1mThis is Xterm colored\033[0m"
			xtermFallbackResult = "\033[31mThis is Xterm colored\033[0m"
		)

		It("processes all color codes in a string", func() {
			Ω(Colorize(colored)).To(Equal(result))
		})

		It("does nothing for plain strings", func() {
			Ω(Colorize(str)).To(Equal(str))
		})

		It("does nothing for pre-colored strings", func() {
			Ω(Colorize(Colorize(colored))).To(Equal(result))
		})

		It("does not replace escaped color codes", func() {
			Ω(Colorize(escaped)).To(Equal(escapedResult))
		})

		It("colorizes background when putting a '-' before the code", func() {
			Ω(Colorize(withBackground)).To(Equal(bgResult))
		})

		It("colorizes xterm 256 color codes", func() {
			Ω(Colorize(withXterm)).To(Equal(xtermResult))
		})

		It("falls back to ASCII from xterm when told to", func() {
			Ω(ColorizeWithFallback(withXterm, true)).To(Equal(xtermFallbackResult))
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
			Ω(Purge(colored)).To(Equal(result))
		})

		It("does not purge escaped color codes", func() {
			Ω(Purge(escaped)).To(Equal(escapedResult))
		})
	})

	Describe("Escape", func() {
		var (
			str    = "{r}red{x}"
			result = strings.Replace("\033[31mred\033[0m", "\033", "\\033", -1)
		)

		It("escapes the ANSI escape sequence", func() {
			Ω(Escape(Colorize(str))).To(Equal(result))
		})
	})
})
