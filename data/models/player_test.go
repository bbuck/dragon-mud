package models_test

import (
	. "github.com/bbuck/dragon-mud/data/models"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Player", func() {
	var player *Player

	BeforeEach(func() {
		player = &Player{
			Username:    "Izuriel",
			RawPassword: "password",
		}
	})

	Describe("saving", func() {
		BeforeEach(func() {
			player.BeforeSave()
		})

		It("should lowercase username", func() {
			Ω(player.Username).Should(Equal("izuriel"))
		})

		It("should reset RawPassword", func() {
			Ω(player.RawPassword).Should(Equal(""))
		})

		It("should configure data for password hash", func() {
			Ω(len(player.Password)).Should(BeNumerically(">", 0))
			Ω(player.Iterations).Should(And(
				BeNumerically(">=", uint32(3)),
				BeNumerically("<=", uint32(8)),
			))
			Ω(len(player.Salt)).Should(BeNumerically(">", 0))
		})
	})

	Describe("password matching", func() {
		BeforeEach(func() {
			player.BeforeSave()
		})

		It("should identify password matches", func() {
			Ω(player.IsValidPassword("password")).Should(BeTrue())
		})

		It("should reject password mismatches", func() {
			Ω(player.IsValidPassword("not password")).Should(BeFalse())
		})
	})
})
