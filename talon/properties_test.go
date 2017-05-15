// Copyright (c) 2016-2017 Brandon Buck

package talon_test

import (
	"time"

	. "github.com/bbuck/dragon-mud/talon"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Properties", func() {
	var p Properties

	BeforeEach(func() {
		p = make(Properties)
	})

	Describe("QueryString", func() {
		Context("when the property map is empty", func() {
			It("is just an empty string", func() {
				Ω(p.QueryString()).Should(Equal(""))
			})
		})

		Context("with a single property", func() {
			BeforeEach(func() {
				p["one"] = "two"
			})

			It("is a key-insertion pairing", func() {
				Ω(p.QueryString()).Should(Equal(`{one: {one}}`))
			})
		})

		Context("with more than one property", func() {
			BeforeEach(func() {
				p["one"] = "two"
				p["three"] = "four"
			})

			It("is a key-insertion pairing", func() {
				Ω(p.QueryString()).Should(Equal(`{one: {one}, three: {three}}`))
			})
		})

		Context("with conflicting keys during merge", func() {
			BeforeEach(func() {
				b := make(Properties)
				b["one"] = "three"
				p["one"] = "two"
				p = p.Merge(b)
			})

			It("is a key-insertion pairing", func() {
				Ω(p.QueryString()).Should(Equal(`{one: {one}}`))
			})
		})
	})

	Describe("MarshaledProperties", func() {
		var (
			str           = "string"
			date          = time.Date(1986, time.November, 12, 1, 2, 3, 4, time.UTC)
			ts            = int64(532141323)
			cmplx         = 1 + 2i
			cmplxStr      = "C!1 + 2i"
			before, after Properties
			err           error
		)

		Context("used normally", func() {
			BeforeEach(func() {
				before = Properties{
					"test_date":    date,
					"test_complex": cmplx,
					"test_string":  str,
				}

				after, err = before.MarshaledProperties()
			})

			It("doesn't fail", func() {
				Ω(err).Should(BeNil())
			})

			It("contains the correct number of keys", func() {
				Ω(after).Should(HaveLen(3))
			})

			It("marshales dates in the list", func() {
				Ω(after).Should(HaveKeyWithValue("test_date", ts))
			})

			It("marshales complex types", func() {
				Ω(after).Should(HaveKeyWithValue("test_complex", cmplxStr))
			})

			It("doesn't alter strings", func() {
				Ω(after).Should(HaveKeyWithValue("test_string", str))
			})
		})

		Context("nested properties", func() {
			BeforeEach(func() {
				before = Properties{
					"props": Properties{"key": "value"},
				}

				after, err = before.MarshaledProperties()
			})

			It("marshals with nested properties", func() {
				Ω(err).ShouldNot(HaveOccurred())
			})

			It("produces the correct result", func() {
				Ω(after).Should(HaveKeyWithValue("props", `P!{"key":"value"}`))
			})
		})

		Context("nested map", func() {
			BeforeEach(func() {
				before = Properties{
					"props": map[string]string{"key": "value"},
				}

				after, err = before.MarshaledProperties()
			})

			It("doesn't fail to marshal maps", func() {
				Ω(err).ShouldNot(HaveOccurred())
			})

			It("returns the correct value", func() {
				Ω(after).Should(HaveKeyWithValue("props", `J!{"key":"value"}`))
			})
		})

		Context("nested slice", func() {
			BeforeEach(func() {
				before = Properties{
					"props": []interface{}{"one", 1},
				}

				after, err = before.MarshaledProperties()
			})

			It("doesn't fail to marshal slices", func() {
				Ω(err).ShouldNot(HaveOccurred())
			})

			It("retursn the correct value", func() {
				Ω(after).Should(HaveKeyWithValue("props", `J!["one",1]`))
			})
		})
	})

	Describe("UnmarshaledProperties", func() {
		var (
			str           = "string"
			ts            = int64(532162923)
			cmplx         = complex128(1 + 2i)
			cmplxStr      = "C!1 + 2i"
			props         = `P!{"key":"value"}`
			m             = `J!{"one":2}`
			s             = `J![3,true]`
			before, after Properties
			err           error
		)

		BeforeEach(func() {
			before = Properties{
				"test_date":    ts,
				"test_complex": cmplxStr,
				"test_string":  str,
				"test_props":   props,
				"test_map":     m,
				"test_slice":   s,
			}

			after, err = before.UnmarshaledProperties()
		})

		It("doesn't fail", func() {
			Ω(err).Should(BeNil())
		})

		It("contains the correct number of keys", func() {
			Ω(after).Should(HaveLen(6))
		})

		It("marshales dates in the list", func() {
			Ω(after).Should(HaveKeyWithValue("test_date", ts))
		})

		It("marshales complex types", func() {
			c, ok := after["test_complex"].(complex128)
			Ω(ok).Should(BeTrue())
			Ω(c).Should(Equal(cmplx))
		})

		It("doesn't alter strings", func() {
			Ω(after).Should(HaveKeyWithValue("test_string", str))
		})

		It("unmarshals Property types", func() {
			Ω(after).Should(HaveKey("test_props"))

			p, _ := after["test_props"].(Properties)
			Ω(p).Should(HaveKeyWithValue("key", "value"))
		})

		It("unmarshals maps", func() {
			Ω(after).Should(HaveKey("test_map"))
			m, ok := after["test_map"].(map[string]interface{})
			Ω(ok).Should(BeTrue())
			Ω(m).Should(HaveKeyWithValue("one", float64(2)))
		})

		It("unmarshals slices", func() {
			Ω(after).Should(HaveKey("test_slice"))
			s, ok := after["test_slice"].([]interface{})
			Ω(ok).Should(BeTrue())
			Ω(s).Should(ConsistOf(float64(3), true))
		})
	})
})
