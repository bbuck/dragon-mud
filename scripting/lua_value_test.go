package scripting_test

import (
	"fmt"

	. "github.com/bbuck/dragon-mud/scripting"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("LuaValue", func() {
	var (
		engine *LuaEngine
		str    = "testing"
		i      = int(10)
		i64    = int64(100)
		f64    = float64(11.839)
		b      = true
		fn     = func(a, b int) int {
			return a + b
		}
		value = func(iface interface{}) *LuaValue {
			return engine.ValueFor(iface)
		}
	)

	BeforeEach(func() {
		engine = NewLuaEngine()
	})

	AfterEach(func() {
		engine.Close()
	})

	It("conforms to fmt.Stringer", func() {
		var iface interface{} = value(str)
		str, ok := iface.(fmt.Stringer)
		Ω(ok).Should(BeTrue())
		Ω(len(str.String())).Should(BeNumerically(">", 0))
	})

	DescribeTable("AsString()",
		func(val interface{}, expected string) {
			Ω(value(val).AsString()).Should(Equal(expected))
		},
		Entry("handles strings", str, str),
		Entry("handles ints", i, "10"),
		Entry("handles int64s", i64, "100"),
		Entry("handles float64", f64, "11.839"),
	)

	DescribeTable("AsFloat()",
		func(val interface{}, expected float64) {
			Ω(value(val).AsFloat()).Should(Equal(expected))
		},
		Entry("handles int values", i, float64(i)),
		Entry("handles int64 values", i64, float64(i64)),
		Entry("handles float64 values", f64, f64),
	)

	DescribeTable("AsNumber()",
		func(val interface{}, expected interface{}) {
			Ω(value(val).AsNumber()).Should(Equal(value(expected).AsFloat()))
		},
		Entry("behaves just like AsFloat()", i, i),
	)

	DescribeTable("AsBool()",
		func(val interface{}, expected bool) {
			Ω(value(val).AsBool()).Should(Equal(expected))
		},
		Entry("handles bool values", b, true),
		Entry("converts strings to bools", str, true),
		Entry("converts numbers to bools", i, true),
	)

	DescribeTable("IsTrue()",
		func(val interface{}, expected bool) {
			Ω(value(val).IsTrue()).Should(Equal(expected))
		},
		Entry("handles true", true, true),
		Entry("handles false", false, false),
		Entry("thinks strings are true", str, true),
		Entry("thinks numbers are true", i, true),
		Entry("thinks nil is not true", Nil, false),
		Entry("thinks functions are true", fn, true),
	)

	DescribeTable("IsFalse()",
		func(val interface{}, expected bool) {
			Ω(value(val).IsFalse()).Should(Equal(expected))
		},
		Entry("handles true", true, false),
		Entry("handles false", false, true),
		Entry("thinks strings aren't false", str, false),
		Entry("thinks numbers are't false", i, false),
		Entry("thinks nil is false", Nil, true),
		Entry("does not think functions are false", fn, false),
	)

	DescribeTable("IsNil()",
		func(val interface{}, expected bool) {
			Ω(value(val).IsNil()).Should(Equal(expected))
		},
		Entry("does not think strings are nil", str, false),
		Entry("does not think ints are nil", i, false),
		Entry("does not think int64s are nil", i64, false),
		Entry("does not think float64s are nil", f64, false),
		Entry("thinks nil is nil", Nil, true),
	)

	DescribeTable("IsNumber()",
		func(v interface{}, expected bool) {
			Ω(value(v).IsNumber()).Should(Equal(expected))
		},
		Entry("does not think strings are numbers", str, false),
		Entry("thinks ints are numbers", i, true),
		Entry("thinks int64s are numbers", i64, true),
		Entry("thinks float64s are number", f64, true),
		Entry("doesn't think nil is a number", Nil, false),
	)

	DescribeTable("IsBool()",
		func(v interface{}, expected bool) {
			Ω(value(v).IsBool()).Should(Equal(expected))
		},
		Entry("thinks true is a bool", true, true),
		Entry("thinks false is a bool", false, true),
		Entry("does not think a string is a bool", str, false),
		Entry("does not think a number is a bool", i, false),
		Entry("does not think nil is a bool", Nil, false),
	)

	DescribeTable("IsFunction()",
		func(v interface{}, expected bool) {
			Ω(value(v).IsFunction()).Should(Equal(expected))
		},
		Entry("thinks functions are functions", fn, true),
		Entry("does not think strings are functions", str, false),
		Entry("does not think numbers are functions", i, false),
		Entry("does not think nil is a function", Nil, false),
	)
})
