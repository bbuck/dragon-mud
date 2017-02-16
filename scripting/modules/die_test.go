package modules_test

import (
	"fmt"

	"github.com/bbuck/dragon-mud/scripting"
	"github.com/bbuck/dragon-mud/scripting/engine"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func testDie(e *engine.Lua, method string, min, max float64) {
	Describe(fmt.Sprintf("%s()", method), func() {
		res, err := e.Call("callSimple", 1, method)
		var result float64
		if len(res) > 0 {
			result = res[0].AsNumber()
		}

		It("doesn't fail", func() {
			Ω(err).To(BeNil())
		})

		It("should be in the correct range", func() {
			Ω(result).To(BeNumerically(">=", min))
			Ω(result).To(BeNumerically("<=", max))
		})
	})
}

func validateRange(i interface{}, min, max float64) {
	It("is in the correct range", func() {
		Ω(i).To(BeNumerically(">=", min))
		Ω(i).To(BeNumerically("<=", max))
	})
}

var _ = Describe("Die", func() {
	e := engine.NewLua()
	scripting.OpenDie(e)
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
			results = res[0].ToSlice()
		}

		It("doesn't fail", func() {
			Ω(err).To(BeNil())
		})

		It("generated 3 values", func() {
			Ω(results).To(HaveLen(3))
		})

		for _, i := range results {
			validateRange(i, 1, 8)
		}
	})
})
