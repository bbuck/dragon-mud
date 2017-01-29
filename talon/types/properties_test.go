// Copyright (c) 2016 Brandon Buck

package types_test

import (
	. "github.com/bbuck/talon/types"

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
})
