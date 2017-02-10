package random

import (
	"strconv"
	"strings"
)

var validSides = map[string]func() int{
	"2":   D2,
	"4":   D4,
	"6":   D6,
	"8":   D8,
	"10":  D10,
	"12":  D12,
	"20":  D20,
	"100": D100,
}

// D2 represents a 2-sided die
func D2() int { return Range(1, 2) }

// D4 represents a 4-sided die
func D4() int { return Range(1, 4) }

// D6 represents a 6-sided die
func D6() int { return Range(1, 6) }

// D8 represents a 8-sided die
func D8() int { return Range(1, 8) }

// D10 represents a 10-sided die
func D10() int { return Range(1, 10) }

// D12 represents a 12-sided die
func D12() int { return Range(1, 12) }

// D20 represents a 20-sided die
func D20() int { return Range(1, 20) }

// D100 represents a 100-sided die
func D100() int { return Range(1, 100) }

// RollDie takes a string value of die, such as '3d20' and returns an int slice
// with the results.
func RollDie(die string) []int {
	rolls := make([]int, 0)
	values := strings.Split(die, "d")
	if len(values) != 2 {
		return rolls
	}
	count := 1
	if len(values[0]) > 0 {
		var err error
		count, err = strconv.Atoi(values[0])
		if err != nil {
			return rolls
		}
	}
	rollFn, valid := validSides[values[1]]
	if !valid {
		return rolls
	}
	for i := 0; i < count; i++ {
		rolls = append(rolls, rollFn())
	}

	return rolls
}
