package models_test

import (
	. "github.com/bbuck/dragon-mud/data/models"
	"github.com/bbuck/dragon-mud/data/models/players"
	"github.com/bbuck/dragon-mud/scripting"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ScriptablePlayer", func() {
	var player *Player

	BeforeEach(func() {
		player = players.New("Izuriel", "password")
		Save(player)
	})

	AfterEach(func() {
		player.DB().Unscoped().Delete(player)
		player = nil
	})

	Describe("Player", func() {
		It("conforms to scripting.ScriptableObject", func() {
			var iface interface{} = player
			_, ok := iface.(scripting.ScriptableObject)
			Ω(ok).Should(BeTrue())
		})

		It("returns a ScriptablePlayer", func() {
			obj := player.ScriptObject()
			_, ok := obj.(*ScriptablePlayer)
			Ω(ok).Should(BeTrue())
		})
	})

	Describe("methods", func() {
		var sp *ScriptablePlayer

		BeforeEach(func() {
			obj := player.ScriptObject()
			sp, _ = obj.(*ScriptablePlayer)
		})

		It("returns a different value for DisplayName", func() {
			Ω(sp.DisplayName()).ShouldNot(Equal(player.DisplayName))
		})
	})
})
