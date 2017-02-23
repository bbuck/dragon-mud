package modules_test

import (
	"github.com/bbuck/dragon-mud/scripting"
	"github.com/bbuck/dragon-mud/scripting/lua"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Random", func() {
	e := lua.NewEngine()
	scripting.OpenLibs(e, "random")
	e.DoString(`
        local random = require("random")

        function gen(max)
            return random.gen(max)
        end

        function range(min, max)
            return random.range(min, max)
        end
    `)

	Describe("gen()", func() {
		res, err := e.Call("gen", 1, 100)
		var result int64 = -1
		if len(res) > 0 {
			result = int64(res[0].AsNumber())
		}

		It("doesn't fail", func() {
			Ω(err).Should(BeNil())
		})

		It("is in valid range", func() {
			Ω(result).Should(BeNumerically(">=", 0))
			Ω(result).Should(BeNumerically("<", 100))
		})
	})

	Describe("range()", func() {
		res, err := e.Call("range", 1, 50, 90)
		var result int64
		if len(res) > 0 {
			result = int64(res[0].AsNumber())
		}

		It("doesn't fail", func() {
			Ω(err).Should(BeNil())
		})

		It("is in valid range", func() {
			Ω(result).Should(BeNumerically(">=", 50))
			Ω(result).Should(BeNumerically("<", 90))
		})
	})
})
