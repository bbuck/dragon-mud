// Copyright (c) 2016-2017 Brandon Buck

package types_test

import (
	"fmt"
	"time"

	. "github.com/bbuck/dragon-mud/talon/types"

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
			date          = time.Date(1986, time.November, 12, 1, 2, 3, 4, time.Local)
			dateStr       = fmt.Sprintf("T!RFC3339!!%s", date.Format(DefaultTimeFormat))
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
				Ω(after).Should(HaveKeyWithValue("test_date", dateStr))
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
					"props": Properties{},
				}

				after, err = before.MarshaledProperties()
			})

			It("fails to marshal nested properties", func() {
				Ω(err).ShouldNot(BeNil())
			})

			It("fails with nested properties error", func() {
				Ω(err).Should(Equal(ErrNoNestedProperties))
			})
		})

		Context("nested map", func() {
			BeforeEach(func() {
				before = Properties{
					"props": map[string]string{},
				}

				after, err = before.MarshaledProperties()
			})

			It("fails to marshal nested collections", func() {
				Ω(err).ShouldNot(BeNil())
			})

			It("fails with nested collections error", func() {
				Ω(err).Should(Equal(ErrNoRawCollections))
			})
		})

		Context("nested slice", func() {
			BeforeEach(func() {
				before = Properties{
					"props": []string{},
				}

				after, err = before.MarshaledProperties()
			})

			It("fails to marshal nested collections", func() {
				Ω(err).ShouldNot(BeNil())
			})

			It("fails with nested collections error", func() {
				Ω(err).Should(Equal(ErrNoRawCollections))
			})
		})
	})

	Describe("UnmarshaledProperties", func() {
		var (
			str           = "string"
			date          = time.Date(1986, time.November, 12, 1, 2, 3, 0, time.Local)
			dateStr       = fmt.Sprintf("T!RFC3339!!%s", date.Format(DefaultTimeFormat))
			cmplx         = complex64(1 + 2i)
			cmplxStr      = "C!1 + 2i"
			before, after Properties
			err           error
		)

		BeforeEach(func() {
			before = Properties{
				"test_date":    dateStr,
				"test_complex": cmplxStr,
				"test_string":  str,
			}

			after, err = before.UnmarshaledProperties()
		})

		It("doesn't fail", func() {
			Ω(err).Should(BeNil())
		})

		It("contains the correct number of keys", func() {
			Ω(after).Should(HaveLen(3))
		})

		It("marshales dates in the list", func() {
			Ω(after).Should(HaveKey("test_date"))
			d, ok := after["test_date"].(*Time)
			Ω(ok).Should(BeTrue())
			Ω(date.Equal(d.Time)).Should(BeTrue())
		})

		It("marshales complex types", func() {
			c, ok := after["test_complex"].(*Complex)
			Ω(ok).Should(BeTrue())
			Ω(complex64(*c)).Should(Equal(cmplx))
		})

		It("doesn't alter strings", func() {
			Ω(after).Should(HaveKeyWithValue("test_string", str))
		})
	})
})
