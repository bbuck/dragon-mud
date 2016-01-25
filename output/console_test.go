package output_test

import (
	"bytes"
	"fmt"

	"github.com/bbuck/dragon-mud/color"
	. "github.com/bbuck/dragon-mud/output"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type testStringer string

func (t testStringer) String() string {
	return string(t)
}

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

	Context("colored text", func() {
		var (
			coloredStr = "{red}this is {green}colored{reset} text"
			result     = color.Colorize(coloredStr)
		)

		Describe("Println", func() {
			It("processes colors and then prints", func() {
				console.Println(coloredStr)
				Ω(buffer.String()).Should(Equal(result + "\n"))
			})

			It("accepts an fmt.Stringer", func() {
				stringer := testStringer(coloredStr)
				console.Println(stringer)
				Ω(buffer.String()).Should(Equal(result + "\n"))
			})

			Context("arbitrary values", func() {
				It("accpets integers", func() {
					console.Println(10)
					Ω(buffer.String()).Should(Equal(fmt.Sprintf("%d\n", 10)))
				})

				It("accepts floats", func() {
					console.Println(float32(1))
					Ω(buffer.String()).Should(Equal(fmt.Sprintf("%f\n", float32(1))))
				})

				It("accepts arbitrary values", func() {
					console.Println(complex64(1))
					Ω(buffer.String()).Should(Equal(fmt.Sprintf("%v\n", complex64(1))))
				})
			})
		})

		Describe("Printf", func() {
			BeforeEach(func() {
				console.Printf("%s", coloredStr)
			})

			It("prints the text given to it", func() {
				Ω(buffer.String()).Should(Equal(result))
			})
		})

		Describe("as io.Writer", func() {
			BeforeEach(func() {
				fmt.Fprintf(console, "%s", coloredStr)
			})

			It("writes the text to the buffer in color", func() {
				Ω(buffer.String()).Should(Equal(result))
			})
		})
	})

	Context("without color", func() {
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
})
