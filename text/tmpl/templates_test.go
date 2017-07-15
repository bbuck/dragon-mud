package tmpl_test

import (
	. "github.com/bbuck/dragon-mud/text/tmpl"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var testTemplate = "Hello, {{ Name }}!"

var _ = Describe("Templates", func() {
	Describe("Register", func() {
		var err error

		BeforeEach(func() {
			err = Register("test", testTemplate)
		})

		AfterEach(func() {
			Unregister("test")
		})

		It("should not error", func() {
			Ω(err).Should(BeNil())
		})
	})

	Describe("Template", func() {
		var (
			r   Renderer
			err error
		)

		Context("with a registered template", func() {
			BeforeEach(func() {
				Register("test", testTemplate)
				r, err = Template("test")
			})

			AfterEach(func() {
				Unregister("test")
			})

			It("does not return an error", func() {
				Ω(err).Should(BeNil())
			})

			It("returns an error", func() {
				Ω(r).ShouldNot(BeNil())
			})
		})

		Context("without a registered template", func() {
			BeforeEach(func() {
				r, err = Template("test")
			})

			It("does not return an error", func() {
				Ω(r).Should(BeNil())
			})

			It("does return an error", func() {
				Ω(err).ShouldNot(BeNil())
			})
		})
	})
})
