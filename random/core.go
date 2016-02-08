package random

import (
	"math/rand"
	"time"
)

var (
	source    = rand.NewSource(time.Now().UnixNano())
	generator = rand.New(source)
)

// Intn wraps rand.Intn
func Intn(max int) int {
	return generator.Intn(max)
}

// Range generates a number between the min and max values provided.
func Range(min, max int) int {
	value := generator.Intn(max - min)

	return value + min
}

// SetSource is used exclusively for testing, it should never be used outside
// of an _test file. This will allow setting a known generator with a predicatble
// source of random numbers for test prediction.
func SetSource(source rand.Source) {
	generator = rand.New(source)
}
