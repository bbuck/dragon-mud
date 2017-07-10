package modules_test

import (
	"github.com/bbuck/dragon-mud/scripting"
	"github.com/bbuck/dragon-mud/scripting/lua"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var script = `
    local tmpl = require("tmpl")

    tmpl.register("lua_test", "Hello, [[name]]!")

    function test_template()
        result = tmpl.render("lua_test", {name = "World"})

		return result
    end

	tmpl.register("simple_layout", "layout: [[content]]")

	function test_simple_layout()
		result = tmpl.render_in_layout("simple_layout", "lua_test", {name = "World"})

		return result
	end

	tmpl.register("complex_layout", "first: [[first]], second: [[second]]")
	tmpl.register("first", "[[fvalue]]")
	tmpl.register("second", "[[svalue]]")

	function test_complex_layout()
		result = tmpl.render_in_layout("complex_layout", {first = "first", second = "second"}, {fvalue = "one", svalue = "two"})

		return result
	end
`

var _ = Describe("tmpl Module", func() {
	Describe("Simple Usage", func() {
		var (
			e      *lua.Engine
			result string
			values []*lua.Value
			err    error
		)

		e = lua.NewEngine()
		scripting.OpenLibs(e, "tmpl")
		e.DoString(script)

		BeforeEach(func() {
			values, err = e.Call("test_template", 1)
			if err == nil {
				result = values[0].AsString()
			}
		})

		It("successfully calls the script method", func() {
			Ω(err).Should(BeNil())
		})

		It("doesn't fail", func() {
			Ω(err).Should(BeNil())
		})

		It("should render correctly", func() {
			Ω(result).Should(Equal("Hello, World!"))
		})
	})

	Describe("render_in_layout", func() {
		Context("With a simple layout", func() {
			var (
				e      *lua.Engine
				result string
				values []*lua.Value
				err    error
			)

			e = lua.NewEngine()
			scripting.OpenLibs(e, "tmpl")
			e.DoString(script)

			BeforeEach(func() {
				values, err = e.Call("test_simple_layout", 1)
				if err == nil {
					result = values[0].AsString()
				}
			})

			It("successfully calls the script method", func() {
				Ω(err).Should(BeNil())
			})

			It("doesn't fail", func() {
				Ω(err).Should(BeNil())
			})

			It("renders correctly", func() {
				Ω(result).Should(Equal("layout: Hello, World!"))
			})
		})

		Context("With a complex layout", func() {
			var (
				e      *lua.Engine
				result string
				values []*lua.Value
				err    error
			)

			e = lua.NewEngine()
			scripting.OpenLibs(e, "tmpl")
			e.DoString(script)

			BeforeEach(func() {
				values, err = e.Call("test_complex_layout", 1)
				if err == nil {
					result = values[0].AsString()
				}
			})

			It("successfully calls the script method", func() {
				Ω(err).Should(BeNil())
			})

			It("doesn't fail", func() {
				Ω(err).Should(BeNil())
			})

			It("Renders correctly", func() {
				Ω(result).Should(Equal("first: one, second: two"))
			})
		})
	})
})
