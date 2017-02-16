package modules

import (
	"github.com/bbuck/dragon-mud/random"
	"github.com/bbuck/dragon-mud/scripting/engine"
)

// Die is a module mapping that provides simulated die rolling methods to the
// the scripting engine.
//   d2()
//     simulate rolling 1d2
//   d4()
//     simulate rolling 1d4
//   d6()
//     simulate rolling 1d6
//   d8()
//     simulate rolling 1d8
//   d10()
//     simulate rolling 1d10
//   d12()
//     simulate rolling 1d12
//   d20()
//     simulate rolling 1d20
//   d100()
//     simulate rolling 1d100
//   roll(die: string)
//     parse die input and roll the specified number of sided die, for example
//     die.roll("3d8") will simulate rolling 3 8-sided die, and return the values
//     as a table.
var Die = map[string]interface{}{
	"d2":   random.D2,
	"d4":   random.D4,
	"d6":   random.D6,
	"d8":   random.D8,
	"d10":  random.D10,
	"d12":  random.D12,
	"d20":  random.D20,
	"d100": random.D100,
	"roll": func(e *engine.Lua) int {
		str := e.PopString()
		rolls := random.RollDie(str)
		e.PushValue(e.TableFromSlice(rolls))

		return 1
	},
}
