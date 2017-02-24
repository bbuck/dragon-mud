package modules_test

import (
	"fmt"

	"github.com/bbuck/dragon-mud/scripting"
	"github.com/bbuck/dragon-mud/scripting/lua"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func testDie(e *lua.Engine, method string, min, max float64) {
	Describe(fmt.Sprintf("%s()", method), func() {
		res, err := e.Call("callSimple", 1, method)
		var result float64
		if len(res) > 0 {
			result = res[0].AsNumber()
		}

		It("doesn't fail", func() {
			Ω(err).Should(BeNil())
		})

		It("should be in the correct range", func() {
			Ω(result).Should(BeNumerically(">=", min))
			Ω(result).Should(BeNumerically("<=", max))
		})
	})
}

func validateRange(i interface{}, min, max float64) {
	It("is in the correct range", func() {
		Ω(i).Should(BeNumerically(">=", min))
		Ω(i).Should(BeNumerically("<=", max))
	})
}

var _ = Describe("Die", func() {
	e := lua.NewEngine()
	scripting.OpenLibs(e, "die")
	e.DoString(`
		local die = require("die")
		function callSimple(name)
			return die[name]()
		end

		function rollDie(str)
			return die.roll(str)
		end
	`)

	testDie(e, "d2", 1, 2)
	testDie(e, "d4", 1, 4)
	testDie(e, "d6", 1, 6)
	testDie(e, "d8", 1, 8)
	testDie(e, "d10", 1, 10)
	testDie(e, "d12", 1, 12)
	testDie(e, "d20", 1, 20)
	testDie(e, "d100", 1, 100)

	Describe("roll()", func() {
		res, err := e.Call("rollDie", 1, "3d8")
		var results []interface{}
		if len(res) > 0 {
			results = res[0].AsSliceInterface()
		}

		It("doesn't fail", func() {
			Ω(err).Should(BeNil())
		})

		It("generated 3 values", func() {
			Ω(results).Should(HaveLen(3))
		})

		for _, i := range results {
			validateRange(i, 1, 8)
		}
	})
})
