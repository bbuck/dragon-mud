package game

import (
	"context"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"bbuck.dev/dragon-mud/plugin"
	"bbuck.dev/dragon-mud/scripting"
	"bbuck.dev/dragon-mud/scripting/lua"
)

// taskFiles is a game whose init.lua exports lua/tasks.lua as its tasks.
func taskFiles(tasks string) fstest.MapFS {
	return fstest.MapFS{
		"init.lua":      file(`return { tasks = require("tasks") }`),
		"lua/tasks.lua": file(tasks),
	}
}

func runTaskLines(t *testing.T, g *Game, name string, args ...string) ([]string, error) {
	t.Helper()

	var lines []string
	err := g.RunTask(context.Background(), name, args, func(line string) { lines = append(lines, line) })

	return lines, err
}

func TestTaskRunsAfterItsDependencies(t *testing.T) {
	g := startGame(t, taskFiles(`
		local world = require("dragon.world")
		return {
			clear = function(args, out) out("clearing", #args) end,
			plant = { depends = { "clear" }, run = function(args, out) out("planting") end },
			seed = {
				desc = "Make the starting room.",
				depends = { "clear", "plant" },
				run = function(args, out)
					world.create({ key = "start", properties = { name = args[1] } })
					out("seeded", args[1], args[2])
				end,
			},
		}
	`))

	lines, err := runTaskLines(t, g, "game:seed", "Hall", "7")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"clearing 0", "planting", "seeded Hall 7"}; !slices.Equal(lines, want) {
		t.Errorf("printed %q, want %q", lines, want)
	}

	lines, err = runTaskLines(t, g, "game:clear")
	if err != nil || !slices.Equal(lines, []string{"clearing 0"}) {
		t.Errorf("game:clear printed %q, %v", lines, err)
	}
}

// What a task changes is saved, so the next run of the game sees it.
func TestTaskChangesAreSaved(t *testing.T) {
	db := openStore(t)
	g, err := newGameWith(t, db, taskFiles(`
		local world = require("dragon.world")
		return { seed = function() world.create({ key = "start" }) end }
	`))
	if err != nil {
		t.Fatal(err)
	}
	stop := runGame(t, g)
	if _, err := runTaskLines(t, g, "game:seed"); err != nil {
		t.Fatal(err)
	}
	stop()

	records, err := db.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || records[0].Key != "start" {
		t.Errorf("saved %+v", records)
	}
}

func TestTaskErrors(t *testing.T) {
	g := startGame(t, taskFiles(`
		return {
			fail = function() error("the map is on fire") end,
			seed = function() end,
		}
	`))

	if _, err := runTaskLines(t, g, "game:sede"); err == nil || !strings.Contains(err.Error(), `there's no task called game:sede. Did you mean "game:seed"?`) {
		t.Errorf("unknown task: %v", err)
	}
	if _, err := runTaskLines(t, g, "game:fail"); err == nil || !strings.Contains(err.Error(), "task game:fail failed") || !strings.Contains(err.Error(), "the map is on fire") {
		t.Errorf("failing task: %v", err)
	}
}

func TestTaskDefinitionErrors(t *testing.T) {
	tests := []struct {
		name  string
		tasks string
		want  string
	}{
		{
			"missing dependency",
			`return { seed = { depends = { "clera" }, run = function() end }, clear = function() end }`,
			`tasks.seed depends on game:clera, but there's no such task. Did you mean "game:clear"?`,
		},
		{
			"cycle",
			`return { a = { depends = { "b" }, run = function() end }, b = { depends = { "a" }, run = function() end } }`,
			`tasks depend on each other in a circle: game:a → game:b → game:a.`,
		},
		{
			"namespaced name",
			`return { ["game:seed"] = function() end }`,
			`tasks["game:seed"]: the engine puts game: in front of the plugin's task names itself, so name it "seed".`,
		},
		{
			"no run",
			`return { seed = { desc = "Seeds." } }`,
			`tasks.seed needs run = function(args, out) ... end`,
		},
		{
			"live",
			`return { seed = { live = true, run = function() end } }`,
			`live tasks run inside the running game through the admin API, which isn't built yet.`,
		},
		{
			"depends as a string",
			`return { seed = { depends = "clear", run = function() end } }`,
			`tasks.seed: depends must be a list of tasks. Write depends = { "clear" }.`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := newGame(t, taskFiles(tt.tasks))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %v\nwant it to contain %q", err, tt.want)
			}
		})
	}
}

func TestTasksAreListed(t *testing.T) {
	tasks, err := Tasks(context.Background(), Options{
		NewEngine: func() scripting.Engine { return lua.New() },
		Plugins: append(sources(t, nil), plugin.Source{Origin: "game", Game: true, Files: taskFiles(`
			return { seed = { desc = "Seeds.", run = function() end }, clear = function() end }
		`)}),
	})
	if err != nil {
		t.Fatal(err)
	}

	var names []string
	for _, task := range tasks {
		names = append(names, task.Name)
	}
	if want := []string{"game:clear", "game:seed"}; !slices.Equal(names, want) {
		t.Errorf("tasks = %q, want %q", names, want)
	}
}

// A plugin's tasks need the tasks capability; the game's own don't.
func TestTasksNeedTheCapability(t *testing.T) {
	files := fstest.MapFS{
		"init.lua":      file(`return { tasks = require("tasks") }`),
		"lua/tasks.lua": file(`return { rebuild = function() end }`),
	}

	_, err := newGameWithPlugins(t, nil, localPlugin("mapping", `capabilities = ["sql"]`, files))
	want := `game/plugins/mapping: tasks: exporting tasks needs the tasks capability, which plugin.toml doesn't declare. It declares sql. Add it to plugin.toml:

capabilities = ["sql", "tasks"]`
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Errorf("error = %v\nwant it to contain %q", err, want)
	}

	g, err := newGameWithPlugins(t, nil, localPlugin("mapping", `capabilities = ["tasks"]`, files))
	if err != nil {
		t.Fatal(err)
	}
	runGame(t, g)
	if _, err := runTaskLines(t, g, "mapping:rebuild"); err != nil {
		t.Error(err)
	}
}
