package models

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math"
	"math/big"
	"runtime"
	"strings"

	"github.com/bbuck/dragon-mud/random"
	"github.com/pzduniak/argon2"
)

const (
	passwordMemorySize = 4096
	passwordLength     = 32
)

// Player represents an authenticate user. Once the user has connected and
// signed into a player this will likley be populated.
type Player struct {
	BaseModel
	Username    string `json:"username" sql:"index:idx_username;unique;not null"`
	DisplayName string `json:"display_name" sql:"not_null"`
	RawPassword string `json:"-" sql:"-"`
	Password    string `json:"-" sql:"not null"`
	Salt        string `json:"-" sql:"not null"`
	Iterations  uint32 `json:"-" sql:"not null"`
}

// NewPlayer creates a player with the given DisplayName and Password, doing some
// pregeneratio of BeforeSave operations.
func NewPlayer(displayName, password string) *Player {
	p := &Player{
		Username:    strings.ToLower(displayName),
		DisplayName: displayName,
	}

	hash, err := p.hashPassword(password)
	if err != nil {
		p.RawPassword = password
	} else {
		p.Password = hash
	}

	return p
}

// FindPlayerByUsername searches the player database for a player with the given
// username
func FindPlayerByUsername(query string) (*Player, bool) {
	player := new(Player)
	player.DB().Where(&Player{Username: strings.ToLower(query)}).First(&player)

	return player, player.ID != 0
}

// BeforeSave is provided to satisy the BeforeSaver interface.
func (p *Player) BeforeSave() error {
	if len(p.RawPassword) > 0 {
		hash, err := p.hashPassword(p.RawPassword)
		if err != nil {
			return err
		}
		p.Password = hash
		p.RawPassword = ""
	}

	if len(p.Password) == 0 {
		return errors.New("Cannot save player without a password")
	}

	if len(p.DisplayName) == 0 {
		p.DisplayName = strings.Title(p.Username)
	}
	p.Username = strings.ToLower(p.DisplayName)

	return nil
}

// IsValidPassword checks the given string against the users password to see
// if there is a match.
func (p *Player) IsValidPassword(password string) bool {
	hash, err := p.hashPassword(password)
	if err != nil {
		return false
	}

	return p.Password == hash
}

// perform an argon password hash on the given string based on the current
// players hash settings (or generating a salt/iteration count if none is set)
func (p *Player) hashPassword(password string) (string, error) {
	if len(p.Salt) == 0 {
		num, err := rand.Int(rand.Reader, big.NewInt(math.MaxInt64))
		if err != nil {
			return "", err
		}
		p.Salt = fmt.Sprintf("%x", num)
		p.Iterations = uint32(random.Range(3, 8))
	}
	pass := []byte(password)
	salt := []byte(p.Salt)
	hash, err := argon2.Key(pass, salt, p.Iterations, uint32(runtime.NumCPU()), passwordMemorySize, passwordLength, argon2.Argon2i)
	if err != nil {
		return "", err
	}

	return string(hash), nil
}
