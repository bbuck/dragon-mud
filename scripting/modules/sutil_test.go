package modules_test

import (
	"fmt"

	"github.com/bbuck/dragon-mud/scripting"
	"github.com/bbuck/dragon-mud/scripting/lua"
	"github.com/bbuck/dragon-mud/scripting/pool"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("Sutil", func() {
	p := pool.NewEnginePool(4, func(eng *lua.Engine) {
		scripting.OpenLibs(eng, "sutil")
		eng.DoString(`sutil = require("sutil")`)
	})

	DescribeTable("split()",
		func(str, sep string, result []interface{}) {
			eng := p.Get()
			defer eng.Release()

			res, err := testReturn(eng.Engine, fmt.Sprintf("return sutil.split(%q, %q)", str, sep))
			var r []interface{}
			if err == nil && len(res) > 0 {
				r = res[0].AsSliceInterface()
			}

			Ω(err).Should(BeNil())
			Ω(r).Should(Equal(result))
		},
		Entry("splits single separator", "one two three", " ", []interface{}{"one", "two", "three"}),
		Entry("splits multi separator", "one, two, three", ", ", []interface{}{"one", "two", "three"}),
		Entry("no separator found", "one two three", ",", []interface{}{"one two three"}))

	DescribeTable("join()",
		func(strs []string, joiner, result string) {
			eng := p.Get()
			defer eng.Release()

			eng.SetGlobal("words", eng.TableFromSlice(strs))
			res, err := testReturn(eng.Engine, fmt.Sprintf("return sutil.join(words, %q)", joiner))
			var s string
			if err == nil && len(res) > 0 {
				s = res[0].AsString()
			}

			Ω(err).Should(BeNil())
			Ω(s).Should(Equal(result))
		},
		Entry("joins with single joiner", []string{"one", "two", "three"}, ",", "one,two,three"),
		Entry("joins with multi joiner", []string{"one", "two", "three"}, ", ", "one, two, three"),
		Entry("works on single input", []string{"one"}, ",", "one"))

	DescribeTable("text_rx()",
		func(needle, haystack string, result bool) {
			eng := p.Get()
			defer eng.Release()

			res, err := testReturn(eng.Engine, fmt.Sprintf("return sutil.test_rx(%q, %q)", needle, haystack))
			var b bool
			if err == nil && len(res) > 0 {
				b = res[0].AsBool()
			}

			Ω(err).Should(BeNil())
			Ω(b).Should(Equal(result))
		},
		Entry("finds simple regular expression", "abc", "abcdef", true),
		Entry("fails to find simple regular expression", "ghi", "abcdef", false),
		Entry("find more complex regular expression", "one|two", "two", true),
		Entry("fails to find more complex regular expression", "one|two", "three", false),
		Entry("matches complex pattern", `\d{0,3} ?\d{3}-\d{4}`, "123-4567", true),
		Entry("invalid regexp fails match", "(a", "one", false),
		Entry("allows flags to be set", "(?i)happy", "hApPy", true))

	DescribeTable("starts_with()",
		func(str, prefix string, result bool) {
			eng := p.Get()
			defer eng.Release()

			res, err := testReturn(eng.Engine, fmt.Sprintf("return sutil.starts_with(%q, %q)", str, prefix))
			var b bool
			if err == nil && len(res) > 0 {
				b = res[0].AsBool()
			}

			Ω(err).Should(BeNil())
			Ω(b).Should(Equal(result))
		},
		Entry("correctly determines starts with", "prefix", "pre", true),
		Entry("doesn't report false positives", "prefix", "post", false))

	DescribeTable("ends_with()",
		func(str, suffix string, result bool) {
			eng := p.Get()
			defer eng.Release()

			res, err := testReturn(eng.Engine, fmt.Sprintf("return sutil.ends_with(%q, %q)", str, suffix))
			var b bool
			if err == nil && len(res) > 0 {
				b = res[0].AsBool()
			}

			Ω(err).Should(BeNil())
			Ω(b).Should(Equal(result))
		},
		Entry("correctly determines ends with", "suffix", "fix", true),
		Entry("doesn't report false positives", "suffix", "post", false))

	DescribeTable("contains()",
		func(needle, haystack string, result bool) {
			eng := p.Get()
			defer eng.Release()

			res, err := testReturn(eng.Engine, fmt.Sprintf("return sutil.contains(%q, %q)", haystack, needle))
			var b bool
			if err == nil && len(res) > 0 {
				b = res[0].AsBool()
			}

			Ω(err).Should(BeNil())
			Ω(b).Should(Equal(result))
		},
		Entry("shows substring is present", "ll", "hello", true),
		Entry("doesn't show false positives", "bye", "hello", false))

	DescribeTable("matches()",
		func(rx, haystack string, result []interface{}) {
			eng := p.Get()
			defer eng.Release()

			res, err := testReturn(eng.Engine, fmt.Sprintf("return sutil.matches(%q, %q)", rx, haystack))
			var strs []interface{}
			if err == nil && len(res) > 0 {
				strs = res[0].AsSliceInterface()
			}

			Ω(err).Should(BeNil())
			Ω(strs).Should(Equal(result))
		},
		Entry("extracts parts of a string that match a regex", `t\w+`, "one, two, three", []interface{}{"two", "three"}),
		Entry("doesn't extract when no match", `a`, "bbb", make([]interface{}, 0)))
})
