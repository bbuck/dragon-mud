package assets_test

import (
	. "github.com/bbuck/dragon-mud/assets"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Existence", func() {
	Describe("default_config.toml", func() {
		It("should exist as an asset file", func() {
			_, err := Asset("default_config.toml")
			Ω(err).Should(BeNil())
		})
	})
})
