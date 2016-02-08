package models_test

import (
	. "github.com/bbuck/dragon-mud/data/models"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("BaseModel", func() {
	Describe("migrations", func() {
		It("returns nil if migrating a second time", func() {
			// First migration is called in BeforeSuite
			err := MigrateDatabase()
			Ω(err).Should(BeNil())
		})
	})
})
