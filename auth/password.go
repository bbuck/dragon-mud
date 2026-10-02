// Package auth handles player credentials.
package auth

import (
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// DefaultCost is the bcrypt cost used when the configuration doesn't set one.
const DefaultCost = bcrypt.DefaultCost

// HashPassword hashes password with bcrypt at the given cost.
func HashPassword(password string, cost int) (string, error) {
	if cost < bcrypt.MinCost || cost > bcrypt.MaxCost {
		return "", fmt.Errorf("bcrypt cost %d out of range [%d, %d]", cost, bcrypt.MinCost, bcrypt.MaxCost)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), cost)
	if err != nil {
		return "", err
	}

	return string(hash), nil
}

// CheckPassword reports whether password matches hash. An error is returned
// only when the hash itself is invalid.
func CheckPassword(password, hash string) (bool, error) {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, bcrypt.ErrMismatchedHashAndPassword):
		return false, nil
	default:
		return false, err
	}
}
