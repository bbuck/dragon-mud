package color_test

import (
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
			colored = "{red}this is {green}a colored{reset} string"
			result  = ColorizeWithCode("red", "this is ") + ColorizeWithCode("green", "a colored") + ColorizeWithCode("reset", " string")
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
	})
})
