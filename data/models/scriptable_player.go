package models

import "fmt"

// ScriptObject returns a ScriptablePlayer instance that will be passed into in
// game script engines for to allow access to a player object.
func (p *Player) ScriptObject() interface{} {
	return &ScriptablePlayer{p}
}

// ScriptablePlayer is the ScriptObject that will represent a player during in
// game script executions.
type ScriptablePlayer struct {
	player *Player
}

// DisplayName returns the display name of the user.
func (s *ScriptablePlayer) DisplayName() string {
	return fmt.Sprintf("{G}%s{x}", s.player.DisplayName)
}
