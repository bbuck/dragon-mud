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
			plant = { depends = { "clear" }, execute = function(args, out) out("planting") end },
			seed = {
				desc = "Make the starting room.",
				depends = { "clear", "plant" },
				execute = function(args, out)
					world.create({ key = "start", properties = { name = args[1] } })
					out("seeded", args[1], args[2])
				end,
			},
		}
	`))

	lines, err := runTaskLines(t, g, "seed", "Hall", "7")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"clearing 0", "planting", "seeded Hall 7"}; !slices.Equal(lines, want) {
		t.Errorf("printed %q, want %q", lines, want)
	}

	lines, err = runTaskLines(t, g, "clear")
	if err != nil || !slices.Equal(lines, []string{"clearing 0"}) {
		t.Errorf("clear printed %q, %v", lines, err)
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
	if _, err := runTaskLines(t, g, "seed"); err != nil {
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

	if _, err := runTaskLines(t, g, "sede"); err == nil || !strings.Contains(err.Error(), `there's no task called sede. Did you mean "seed"?`) {
		t.Errorf("unknown task: %v", err)
	}
	if _, err := runTaskLines(t, g, "fail"); err == nil || !strings.Contains(err.Error(), "task fail failed") || !strings.Contains(err.Error(), "the map is on fire") {
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
			`return { seed = { depends = { "clera" }, execute = function() end }, clear = function() end }`,
			`tasks.seed depends on clera, but there's no such task. Did you mean "clear"?`,
		},
		{
			"cycle",
			`return { a = { depends = { "b" }, execute = function() end }, b = { depends = { "a" }, execute = function() end } }`,
			`tasks depend on each other in a circle: a → b → a.`,
		},
		{
			"name with a colon",
			`return { ["world:seed"] = function() end }`,
			`tasks["world:seed"]: a task's name is one word; put it in a namespace with a group instead, like { namespace = "world", tasks = { seed = ... } }.`,
		},
		{
			"no run",
			`return { seed = { desc = "Seeds." } }`,
			`tasks.seed needs execute = function(args, out) ... end`,
		},
		{
			"old run key",
			`return { seed = { run = function() end } }`,
			`tasks.seed: tasks call execute now: execute = function(args, out) ... end.`,
		},
		{
			"reserved namespace",
			`return { { namespace = "dragon", tasks = { x = function() end } } }`,
			`tasks[1] uses the "dragon" namespace, which is reserved for the engine's built-in plugins.`,
		},
		{
			"group without tasks",
			`return { { namespace = "world" } }`,
			`tasks[1] has a namespace but no tasks.`,
		},
		{
			"list task without a name",
			`return { { execute = function() end } }`,
			`tasks[1] needs a name, like name = "seed", or a namespace and tasks if it's a group.`,
		},
		{
			"same name twice",
			`return { seed = function() end, { name = "seed", execute = function() end } }`,
			`tasks[1] and tasks.seed are both the task seed.`,
		},
		{
			"live",
			`return { seed = { live = true, execute = function() end } }`,
			`live tasks run inside the running game through the admin API, which isn't built yet.`,
		},
		{
			"depends as a string",
			`return { seed = { depends = "clear", execute = function() end } }`,
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
			return { seed = { desc = "Seeds.", execute = function() end }, clear = function() end }
		`)}),
	})
	if err != nil {
		t.Fatal(err)
	}

	var names []string
	for _, task := range tasks {
		names = append(names, task.Name)
	}
	if want := []string{"clear", "seed"}; !slices.Equal(names, want) {
		t.Errorf("tasks = %q, want %q", names, want)
	}
}

// A plugin's tasks need the tasks capability; the game's own don't.
func TestTasksNeedTheCapability(t *testing.T) {
	files := fstest.MapFS{
		"init.lua":      file(`return { tasks = require("tasks") }`),
		"lua/tasks.lua": file(`return { { namespace = "mapping", tasks = { rebuild = function() end } } }`),
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

// Groups put tasks in namespaces, as written, and nest.
func TestTaskNamespaces(t *testing.T) {
	g := startGame(t, taskFiles(`
		return {
			seed = function(args, out) out("seed") end,
			{
				namespace = "world",
				tasks = {
					reset = function(args, out) out("world:reset") end,
					{
						name = "fill",
						desc = "Fill the world.",
						depends = { "world:reset", "seed" },
						execute = function(args, out) out("world:fill") end,
					},
					{
						namespace = { "areas", "river" },
						tasks = { { name = "flood", execute = function(args, out) out("world:areas:river:flood") end } },
					},
				},
			},
		}
	`))

	lines, err := runTaskLines(t, g, "world:fill")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"world:reset", "seed", "world:fill"}; !slices.Equal(lines, want) {
		t.Errorf("printed %q, want %q", lines, want)
	}
	if lines, err := runTaskLines(t, g, "world:areas:river:flood"); err != nil || !slices.Equal(lines, []string{"world:areas:river:flood"}) {
		t.Errorf("nested: %q, %v", lines, err)
	}
}

// Two plugins' tasks can't share a name.
func TestTaskNamesAcrossPlugins(t *testing.T) {
	files := fstest.MapFS{"init.lua": file(`return { tasks = { seed = function() end } }`)}
	_, err := newGameWithPlugins(t, taskFiles(`return { seed = function() end }`), localPlugin("mapping", `capabilities = ["tasks"]`, files))
	want := `mapping and game both have a task called seed. Task names are as written, so rename one, or put it in a namespace of its plugin's own, like game:seed.`
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Errorf("error = %v\nwant it to contain %q", err, want)
	}
}
