package random

import (
	"slices"
	"testing"
)

func TestRangeIsInclusive(t *testing.T) {
	r := New(1)
	seen := map[int]bool{}

	for range 1000 {
		n := r.Range(1, 6)
		if n < 1 || n > 6 {
			t.Fatalf("Range(1, 6) = %d, out of range", n)
		}
		seen[n] = true
	}

	for i := 1; i <= 6; i++ {
		if !seen[i] {
			t.Errorf("Range(1, 6) never returned %d", i)
		}
	}
}

func TestSameSeedSameRolls(t *testing.T) {
	a, _ := New(42).RollDice("10d20")
	b, _ := New(42).RollDice("10d20")

	if !slices.Equal(a, b) {
		t.Errorf("same seed produced %v and %v", a, b)
	}
}

func TestRollDice(t *testing.T) {
	tests := []struct {
		dice    string
		count   int
		sides   int
		wantErr bool
	}{
		{"d10", 1, 10, false},
		{"2d20", 2, 20, false},
		{"3d7", 3, 7, false},
		{"ad10", 0, 0, true},
		{"a", 0, 0, true},
		{"1da", 0, 0, true},
		{"0d6", 0, 0, true},
		{"1d0", 0, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.dice, func(t *testing.T) {
			rolls, err := New(1).RollDice(tt.dice)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("RollDice(%q) = %v, want error", tt.dice, rolls)
				}
				return
			}
			if err != nil {
				t.Fatalf("RollDice(%q) error: %v", tt.dice, err)
			}
			if len(rolls) != tt.count {
				t.Fatalf("RollDice(%q) returned %d rolls, want %d", tt.dice, len(rolls), tt.count)
			}
			for _, n := range rolls {
				if n < 1 || n > tt.sides {
					t.Errorf("RollDice(%q) rolled %d", tt.dice, n)
				}
			}
		})
	}
}
