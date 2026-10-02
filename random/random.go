// Package random provides seedable random numbers and dice rolling.
//
// The game loop owns a single Rand so that, given the same seed and the same
// commands, a game produces the same results.
package random

import (
	"fmt"
	"math/rand/v2"
	"strconv"
	"strings"
)

// Rand is a source of random numbers for game logic. It is not safe for
// concurrent use; it belongs to the game loop.
type Rand struct {
	rng *rand.Rand
}

// New returns a Rand seeded with seed.
func New(seed uint64) *Rand {
	return &Rand{
		rng: rand.New(rand.NewPCG(seed, seed)),
	}
}

// Range returns a number between min and max, inclusive.
func (r *Rand) Range(min, max int) int {
	return min + r.rng.IntN(max-min+1)
}

// Roll rolls a single die with the given number of sides.
func (r *Rand) Roll(sides int) int {
	return r.Range(1, sides)
}

// RollDice parses dice notation such as "3d6" or "d20" and returns each roll.
func (r *Rand) RollDice(dice string) ([]int, error) {
	countStr, sidesStr, ok := strings.Cut(dice, "d")
	if !ok {
		return nil, fmt.Errorf("invalid dice %q: expected the form NdS", dice)
	}

	count := 1
	if countStr != "" {
		var err error
		count, err = strconv.Atoi(countStr)
		if err != nil || count < 1 {
			return nil, fmt.Errorf("invalid dice %q: bad count", dice)
		}
	}

	sides, err := strconv.Atoi(sidesStr)
	if err != nil || sides < 1 {
		return nil, fmt.Errorf("invalid dice %q: bad side count", dice)
	}

	rolls := make([]int, count)
	for i := range rolls {
		rolls[i] = r.Roll(sides)
	}

	return rolls, nil
}
