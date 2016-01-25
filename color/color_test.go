package color_test

import (
	"strings"

	. "github.com/bbuck/dragon-mud/color"
	"github.com/mgutz/ansi"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Color", func() {
	var str = "this is a string"
	Describe("ColorizeWithCode", func() {
		var (
			code = "red+h"
		)

		It("performs the same action as ansi.Color", func() {
			Ω(ColorizeWithCode(code, str)).Should(Equal(ansi.Color(str, code)))
		})
	})

	Describe("Colorize", func() {
		var (
			colored       = "{red}this is {green}a colored{reset} string"
			result        = ColorizeWithCode("red", "this is ") + ColorizeWithCode("green", "a colored") + ColorizeWithCode("reset", " string")
			escaped       = "sample code \\{red}"
			escapedResult = "sample code {red}"
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
			colored       = "{red}this is {green}a colored{reset} string"
			result        = "this is a colored string"
			escaped       = "sample code \\{red}"
			escapedResult = "sample code {red}"
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
			str    = "{red}red{reset}"
			result = strings.Replace(ansi.Red+"red"+ansi.Reset, "\033", "\\033", -1)
		)

		It("escapes the ANSI escape sequence", func() {
			Ω(Escape(Colorize(str))).Should(Equal(result))
		})
	})
})
