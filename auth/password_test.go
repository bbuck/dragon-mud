package auth

import (
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestHashAndCheck(t *testing.T) {
	hash, err := HashPassword("hunter2", bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}

	if ok, err := CheckPassword("hunter2", hash); err != nil || !ok {
		t.Errorf("CheckPassword(correct) = %v, %v; want true, nil", ok, err)
	}

	if ok, err := CheckPassword("wrong", hash); err != nil || ok {
		t.Errorf("CheckPassword(wrong) = %v, %v; want false, nil", ok, err)
	}
}

func TestCheckInvalidHash(t *testing.T) {
	if _, err := CheckPassword("hunter2", "not a hash"); err == nil {
		t.Error("expected an error for an invalid hash")
	}
}

func TestHashRejectsBadCost(t *testing.T) {
	for _, cost := range []int{bcrypt.MinCost - 1, bcrypt.MaxCost + 1} {
		if _, err := HashPassword("hunter2", cost); err == nil {
			t.Errorf("HashPassword with cost %d: expected error", cost)
		}
	}
}
