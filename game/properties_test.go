package game

import (
	"testing"
	"testing/fstest"
)

// A game seeds rooms with properties of every shape, then a second boot
// from the same database reads them back.
func TestPropertiesSurviveARestart(t *testing.T) {
	files := fstest.MapFS{
		"handlers.lua": file(`
						local world = require("dragon.world")
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
						local world = require("dragon.world")
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

func TestStructuralFieldsArentProperties(t *testing.T) {
	g := startGame(t, fstest.MapFS{
		"commands.lua": file(`
						local world = require("dragon.world")
			return {
				try = { forms = { { "try <what>", function(actor, args)
					local tries = {
						set = function() actor:set("location", world.create({})) end,
						get = function() return actor:get("parent") end,
						create = function() world.create({ properties = { key = "tavern" } }) end,
						delete = function() actor:delete("contents") end,
					}
					local ok, err = pcall(tries[args.what])
					actor:send(args.what .. ": " .. tostring(err))
				end } } },
			}
		`),
	})

	alice := connect(t, g)
	alice.login("Alice")

	for what, want := range map[string]string{
		"set":    `"location" is one of an object's fields, not a property. Read it as o.location, and set it with o:move_to(place) or location = ... in world.create.`,
		"get":    `"parent" is one of an object's fields, not a property. Read it as o.parent`,
		"create": `properties: "key" is one of an object's fields, not a property. Read it as o.key, and set it with o:set_key(key)`,
		"delete": `"contents" is one of an object's fields, not a property. Read it as o.contents`,
	} {
		alice.send("try " + what)
		alice.expect(want)
	}
}

func TestObjectFieldErrors(t *testing.T) {
	g := startGame(t, fstest.MapFS{
		"commands.lua": file(`
			return {
				readname = { execute = function(actor) return actor.name end },
				setname = { execute = function(actor) actor.name = "Bob" end },
				move = { execute = function(actor) actor.location = actor end },
			}
		`),
	})

	alice := connect(t, g)
	alice.login("Alice")

	for input, want := range map[string]string{
		"readname": `object has no field or method "name". Its fields are contents, id, key, location, parent. Its methods are `,
		"setname":  `object.name can't be assigned: object fields are read-only; use its methods. To store name as a property, write o:set("name", value).`,
		"move":     `object.location can't be assigned: object fields are read-only; use its methods. Move it with o:move_to(place).`,
	} {
		alice.send(input)
		alice.expect(want)
	}

	alice.send("readname")
	alice.expect(`If name is a property, read it with o:get("name").`)
}
