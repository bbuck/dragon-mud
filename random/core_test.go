package random_test

import (
	"math/rand"

	. "github.com/bbuck/dragon-mud/random"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Core", func() {
	BeforeEach(func() {
		SetSource(rand.NewSource(1))
	})

	Describe("Intn", func() {
		It("generates a random number", func() {
			Ω(Intn(10)).To(Equal(1))
		})
	})

	Describe("Range", func() {
		It("generates a random number", func() {
			Ω(Range(1, 6)).To(Equal(2))
		})

		It("generates a number between maximum and minimum value", func() {
			for i := 0; i < 100000; i++ {
				val := Range(10, 20)
				Ω(val).To(BeNumerically(">=", 10))
				Ω(val).To(BeNumerically("<=", 20))
			}
		})
	})
})
