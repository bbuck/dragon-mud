package modules_test

import (
	"github.com/bbuck/dragon-mud/scripting"
	"github.com/bbuck/dragon-mud/scripting/engine"

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
		e      *engine.Lua
		result string
		values []*engine.LuaValue
		ok     bool
		err    error
	)

	e = engine.NewLua()
	scripting.OpenTmpl(e)
	e.DoString(script)

	BeforeEach(func() {
		values, err = e.Call("testTemplate", 2)
		if err == nil {
			result = values[0].AsString()
			ok = values[1].AsBool()
		}
	})

	It("successfully calls the script method", func() {
		Ω(err).To(BeNil())
	})

	It("doesn't fail", func() {
		Ω(err).To(BeNil())
		Ω(ok).To(BeTrue())
	})

	It("should render correctly", func() {
		Ω(err).To(BeNil())
		Ω(result).To(Equal("Hello, World!"))
	})
})
