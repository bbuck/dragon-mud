package lua_test

import (
	"fmt"

	. "github.com/bbuck/dragon-mud/scripting/lua"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("LuaValue", func() {
	var (
		engine *Engine
		str    = "testing"
		i      = int(10)
		i64    = int64(100)
		f64    = float64(11.839)
		b      = true
		fn     = func(a, b int) int {
			return a + b
		}
	)
	value := func(iface interface{}) *Value {
		return engine.ValueFor(iface)
	}

	BeforeEach(func() {
		engine = NewEngine()
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

	DescribeTable("IsString()",
		func(v interface{}, expected bool) {
			Ω(value(v).IsString()).Should(Equal(expected))
		},
		Entry("thinks a string is a string", str, true),
		Entry("does not think a number is a string", i, false),
		Entry("does not think a boolean is a string", b, false),
		Entry("does not think a function is a string", fn, false),
		Entry("does not think nil is a string", Nil, false),
	)

	Context("with a table as a list", func() {
		var list *Value

		BeforeEach(func() {
			list = engine.NewTable()
			list.Append(str)
			list.Append(i)
			list.Append(fn)
		})

		It("has a length of 3", func() {
			Ω(list.Len()).Should(Equal(3))
		})

		It("contains a string at index 1", func() {
			Ω(list.Get(1).AsString()).Should(Equal(str))
		})

		It("contains a number at index 2", func() {
			Ω(list.Get(2).AsNumber()).Should(Equal(float64(i)))
		})

		It("contains a function at index 3", func() {
			Ω(list.Get(3).IsFunction()).Should(BeTrue())
		})

		Context("when calling functions on the list", func() {
			var (
				results []*Value
				err     error
			)

			BeforeEach(func() {
				results, err = list.Get(3).Call(1, i, i64)
			})

			It("should not fail", func() {
				Ω(err).Should(BeNil())
			})

			It("should return 1 result", func() {
				Ω(len(results)).Should(Equal(1))
			})

			It("should return the correct value", func() {
				Ω(results[0].AsNumber()).Should(Equal(float64(int64(i) + i64)))
			})
		})

		Context("iterating over a list", func() {
			var (
				isString   bool
				isNumber   bool
				isFunction bool
			)

			BeforeEach(func() {
				list.ForEach(func(key, val *Value) {
					i := int(key.AsNumber())
					switch i {
					case 1:
						isString = val.IsString()
					case 2:
						isNumber = val.IsNumber()
					case 3:
						isFunction = val.IsFunction()
					}
				})
			})

			It("found a string", func() {
				Ω(isString).Should(BeTrue())
			})

			It("found a number", func() {
				Ω(isNumber).Should(BeTrue())
			})

			It("found a function", func() {
				Ω(isFunction).Should(BeTrue())
			})
		})

		Context("when inserting values", func() {
			BeforeEach(func() {
				list.Insert(2, i64)
			})

			It("changed the value at index 2", func() {
				Ω(list.Get(2).AsNumber()).Should(Equal(float64(i64)))
			})
		})

		Context("when removing a value", func() {
			BeforeEach(func() {
				list.Remove(2)
			})

			It("remove the value at index 2", func() {
				Ω(list.Get(2).IsFunction()).Should(BeTrue())
			})
		})
	})

	Describe("AsMapStringInterface()", func() {
		var (
			table *Value
			m     map[string]interface{}
		)

		BeforeEach(func() {
			table = engine.NewTable()
			table.Set("one", 1)
			table.Set("two", "two")
			m = table.AsMapStringInterface()
		})

		It("has two keys", func() {
			Ω(m).Should(HaveLen(2))
		})

		It("has the number 1 at 'one'", func() {
			n, ok := m["one"]
			Ω(ok).Should(BeTrue())
			Ω(n).Should(Equal(float64(1)))
		})

		It("has the string 'two' at 'two'", func() {
			s, ok := m["two"]
			Ω(ok).Should(BeTrue())
			Ω(s).Should(Equal("two"))
		})
	})

	Describe("AsSliceInterface()", func() {
		var (
			table *Value
			s     []interface{}
		)

		BeforeEach(func() {
			table = engine.NewTable()
			table.Append(2)
			table.Append(1)
			s = table.AsSliceInterface()
		})

		It("has a length of 2", func() {
			Ω(s).Should(HaveLen(2))
		})

		It("has the value 2 at index 0", func() {
			Ω(s[0]).Should(Equal(float64(2)))
		})

		It("has the value 1 at index 1", func() {
			Ω(s[1]).Should(Equal(float64(1)))
		})
	})
})
