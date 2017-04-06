// Copyright (c) 2016-2017 Brandon Buck

package types_test

import (
	"fmt"

	. "github.com/bbuck/dragon-mud/talon/types"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("Complex", func() {
	Describe("NewComplex", func() {
		var (
			zero complex128 = 0i
			c64  complex64  = 1 + 2i
			c128 complex128 = 3 + 6i
		)

		Context("when fed a complex64", func() {
			var (
				c64to128 = complex128(c64)
				c        = NewComplex(c64)
			)

			It("returns a wrapped complex128", func() {
				Ω(complex128(c)).Should(Equal(c64to128))
			})
		})

		Context("when fed a complex128", func() {
			c := NewComplex(c128)

			It("wraps the complex128", func() {
				Ω(complex128(c)).Should(Equal(c128))
			})
		})

		Context("when fed a non-complex value", func() {
			c := NewComplex("")

			It("returns a zero complex", func() {
				Ω(complex128(c)).Should(Equal(zero))
			})
		})
	})

	Describe("MarshalTalon", func() {
		var (
			c        Complex
			result   string
			err      error
			bs       []byte
			expected string
		)

		BeforeEach(func() {
			c = Complex(1 + 2i)
			bs, err = c.MarshalTalon()
			result = string(bs)
			expected = fmt.Sprintf("C!%g + %gi", 1.0, 2.0)
		})

		It("doesn't fail", func() {
			Ω(err).Should(BeNil())
		})

		It("produces the correct string", func() {
			Ω(result).Should(Equal(expected))
		})
	})

	DescribeTable("UnmarshalTalon",
		func(input []byte, expected complex128, toError bool) {
			var (
				c   Complex
				err error
			)

			c = NewComplex(nil)
			err = c.UnmarshalTalon(input)

			if toError {
				Ω(err).ShouldNot(BeNil())

				return
			}

			Ω(err).Should(BeNil())
			Ω(complex128(c)).Should(Equal(expected))
		},
		Entry("simple values", []byte("C!1 + 2i"), complex128(1+2i), false),
		Entry("decimal values", []byte("C!1.2345 + 2.3456i"), complex128(1.2345+2.3456i), false),
		Entry("exponent values", []byte("C!1.23e-10 + 2.34e+10i"), complex128(1.23e-10+2.34e+10i), false),
		Entry("invalid format", []byte("1 + 2i"), complex128(0i), true))
})
