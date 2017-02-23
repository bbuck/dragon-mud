package modules_test

import (
	"github.com/bbuck/dragon-mud/scripting"
	"github.com/bbuck/dragon-mud/scripting/lua"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var script = `
    local tmpl = require("tmpl")

    tmpl.register("Hello, {{name}}!", "lua_test")

    function testTemplate()
        result, ok = tmpl.render("lua_test", {name = "World"})

		return result, ok
    end
`

var _ = Describe("tmpl Module", func() {
	var (
		e      *lua.Engine
		result string
		values []*lua.Value
		ok     bool
		err    error
	)

	e = lua.NewEngine()
	scripting.OpenLibs(e, "tmpl")
	e.DoString(script)

	BeforeEach(func() {
		values, err = e.Call("testTemplate", 2)
		if err == nil {
			result = values[0].AsString()
			ok = values[1].AsBool()
		}
	})

	It("successfully calls the script method", func() {
		Ω(err).Should(BeNil())
	})

	It("doesn't fail", func() {
		Ω(err).Should(BeNil())
		Ω(ok).Should(BeTrue())
	})

	It("should render correctly", func() {
		Ω(err).Should(BeNil())
		Ω(result).Should(Equal("Hello, World!"))
	})
})
