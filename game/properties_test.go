package game

import (
	"testing"
	"testing/fstest"
)

// A game seeds rooms with properties of every shape, then a second boot
// from the same database reads them back.
func TestPropertiesSurviveARestart(t *testing.T) {
	files := fstest.MapFS{
		"plugin.lua": file(`return { name = "game" }`),
		"hooks.lua": file(`
			return {
				["dragon:booted"] = function()
					if world.keyed("tavern") then return end
					local tavern = world.create({ key = "tavern", properties = {
						name = "The Dragon's Rest",
						description = "Smoke curls from the hearth.",
					} })
					local hall = world.create({ key = "hall", properties = { name = "A hall" } })
					local cellar = world.create({ key = "cellar", properties = { name = "A cellar" } })
					tavern:set("neighbors", { hall, cellar })
					tavern:set("exits", { north = hall, down = { room = cellar, locked = true } })
				end,
			}
		`),
		"commands.lua": file(`
			return {
				check = { execute = function(actor)
					local tavern = world.keyed("tavern")
					local exits, neighbors = tavern:get("exits"), tavern:get("neighbors")
					actor:send(table.concat({
						tavern:get("description"),
						neighbors[1]:get("name"),
						neighbors[2]:get("name"),
						exits.north:get("name"),
						exits.down.room:get("name"),
						tostring(exits.down.locked),
						tostring(exits.north == world.keyed("hall")),
					}, " | "))
				end },
			}
		`),
	}

	db := openStore(t)
	want := "Smoke curls from the hearth. | A hall | A cellar | A hall | A cellar | true | true"

	g, err := newGameWith(t, db, files)
	if err != nil {
		t.Fatal(err)
	}
	stop := runGame(t, g)
	alice := connect(t, g)
	alice.login("Alice")
	alice.send("check")
	alice.expect(want)
	alice.send("quit")
	alice.expect("Farewell")
	stop()

	g2, err := newGameWith(t, db, files)
	if err != nil {
		t.Fatal(err)
	}
	runGame(t, g2)
	again := connect(t, g2)
	again.relogin("Alice", " secret pass ")
	again.expect("Welcome")
	again.send("check")
	again.expect(want)
}
