package talon_test

import (
	. "github.com/bbuck/dragon-mud/talon"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("JSON", func() {
	Describe("MarshalTalon", func() {
		Describe("Map", func() {
			var (
				expected = []byte(`J!{"key":"value","other":1}`)
				m        = map[string]interface{}{
					"key":   "value",
					"other": 1,
				}
				bs  []byte
				err error
			)

			BeforeEach(func() {
				j := JSON{Data: m}
				bs, err = j.MarshalTalon()
			})

			It("doens't fail", func() {
				Ω(err).ShouldNot(HaveOccurred())
			})

			It("returns the expected value", func() {
				Ω(bs).Should(Equal(expected))
			})
		})

		Describe("Slice", func() {
			var (
				expected = []byte(`J!["value",1]`)
				s        = []interface{}{"value", 1}
				bs       []byte
				err      error
			)

			BeforeEach(func() {
				j := JSON{Data: s}
				bs, err = j.MarshalTalon()
			})

			It("doesn't fail", func() {
				Ω(err).ShouldNot(HaveOccurred())
			})

			It("returns the expected result", func() {
				Ω(bs).Should(Equal(expected))
			})
		})
	})
})
