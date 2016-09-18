package models_test

import (
	. "github.com/bbuck/dragon-mud/data/models"
	"github.com/bbuck/dragon-mud/data/models/players"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Player", func() {
	var player *Player

	BeforeEach(func() {
		player = &Player{
			Username: "Izuriel",
		}
		player.SetPassword("password")
	})

	Describe("saving", func() {
		BeforeEach(func() {
			Save(player)
		})

		AfterEach(func() {
			player.DB().Unscoped().Delete(player)
		})

		It("should lowercase username", func() {
			Ω(player.Username).Should(Equal("izuriel"))
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
		It("should identify password matches", func() {
			Ω(player.IsValidPassword("password")).Should(BeTrue())
		})

		It("should reject password mismatches", func() {
			Ω(player.IsValidPassword("not password")).Should(BeFalse())
		})
	})

	Describe("with database functions", func() {
		Context("with a value in the database", func() {
			var (
				player  *Player
				oPlayer *Player
				found   bool
			)

			BeforeEach(func() {
				player = &Player{
					DisplayName: "TestName",
				}
				player.SetPassword("password")
				Save(player)
				oPlayer, found = players.FindByUsername("TestName")
			})

			AfterEach(func() {
				player.DB().Unscoped().Delete(player)
				player = nil
				oPlayer = nil
			})

			It("is findable by username", func() {
				Ω(found).Should(Equal(true))
				Ω(oPlayer.ID).Should(Equal(player.ID))
			})

			It("is findable by id", func() {
				ooPlayer := new(Player)
				ByID(player.ID).First(&ooPlayer)
				Ω(oPlayer.ID).Should(Equal(player.ID))
			})

			It("preserves the original display name", func() {
				Ω(oPlayer.ID).Should(Equal(player.ID))
				Ω(oPlayer.DisplayName).Should(Equal("TestName"))
			})
		})
	})
})
