package random_test

import (
	"math/rand"

	. "github.com/bbuck/dragon-mud/random"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Die", func() {
	BeforeEach(func() {
		SetSource(rand.NewSource(1))
	})

	Describe("D2", func() {
		It("generates a random number", func() {
			Ω(D2()).Should(Equal(1))
		})
	})

	Describe("D4", func() {
		It("generates a random number", func() {
			Ω(D4()).Should(Equal(3))
		})
	})

	Describe("D6", func() {
		It("generates a random number", func() {
			Ω(D6()).Should(Equal(2))
		})
	})

	Describe("D8", func() {
		It("generates a random number", func() {
			Ω(D8()).Should(Equal(7))
		})
	})

	Describe("D10", func() {
		It("generates a random number", func() {
			Ω(D10()).Should(Equal(6))
		})
	})

	Describe("D12", func() {
		It("generates a random number", func() {
			Ω(D12()).Should(Equal(2))
		})
	})

	Describe("D20", func() {
		It("generates a random number", func() {
			Ω(D20()).Should(Equal(6))
		})
	})

	Describe("D100", func() {
		It("generates a random number", func() {
			Ω(D100()).Should(Equal(24))
		})
	})

	Describe("RollDie", func() {
		It("works without specifying a die count", func() {
			Ω(RollDie("d10")).Should(Equal([]int{6}))
		})

		It("works with a die count", func() {
			Ω(RollDie("2d20")).Should(Equal([]int{6, 15}))
		})

		It("returns nothing with an invalid count", func() {
			Ω(RollDie("ad10")).Should(Equal([]int{}))
		})

		It("returns nothing with an invalid string", func() {
			Ω(RollDie("a")).Should(Equal([]int{}))
		})

		It("returns nothing with an invalid side count", func() {
			Ω(RollDie("1da")).Should(Equal([]int{}))
		})

		It("generates values within the range specified", func() {
			ret := RollDie("100000d2")
			for _, n := range ret {
				Ω(n).Should(BeNumerically(">=", 1))
				Ω(n).Should(BeNumerically("<=", 2))
			}
		})
	})
})
