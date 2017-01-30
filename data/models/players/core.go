package players

import (
	"strings"

	"github.com/bbuck/dragon-mud/data/models"
)

// New creates a player with the given DisplayName and Password, doing some
// pregeneratio of BeforeSave operations.
func New(displayName, password string) *models.Player {
	p := &models.Player{
		Username:    strings.ToLower(displayName),
		DisplayName: displayName,
	}
	p.SetPassword(password)

	return p
}

// FindByUsername searches the player database for a player with the given
// username
func FindByUsername(query string) (*models.Player, bool) {
	player := new(models.Player)

	return player, player.ID != ""
}
