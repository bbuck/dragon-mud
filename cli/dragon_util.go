package cli

import "github.com/bbuck/dragon-mud/random"

type dragonDetails struct {
	name, color string
}

var dragonColors = []dragonDetails{
	{name: "[l][-W]black[x]", color: "[l][-W]"},
	{name: "[c220]brass[x]", color: "[c220]"},
	{name: "[R]red[x]", color: "[R]"},
	{name: "[c208]bronze[x]", color: "[c208]"},
	{name: "[G]green[x]", color: "[G]"},
	{name: "[Y]gold[x]", color: "[Y]"},
	{name: "[B]blue[x]", color: "[B]"},
	{name: "[c202]copper[x]", color: "[c202]"},
	{name: "[W]white[x]", color: "[W]"},
	{name: "[c250]{u}silver[x]", color: "[c250][u]"},
}

func getRandomDragonDetails() dragonDetails {
	index := random.Intn(len(dragonColors))

	return dragonColors[index]
}
