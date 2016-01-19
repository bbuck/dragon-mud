package random

import (
	"math/rand"
	"time"
)

var (
	source    = rand.NewSource(time.Now().UnixNano())
	generator = rand.New(source)
)

// Int wraps rand.Intn
func Int(max int) int {
	return generator.Intn(max)
}

// Range generates a number between the min and max values provided.
func Range(min, max int) int {
	value := generator.Intn(max - min)

	return value + min
}
