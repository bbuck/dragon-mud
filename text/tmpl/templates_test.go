package tmpl_test

import (
	. "github.com/bbuck/dragon-mud/text/tmpl"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var testTemplate = "Hello, {{name}}!"

var _ = Describe("Templates", func() {
	Describe("Register", func() {
		var err error

		BeforeEach(func() {
			err = Register(testTemplate, "test")
		})

		AfterEach(func() {
			Unregister("test")
		})

		It("should not error", func() {
			Ω(err).To(BeNil())
		})
	})

	Describe("Template", func() {
		var (
			r   Renderer
			err error
		)

		Context("with a registered template", func() {
			BeforeEach(func() {
				Register(testTemplate, "test")
				r, err = Template("test")
			})

			AfterEach(func() {
				Unregister("test")
			})

			It("does not return an error", func() {
				Ω(err).To(BeNil())
			})

			It("returns an error", func() {
				Ω(r).ToNot(BeNil())
			})
		})

		Context("without a registered template", func() {
			BeforeEach(func() {
				r, err = Template("test")
			})

			It("does not return an error", func() {
				Ω(r).To(BeNil())
			})

			It("does return an error", func() {
				Ω(err).ToNot(BeNil())
			})
		})
	})
})
