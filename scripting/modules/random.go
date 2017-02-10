package modules

import "github.com/bbuck/dragon-mud/random"

// Random provides a means for generating random numbers up to a maximum value
// or between a minimum and a maximum.
//   gen(max: number): number
//     generate a number from 0 up to the max value given.
//   range(min: number, max: number): number
//     generate a number between the given minimum and maximum, the range
//     [min, max)
var Random = map[string]interface{}{
	"gen":   random.Intn,
	"range": random.Range,
}
