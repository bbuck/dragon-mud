package modules_test

import (
	"github.com/bbuck/dragon-mud/scripting"
	"github.com/bbuck/dragon-mud/scripting/lua"
	"github.com/spf13/viper"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Config", func() {
	eng := lua.NewEngine()
	scripting.OpenLibs(eng, "config")
	eng.DoString(`
		local config = require("config")

		function fetch(key)
			return config.get(key)
		end
	`)

	var (
		val *lua.Value
		err error
	)

	BeforeEach(func() {
		viper.SetDefault("config.testing", 10)
		vals, err := eng.Call("fetch", 1, "config.testing")
		if err == nil {
			val = vals[0]
		}
	})

	It("doesn't fail", func() {
		Ω(err).Should(BeNil())
	})

	It("returns correct value", func() {
		Ω(val.AsNumber()).Should(Equal(float64(10)))
	})
})
