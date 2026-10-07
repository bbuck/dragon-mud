package game

import (
	"context"
	"errors"
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
	err := g.RunTask(context.Background(), name, args, TaskOutput{
		Print: func(line string) { lines = append(lines, line) },
		Warn:  func(line string) { lines = append(lines, "warn: "+line) },
	})

	return lines, err
}

func TestTaskRunsAfterItsDependencies(t *testing.T) {
	g := startGame(t, taskFiles(`
		local world = require("dragon.world")
		return {
			clear = function(task) task:print("clearing", #task.args) end,
			plant = { depends = { "clear" }, execute = function(task) task:print("planting") end },
			seed = {
				desc = "Make the starting room.",
				depends = { "clear", "plant" },
				execute = function(task)
					world.create({ key = "start", properties = { name = task.args[1] } })
					task:print("seeded", task.args[1], task.args[2])
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
			`tasks.seed needs execute = function(task) ... end`,
		},
		{
			"old run key",
			`return { seed = { run = function() end } }`,
			`tasks.seed: tasks call execute now: execute = function(task) ... end.`,
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
			seed = function(task) task:print("seed") end,
			{
				namespace = "world",
				tasks = {
					reset = function(task) task:print("world:reset") end,
					{
						name = "fill",
						desc = "Fill the world.",
						depends = { "world:reset", "seed" },
						execute = function(task) task:print("world:fill") end,
					},
					{
						namespace = { "areas", "river" },
						tasks = { { name = "flood", execute = function(task) task:print("world:areas:river:flood") end } },
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

func TestTaskObject(t *testing.T) {
	g := startGame(t, taskFiles(`
		return {
			{
				namespace = "world",
				tasks = {
					check = function(task) task:warn("checking", #task.args) end,
					{
						name = "fill",
						depends = { "world:check" },
						execute = function(task)
							task:print(task.name, task.args[1])
							task:warn("2 rooms have no exits")
						end,
					},
				},
			},
		}
	`))

	lines, err := runTaskLines(t, g, "world:fill", "river")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"warn: checking 0", "world:fill river", "warn: 2 rooms have no exits"}; !slices.Equal(lines, want) {
		t.Errorf("printed %q, want %q", lines, want)
	}
}

func TestTaskFail(t *testing.T) {
	g := startGame(t, taskFiles(`
		return {
			missing = function(task) task:fail("there's no area called " .. task.args[1], 3) end,
			plain = function(task) task:fail("can't") end,
			caught = function(task)
				pcall(function() task:fail("nope", 4) end)
				task:print("carried on")
			end,
			badcode = function(task) task:fail("x", 200) end,
			after = { depends = { "plain" }, execute = function(task) task:print("ran") end },
		}
	`))

	var failed *TaskFailed
	_, err := runTaskLines(t, g, "missing", "riverside")
	if !errors.As(err, &failed) || failed.Message != "there's no area called riverside" || failed.Code != 3 || failed.Task != "missing" {
		t.Errorf("missing: %#v", err)
	}
	if _, err := runTaskLines(t, g, "plain"); !errors.As(err, &failed) || failed.Code != 1 {
		t.Errorf("plain: %#v", err)
	}

	// Catching fail's error doesn't undo it.
	lines, err := runTaskLines(t, g, "caught")
	if !errors.As(err, &failed) || failed.Code != 4 || !slices.Equal(lines, []string{"carried on"}) {
		t.Errorf("caught: %v, %q", err, lines)
	}

	// A prerequisite failing stops what depends on it.
	lines, err = runTaskLines(t, g, "after")
	if !errors.As(err, &failed) || failed.Task != "plain" || len(lines) != 0 {
		t.Errorf("after: %v, %q", err, lines)
	}

	if _, err := runTaskLines(t, g, "badcode"); err == nil || errors.As(err, &failed) || !strings.Contains(err.Error(), "task:fail's exit code must be 1 to 125, not 200") {
		t.Errorf("badcode: %v", err)
	}
}
