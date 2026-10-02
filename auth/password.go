// Package auth handles player credentials. Passwords are hashed with
// Argon2id and stored in the standard self-describing format, so the
// parameters can be raised later without breaking existing hashes.
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	saltLength = 16
	keyLength  = 32
)

// Params are Argon2id's cost parameters.
type Params struct {
	// Memory is in KiB.
	Memory  uint32
	Time    uint32
	Threads uint8
}

// DefaultParams follow OWASP's baseline recommendation: 19 MiB, two passes,
// one thread.
var DefaultParams = Params{Memory: 19 * 1024, Time: 2, Threads: 1}

func (p Params) validate() error {
	if p.Time < 1 || p.Threads < 1 || p.Memory < 8*uint32(p.Threads) {
		return fmt.Errorf("invalid argon2id parameters %+v", p)
	}

	return nil
}

// HashPassword hashes password with Argon2id and a random salt.
func HashPassword(password string, p Params) (string, error) {
	if err := p.validate(); err != nil {
		return "", err
	}

	salt := make([]byte, saltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}

	key := argon2.IDKey([]byte(password), salt, p.Time, p.Memory, p.Threads, keyLength)
	b64 := base64.RawStdEncoding

	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, p.Memory, p.Time, p.Threads, b64.EncodeToString(salt), b64.EncodeToString(key)), nil
}

// CheckPassword reports whether password matches hash. An error is returned
// only when the hash itself is invalid.
func CheckPassword(password, hash string) (bool, error) {
	p, salt, key, err := decode(hash)
	if err != nil {
		return false, err
	}

	got := argon2.IDKey([]byte(password), salt, p.Time, p.Memory, p.Threads, uint32(len(key)))

	return subtle.ConstantTimeCompare(got, key) == 1, nil
}

// NeedsRehash reports whether hash was made with parameters other than p,
// so it should be replaced the next time the password is known.
func NeedsRehash(hash string, p Params) bool {
	got, _, _, err := decode(hash)

	return err != nil || got != p
}

func decode(hash string) (Params, []byte, []byte, error) {
	invalid := errors.New("invalid password hash")

	parts := strings.Split(hash, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" {
		return Params{}, nil, nil, invalid
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return Params{}, nil, nil, invalid
	}

	var p Params
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.Memory, &p.Time, &p.Threads); err != nil || p.validate() != nil {
		return Params{}, nil, nil, invalid
	}

	b64 := base64.RawStdEncoding
	salt, err := b64.DecodeString(parts[4])
	if err != nil {
		return Params{}, nil, nil, invalid
	}
	key, err := b64.DecodeString(parts[5])
	if err != nil || len(key) == 0 {
		return Params{}, nil, nil, invalid
	}

	return p, salt, key, nil
}
