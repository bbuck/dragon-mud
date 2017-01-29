// Copyright (c) 2016 Brandon Buck

package talon_test

import (
	. "github.com/bbuck/talon"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ConnectOptions", func() {
	Describe("URL", func() {
		var (
			co       ConnectOptions
			expected string
		)

		GeneratesCorrectURL := func() {
			It("generates the correct URL", func() {
				Ω(co.URL()).Should(Equal(expected))
			})
		}

		Context("with just a host", func() {
			BeforeEach(func() {
				co = ConnectOptions{
					Host: "test.com",
				}
				expected = "bolt://test.com"
			})

			GeneratesCorrectURL()
		})

		Context("with host and port set", func() {
			BeforeEach(func() {
				co = ConnectOptions{
					Host: "test.com",
					Port: 3333,
				}
				expected = "bolt://test.com:3333"
			})

			GeneratesCorrectURL()
		})

		Context("with username and host", func() {
			BeforeEach(func() {
				co = ConnectOptions{
					Host: "test.com",
					User: "example",
				}
				expected = "bolt://example@test.com"
			})

			GeneratesCorrectURL()
		})

		Context("with username, host and port", func() {
			BeforeEach(func() {
				co = ConnectOptions{
					Host: "test.com",
					Port: 3333,
					User: "example",
				}
				expected = "bolt://example@test.com:3333"
			})

			GeneratesCorrectURL()
		})

		Context("with username, password and host", func() {
			BeforeEach(func() {
				co = ConnectOptions{
					Host: "test.com",
					User: "example",
					Pass: "pword",
				}
				expected = "bolt://example:pword@test.com"
			})

			GeneratesCorrectURL()
		})

		Context("with all options", func() {
			BeforeEach(func() {
				co = ConnectOptions{
					Host: "test.com",
					Port: 3333,
					User: "example",
					Pass: "pword",
				}
				expected = "bolt://example:pword@test.com:3333"
			})

			GeneratesCorrectURL()
		})

		Context("with password but no user", func() {
			BeforeEach(func() {
				co = ConnectOptions{
					Host: "test.com",
					Pass: "pword",
				}
				expected = "bolt://test.com"
			})

			GeneratesCorrectURL()
		})
	})
})
