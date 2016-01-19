package output_test

import (
	"bytes"

	"github.com/bbuck/dragon-mud/color"
	. "github.com/bbuck/dragon-mud/output"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Console", func() {
	var (
		console *Console
		buffer  *bytes.Buffer
		str     = "this is text"
	)

	BeforeEach(func() {
		buffer = new(bytes.Buffer)
		console = NewConsole(buffer)
	})

	Describe("Println", func() {
		var (
			coloredStr = "{red}this is {green}colored{reset} text"
			result     = color.Colorize(coloredStr)
		)

		BeforeEach(func() {
			console.Println(coloredStr)
		})

		It("processes colors and then prints", func() {
			Ω(buffer.String()).Should(Equal(result + "\n"))
		})
	})

	Describe("PlainPrintln", func() {
		BeforeEach(func() {
			console.PlainPrintln(str)
		})

		It("prints the text passed to it with a newline", func() {
			Ω(buffer.String()).Should(Equal(str + "\n"))
		})
	})

	Describe("PlainPrintf", func() {
		BeforeEach(func() {
			console.PlainPrintf("%s", str)
		})

		It("prints the text passed to it, based on format", func() {
			Ω(buffer.String()).Should(Equal(str))
		})
	})
})
