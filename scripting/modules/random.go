package modules

import (
	"github.com/bbuck/dragon-mud/random"
	"github.com/bbuck/dragon-mud/scripting/lua"
)

// Random provides a means for generating random numbers up to a maximum value
// or between a minimum and a maximum.
//   gen(max): number
//     @param max: number = the maximum value to generated a number to
//     generate a number from 0 up to the max value given.
//   range(min: number, max: number): number
//     @param min: number = the lower bound (inclusive) of generated random
//       numbers
//     @param max: number = the upper bound (exclusive) of generated random
//       numbers
//     generate a number between the given minimum and maximum, the range
//     [min, max)
var Random = lua.TableMap{
	"gen":   random.Intn,
	"range": random.Range,
}
