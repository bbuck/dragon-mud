package engine_test

import (
	. "github.com/bbuck/dragon-mud/scripting/engine"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("LuaEngine", func() {
	var (
		err          error
		engine       *Lua
		stringScript = `
			function hello(name)
				return "Hello, " .. name .. "!"
			end
		`
	)

	BeforeEach(func() {
		engine = NewLua()
	})

	AfterEach(func() {
		engine.Close()
	})

	Context("when closed", func() {
		BeforeEach(func() {
			engine.Close()
		})

		It("no longer functions", func() {
			_, err := engine.Call("hello", 1, "World")
			Ω(err).ShouldNot(BeNil())
		})
	})

	Context("when loading from a string", func() {
		BeforeEach(func() {
			err = engine.LoadString(stringScript)
		})

		It("should not fail", func() {
			Ω(err).To(BeNil())
		})

		Context("when calling a method", func() {
			var (
				results []*LuaValue
				err     error
			)

			BeforeEach(func() {
				results, err = engine.Call("hello", 1, "World")
			})

			It("does not return an error", func() {
				Ω(err).Should(BeNil())
			})

			It("returns 1 result", func() {
				Ω(len(results)).Should(Equal(1))
			})

			It("doesn't return nil", func() {
				Ω(results[0]).ShouldNot(Equal(Nil))
			})

			It("returns the string 'Hello, World!'", func() {
				Ω(results[0].AsString()).Should(Equal("Hello, World!"))
			})
		})
	})

	Context("when loading from a file", func() {
		BeforeEach(func() {
			err = engine.LoadFile(fileName)
		})

		It("shoult not fail", func() {
			Ω(err).To(BeNil())
		})

		Context("when calling a method", func() {
			var (
				results []*LuaValue
				err     error
			)

			BeforeEach(func() {
				results, err = engine.Call("give_me_one", 1)
			})

			It("does not return an error", func() {
				Ω(err).Should(BeNil())
			})

			It("return 1 result", func() {
				Ω(len(results)).Should(Equal(1))
			})

			It("does not return nil", func() {
				Ω(results[0]).ShouldNot(Equal(Nil))
			})

			It("returns the number 1", func() {
				Ω(results[0].AsNumber()).Should(Equal(float64(1)))
			})
		})
	})

	Describe("Call()", func() {
		var (
			results []*LuaValue
			err     error
			script  = `
				function swap(a, b)
					return b, a
				end
			`
			a                float64 = 10.0
			b                float64 = 20.0
			aResult, bResult float64
		)

		BeforeEach(func() {
			engine.LoadString(script)
			results, err = engine.Call("swap", 2, a, b)
			if err == nil {
				aResult = results[0].AsNumber()
				bResult = results[1].AsNumber()
			}
		})

		It("does not return an error", func() {
			Ω(err).Should(BeNil())
		})

		It("returns two results", func() {
			Ω(len(results)).Should(Equal(2))
		})

		It("returns the second input first", func() {
			Ω(aResult).Should(Equal(b))
		})

		It("returns the first input second", func() {
			Ω(bResult).Should(Equal(a))
		})
	})

	Describe("SetGlobal()", func() {
		var (
			results []*LuaValue
			err     error
		)

		BeforeEach(func() {
			engine.SetGlobal("gbl", "testing")
			err = engine.LoadString(`
			function get_gbl()
				return gbl
			end
			`)
			if err != nil {
				Fail(err.Error())
			}
			results, err = engine.Call("get_gbl", 1)
		})

		It("does not fail", func() {
			Ω(err).Should(BeNil())
		})

		It("returns one value", func() {
			Ω(len(results)).Should(Equal(1))
		})

		It("returns the value assigned to the global", func() {
			Ω(results[0].AsString()).Should(Equal("testing"))
		})
	})

	Describe("GetGlobal()", func() {
		var (
			value *LuaValue
			err   error
		)

		BeforeEach(func() {
			err = engine.LoadString(`
				word = "testing"
			`)
			if err != nil {
				Fail(err.Error())
			}
			value = engine.GetGlobal("word")
		})

		It("doesn't return nil", func() {
			Ω(value).ShouldNot(Equal(Nil))
		})

		It("returns the correct string", func() {
			Ω(value.AsString()).Should(Equal("testing"))
		})
	})

	Describe("RegisterFunc()", func() {
		Context("when registering a raw Go function", func() {
			var (
				results []*LuaValue
				err     error
				called  bool
			)

			BeforeEach(func() {
				engine.RegisterFunc("add", func(a, b int) int {
					called = true
					return a + b
				})
				results, err = engine.Call("add", 1, 10, 11)
			})

			It("should no fail", func() {
				Ω(err).Should(BeNil())
			})

			It("marks the called variable", func() {
				Ω(called).Should(BeTrue())
			})

			It("does not return nil", func() {
				Ω(results[0]).ShouldNot(Equal(Nil))
			})

			It("returns 1 value", func() {
				Ω(len(results)).Should(Equal(1))
			})

			It("returns a value that passed through the Go function", func() {
				Ω(results[0].AsNumber()).Should(Equal(float64(21)))
			})
		})

		Context("when registering a lua specific function", func() {
			var (
				results []*LuaValue
				err     error
				called  bool
			)

			BeforeEach(func() {
				engine.RegisterFunc("sub", func(e *Lua) int {
					second := e.PopInt64()
					first := e.PopInt64()

					if first == 11 && second == 10 {
						called = true
					}

					e.PushValue(first - second)

					return 1
				})
				results, err = engine.Call("sub", 1, 11, 10)
			})

			It("does not fail", func() {
				Ω(err).Should(BeNil())
			})

			It("returns 1 value", func() {
				Ω(len(results)).Should(Equal(1))
			})

			It("marks the variable called", func() {
				Ω(called).Should(BeTrue())
			})

			It("does not return nil", func() {
				Ω(results[0]).ShouldNot(Equal(Nil))
			})

			It("returns the correct value", func() {
				Ω(results[0].AsNumber()).Should(Equal(float64(1)))
			})
		})
	})

	Describe("passing in go objects", func() {
		var obj = TestObject{}

		BeforeEach(func() {
			engine.LoadString(`
				function call_by_value_fn(obj)
				  return obj:GetStringFromValue()
				end

				function call_by_ptr_fn(obj)
					return obj:GetStringFromPtr()
				end
			`)
		})

		Context("calling methods by value", func() {
			var (
				result []*LuaValue
				cerr   error
			)

			BeforeEach(func() {
				result, cerr = engine.Call("call_by_value_fn", 1, obj)
			})

			It("should not fail", func() {
				Ω(cerr).Should(BeNil())
			})

			It("should return the correct value", func() {
				Ω(len(result)).Should(BeNumerically(">", 0))
				Ω(result[0].AsString()).Should(Equal("success"))
			})
		})

		Context("calling methods by pointer", func() {
			var (
				result []*LuaValue
				cerr   error
			)

			BeforeEach(func() {
				result, cerr = engine.Call("call_by_ptr_fn", 1, &obj)
			})

			It("should not fail", func() {
				Ω(cerr).Should(BeNil())
			})

			It("should return the correct value", func() {
				Ω(len(result)).Should(BeNumerically(">", 0))
				Ω(result[0].AsString()).Should(Equal("success"))
			})
		})
	})
})

type TestObject struct{}

func (t TestObject) GetStringFromValue() string {
	return "success"
}

func (t *TestObject) GetStringFromPtr() string {
	return "success"
}
