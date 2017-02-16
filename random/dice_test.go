package random_test

import (
	"math/rand"

	. "github.com/bbuck/dragon-mud/random"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

// The tests found here say "choose a random number" and as the random generated
// is seeded with 1 these tests specify the values that will be generted with a
// seed value of 1, so if you find it odd "generates a random number" has a
// matcher like "should equal 1" it's because a predictable source.

var _ = Describe("Die", func() {
	BeforeEach(func() {
		SetSource(rand.NewSource(1))
	})

	Describe("D2", func() {
		It("generates a random number", func() {
			Ω(D2()).To(Equal(1))
		})
	})

	Describe("D4", func() {
		It("generates a random number", func() {
			Ω(D4()).To(Equal(3))
		})
	})

	Describe("D6", func() {
		It("generates a random number", func() {
			Ω(D6()).To(Equal(2))
		})
	})

	Describe("D8", func() {
		It("generates a random number", func() {
			Ω(D8()).To(Equal(7))
		})
	})

	Describe("D10", func() {
		It("generates a random number", func() {
			Ω(D10()).To(Equal(6))
		})
	})

	Describe("D12", func() {
		It("generates a random number", func() {
			Ω(D12()).To(Equal(2))
		})
	})

	Describe("D20", func() {
		It("generates a random number", func() {
			Ω(D20()).To(Equal(6))
		})
	})

	Describe("D100", func() {
		It("generates a random number", func() {
			Ω(D100()).To(Equal(24))
		})
	})

	Describe("RollDie", func() {
		It("works without specifying a die count", func() {
			Ω(RollDie("d10")).To(Equal([]int{6}))
		})

		It("works with a die count", func() {
			Ω(RollDie("2d20")).To(Equal([]int{6, 15}))
		})

		It("returns nothing with an invalid count", func() {
			Ω(RollDie("ad10")).To(Equal([]int{}))
		})

		It("returns nothing with an invalid string", func() {
			Ω(RollDie("a")).To(Equal([]int{}))
		})

		It("returns nothing with an invalid side count", func() {
			Ω(RollDie("1da")).To(Equal([]int{}))
		})

		It("generates values within the range specified", func() {
			ret := RollDie("100000d2")
			for _, n := range ret {
				Ω(n).To(BeNumerically(">=", 1))
				Ω(n).To(BeNumerically("<=", 2))
			}
		})
	})
})
