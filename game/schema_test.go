package game

import (
	"slices"
	"strings"
	"testing"
	"testing/fstest"
)

// schemaGame is a game whose plugin exports lua/schema.lua as its schema
// and lua/tasks.lua as its tasks, which tests use to run code.
func schemaGame(t *testing.T, schema, tasks string) *Game {
	t.Helper()

	return startGame(t, fstest.MapFS{
		"init.lua":       file(`return { schema = require("schema"), tasks = require("tasks") }`),
		"lua/schema.lua": file(schema),
		"lua/tasks.lua":  file(tasks),
	})
}

const itemSchema = `
	return {
		types = {
			["items:item"] = {
				desc = "Something that can be carried.",
				fields = {
					description = { desc = "what players see when they look at it", type = "text" },
					weight = { desc = "how heavy it is, in pounds", type = "number", default = 1 },
				},
			},
			["items:container"] = {
				fields = { capacity = { desc = "how much it holds", type = "integer", default = 10 } },
			},
		},
	}
`

// try runs fn in a task and prints what it returned, or its error.
const tryTasks = `
	local world = require("dragon.world")
	local function try(out, fn)
		local ok, result = pcall(fn)
		if ok then out(tostring(result)) else out("error: " .. tostring(result)) end
	end
	return {
		run = function(args, out)
			local bag = world.create({ types = { "items:item", "items:container" }, properties = { name = "bag" } })
			local loose = world.create({ properties = { anything = "goes" } })
			try(out, function() return bag:get("weight") end)
			try(out, function() return bag:get("capacity") end)
			try(out, function() bag:set("weight", 2.5) return bag:get("weight") end)
			try(out, function() bag:set("descrition", "a sack") end)
			try(out, function() bag:set("weight", "heavy") end)
			try(out, function() return bag:get("colour") end)
			try(out, function() loose:set("colour", "red") return loose:get("colour") end)
			try(out, function() return table.concat(bag.types, ",") end)
			try(out, function() return bag:has_type("items:container") end)
			local small = world.create({ parent = bag })
			try(out, function() return table.concat(small.types, ",") end)
			try(out, function() small:set("capacty", 2) end)
			try(out, function() world.create({ types = { "items:iten" } }) end)
		end,
	}
`

func TestSchemaChecksTypedObjects(t *testing.T) {
	g := schemaGame(t, itemSchema, tryTasks)

	lines, err := runTaskLines(t, g, "run")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"1",
		"10",
		"2.5",
		`"descrition" isn't a field of items:item or items:container. Did you mean "description"?`,
		`items:item's weight is a number field, so it can't hold the string "heavy".`,
		`"colour" isn't a field of items:item or items:container.`,
		"red",
		"items:item,items:container",
		"true",
		"items:item,items:container",
		`"capacty" isn't a field of items:item or items:container. Did you mean "capacity"?`,
		`types: there's no type "items:iten". Did you mean "items:item"?`,
	}
	if len(lines) != len(want) {
		t.Fatalf("printed %q", lines)
	}
	for i := range want {
		if !strings.Contains(lines[i], want[i]) {
			t.Errorf("line %d = %q, want it to contain %q", i+1, lines[i], want[i])
		}
	}
}

func TestChangingTypesChecksProperties(t *testing.T) {
	g := schemaGame(t, itemSchema, `
		local world = require("dragon.world")
		return {
			run = function(args, out)
				local rock = world.create({ properties = { weight = 3, colour = "grey" } })
				local ok, err = pcall(function() rock:add_type("items:item") end)
				out(tostring(ok), tostring(err))
				rock:delete("colour")
				rock:add_type("items:item")
				out(table.concat(rock.types, ","))
				rock:set("description", "A grey rock.")
				rock:remove_type("items:item")
				rock:set("colour", "grey")
				out(rock:get("colour"))
			end,
		}
	`)

	lines, err := runTaskLines(t, g, "run")
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 3 || !strings.Contains(lines[0], `false`) || !strings.Contains(lines[0], `can't add the type items:item: "colour" isn't a field of items:item.`) {
		t.Fatalf("printed %q", lines)
	}
	if lines[1] != "items:item" || lines[2] != "grey" {
		t.Errorf("printed %q", lines)
	}
}

// A plugin adds fields to another plugin's type, named for itself.
func TestSchemaExtensions(t *testing.T) {
	mapping := localPlugin("mapping", "", fstest.MapFS{
		"init.lua": file(`return { schema = { extend = { room = { fields = { coords = { desc = "where it is on the map", type = "table" } } } } } }`),
	})
	g, err := newGameWithPlugins(t, fstest.MapFS{
		"init.lua": file(`return {
			schema = { types = { room = { fields = { description = "what it looks like" } } } },
			tasks = { run = function(args, out)
				local room = require("dragon.world").create({ types = { "room" } })
				room:set("mapping.coords", { x = 1, y = 2 })
				out(room:get("mapping.coords").x)
				local ok, err = pcall(function() room:set("coords", {}) end)
				out(err)
			end },
		}`),
	}, mapping)
	if err != nil {
		t.Fatal(err)
	}
	runGame(t, g)

	lines, err := runTaskLines(t, g, "run")
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 2 || lines[0] != "1" || !strings.Contains(lines[1], `"coords" isn't a field of room. Did you mean "mapping.coords"?`) {
		t.Errorf("printed %q", lines)
	}
}

func TestSchemaDefinitionErrors(t *testing.T) {
	tests := []struct {
		name   string
		schema string
		want   string
	}{
		{
			"unknown kind",
			`return { types = { item = { fields = { weight = { desc = "how heavy", type = "nubmer" } } } } }`,
			`schema.types.item: field weight: type must be one of any, string, text, number, integer, boolean, object, list and table, not nubmer. Did you mean "number"?`,
		},
		{
			"default of the wrong kind",
			`return { types = { item = { fields = { weight = { desc = "how heavy", type = "number", default = "lots" } } } } }`,
			`schema.types.item: field weight: its default is a string, but the field is a number.`,
		},
		{
			"engine field",
			`return { types = { item = { fields = { name = "what it's called" } } } }`,
			`declares name, which every type has already, since the engine reads it.`,
		},
		{
			"no description",
			`return { types = { item = { fields = { weight = { type = "number" } } } } }`,
			`schema.types.item: field weight needs desc, saying what it holds`,
		},
		{
			"reserved namespace",
			`return { types = { ["dragon:item"] = {} } }`,
			`schema.types["dragon:item"] uses the "dragon:" namespace, which is reserved`,
		},
		{
			"extending its own type",
			`return { types = { item = {} }, extend = { item = { fields = { weight = "how heavy" } } } }`,
			`adds fields to item, which the same plugin declares. Add them to its fields instead.`,
		},
		{
			"unknown key",
			`return { type = {} }`,
			`schema has an unknown field "type". Did you mean "types"?`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := newGame(t, fstest.MapFS{
				"init.lua":       file(`return { schema = require("schema") }`),
				"lua/schema.lua": file(tt.schema),
			})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %v\nwant it to contain %q", err, tt.want)
			}
		})
	}
}

// Objects keep types no plugin declares any more, and take any property
// until the type comes back.
func TestUndeclaredTypesAreUnchecked(t *testing.T) {
	db := openStore(t)
	g, err := newGameWith(t, db, fstest.MapFS{
		"init.lua": file(`return {
			schema = { types = { item = { fields = { weight = "how heavy" } } } },
			tasks = { run = function() require("dragon.world").create({ key = "rock", types = { "item" } }) end },
		}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	stop := runGame(t, g)
	if _, err := runTaskLines(t, g, "run"); err != nil {
		t.Fatal(err)
	}
	stop()

	g = startGameWith(t, db, fstest.MapFS{
		"init.lua": file(`return { tasks = { run = function(args, out)
			local rock = require("dragon.world").keyed("rock")
			rock:set("colour", "grey")
			out(table.concat(rock.types, ","), rock:get("colour"))
		end } }`),
	})
	lines, err := runTaskLines(t, g, "run")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(lines, []string{"item grey"}) {
		t.Errorf("printed %q", lines)
	}
}

func TestSafeReads(t *testing.T) {
	g := schemaGame(t, itemSchema, `
		local world = require("dragon.world")
		return {
			run = function(args, out)
				local bag = world.create({ types = { "items:item" } })
				local loose = world.create({})
				out(tostring(bag:try_get("colour")), tostring(loose:try_get("colour")))
				out(bag:get_or("colour", "grey"), loose:get_or("colour", "grey"))
				-- A field's default is its value, so it wins.
				out(bag:get_or("weight", 9))
				out(bag:get_or("description", "plain"))
				out(loose:get_or_set("colour", "red"), loose:get("colour"), loose:get_or_set("colour", "blue"))
				out(bag:get_or_set("description", "A sack."), bag:get("description"))
				local ok, err = pcall(function() bag:get_or_set("colour", "red") end)
				out(tostring(ok), err)
			end,
		}
	`)

	lines, err := runTaskLines(t, g, "run")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"nil nil",
		"grey grey",
		"1",
		"plain",
		"red red red",
		"A sack. A sack.",
		`false`,
	}
	if len(lines) != len(want) {
		t.Fatalf("printed %q", lines)
	}
	for i := range want {
		if !strings.HasPrefix(lines[i], want[i]) {
			t.Errorf("line %d = %q, want %q", i+1, lines[i], want[i])
		}
	}
	if !strings.Contains(lines[6], `"colour" isn't a field of items:item.`) {
		t.Errorf("get_or_set of an undeclared field: %q", lines[6])
	}
}
