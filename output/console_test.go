package output_test

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/bbuck/dragon-mud/ansi"
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

	Describe("Println", func() {
		It("accepts an fmt.Stringer", func() {
			stringer := testStringer("testing")
			console.Println(stringer)
			Ω(buffer.String()).To(Equal("testing\n"))
		})

		Context("arbitrary values", func() {
			It("accpets integers", func() {
				console.Println(10)
				Ω(buffer.String()).To(Equal(fmt.Sprintf("%d\n", 10)))
			})

			It("accepts floats", func() {
				console.Println(float32(1))
				Ω(buffer.String()).To(Equal(fmt.Sprintf("%f\n", float32(1))))
			})

			It("accepts arbitrary values", func() {
				console.Println(complex64(1))
				Ω(buffer.String()).To(Equal(fmt.Sprintf("%v\n", complex64(1))))
			})
		})
	})

	Context("colored text", func() {
		var (
			coloredStr     = "{r}this is {g}colored{x} text"
			result         = ansi.Colorize(coloredStr)
			xtermStr       = "{c001}this is red{x}"
			xtermResult    = ansi.Colorize(xtermStr)
			fallbackResult = ansi.ColorizeWithFallback(xtermStr, true)
		)

		Describe("Println", func() {
			Context("ColorMono support", func() {
				BeforeEach(func() {
					console.ColorSupport = ColorMono
					console.Println(coloredStr)
				})

				It("purges color codes", func() {
					Ω(buffer.String()).To(Equal("this is colored text\n"))
				})
			})

			Context("ColorBASIC support", func() {
				BeforeEach(func() {
					console.ColorSupport = ColorBasic
				})

				It("processes colors and then prints", func() {
					console.Println(coloredStr)
					Ω(buffer.String()).To(Equal(result + "\n"))
				})

				It("uses fallback on xterm colors", func() {
					console.Println(xtermStr)
					Ω(buffer.String()).To(Equal(fallbackResult + "\n"))
				})
			})

			Context("Color256 support", func() {
				BeforeEach(func() {
					console.ColorSupport = Color256
					console.Println(xtermStr)
				})

				It("processes colors and then prints", func() {
					Ω(buffer.String()).To(Equal(xtermResult + "\n"))
				})
			})
		})

		Describe("Printf", func() {
			Context("ColorMono support", func() {
				BeforeEach(func() {
					console.ColorSupport = ColorMono
					console.Printf("%s", "{r}testing{x}")
				})

				It("prints the text given to it", func() {
					Ω(buffer.String()).To(Equal("testing"))
				})
			})

			Context("ColorBasic support", func() {
				BeforeEach(func() {
					console.ColorSupport = ColorBasic
				})

				It("prints the text given to it", func() {
					console.Printf("%s", coloredStr)
					Ω(buffer.String()).To(Equal(result))
				})

				It("uses fallback on xterm colors", func() {
					console.Printf("%s", xtermStr)
					Ω(buffer.String()).To(Equal(fallbackResult))
				})
			})

			Context("Color256 support", func() {
				BeforeEach(func() {
					console.ColorSupport = Color256
					console.Printf("%s", xtermStr)
				})

				It("processes colors and then prints", func() {
					Ω(buffer.String()).To(Equal(xtermResult))
				})
			})
		})

		Describe("as io.Writer", func() {
			Context("ColorMono support", func() {
				BeforeEach(func() {
					console.ColorSupport = ColorMono
					fmt.Fprintf(console, "%s", "{r}testing{x}")
				})

				It("writes the text to the buffer without color", func() {
					Ω(buffer.String()).To(Equal("testing"))
				})
			})

			Context("ColorBasic support", func() {
				BeforeEach(func() {
					console.ColorSupport = ColorBasic
				})

				It("writes the text given to it", func() {
					console.Printf("%s", coloredStr)
					Ω(buffer.String()).To(Equal(result))
				})

				It("uses fallback on xterm colors", func() {
					console.Printf("%s", xtermStr)
					Ω(buffer.String()).To(Equal(fallbackResult))
				})
			})

			Context("Color256 support", func() {
				BeforeEach(func() {
					console.ColorSupport = Color256
					console.Printf("%s", xtermStr)
				})

				It("writes the text to the buffer in color", func() {
					Ω(buffer.String()).To(Equal(xtermResult))
				})
			})
		})
	})

	Context("with plain output funcs", func() {
		Describe("PlainPrintln", func() {
			BeforeEach(func() {
				console.PlainPrintln(str)
			})

			It("prints the text passed to it with a newline", func() {
				Ω(buffer.String()).To(Equal(str + "\n"))
			})
		})

		Describe("PlainPrintf", func() {
			BeforeEach(func() {
				console.PlainPrintf("%s", str)
			})

			It("prints the text passed to it, based on format", func() {
				Ω(buffer.String()).To(Equal(str))
			})
		})
	})

	Describe("native consoles", func() {
		Describe("Stdout()", func() {
			var (
				fileCache *os.File
				colored   = "{R}testing{x}"
				result    = ansi.Colorize(colored)
				rdr, wtr  *os.File
				err       error
			)

			BeforeEach(func() {
				fileCache = os.Stdout
				rdr, wtr, err = os.Pipe()
				if err != nil {
					Fail(err.Error())
				}
				os.Stdout = wtr
				Stdout().Println(colored)
				wtr.Close()
				io.Copy(buffer, rdr)
			})

			It("prints to the expected place with color", func() {
				Ω(buffer.String()).To(Equal(result + "\n"))
			})

			AfterEach(func() {
				os.Stdout = fileCache
				rdr.Close()
			})
		})

		Describe("Stderr()", func() {
			var (
				fileCache *os.File
				colored   = "{R}testing{x}"
				result    = ansi.Colorize(colored)
				rdr, wtr  *os.File
				err       error
			)

			BeforeEach(func() {
				fileCache = os.Stderr
				rdr, wtr, err = os.Pipe()
				if err != nil {
					Fail(err.Error())
				}
				os.Stderr = wtr
				Stderr().Println(colored)
				wtr.Close()
				io.Copy(buffer, rdr)
			})

			It("prints to the expected place with color", func() {
				Ω(buffer.String()).To(Equal(result + "\n"))
			})

			AfterEach(func() {
				os.Stderr = fileCache
				rdr.Close()
			})
		})
	})
})
