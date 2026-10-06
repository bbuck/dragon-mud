package plugin

import (
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"

	"bbuck.dev/dragon-mud/scripting"
)

// taskNameRx matches a task's name as a plugin writes it, without the
// namespace the engine adds.
var taskNameRx = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)

// taskRefRx matches a task named in depends: a task of the same plugin,
// or one of another plugin's after its namespace.
var taskRefRx = regexp.MustCompile(`^([a-z][a-z0-9_-]*:)?[a-z][a-z0-9_-]*$`)

// taskKeys are the fields a task entry may have.
var taskKeys = []string{"desc", "depends", "run"}

// TaskDef is a task as a plugin declares it: something run from the
// command line with dragon <plugin>:<task>, outside the running game.
type TaskDef struct {
	// Name is the task's full name, with the plugin's namespace, such as
	// mapping:rebuild.
	Name string

	Plugin string
	Desc   string

	// Path is where the plugin exports it, such as tasks.rebuild.
	Path string

	// Depends are the full names of the tasks that run before this one,
	// in order.
	Depends []string

	// Run is called as run(args, out): args are the words after the
	// task's name on the command line, and out(text) prints a line.
	Run scripting.Function
}

// TaskNamespace is the namespace the engine puts in front of the plugin's
// task names: its manifest name, such as mapping for mapping:rebuild, or
// game for the game's own tasks. Built-ins use their name without
// dragon:, so the chat plugin's tasks are chat:<task>.
func (p *Plugin) TaskNamespace() string {
	return p.Manifest.Name
}

// Tasks loads the tasks the plugin exports as tasks, sorted by name.
//
//	tasks = {
//	  rebuild = {
//	    desc = "Redraw every map.",
//	    depends = { "clear", "rooms:check" },
//	    run = function(args, out) ... out("Drew 12 maps.") end,
//	  },
//	}
func (p *Plugin) Tasks() ([]TaskDef, error) {
	table, err := p.export("tasks", `tasks = { rebuild = { desc = "...", run = function(args, out) ... end } }`)
	if err != nil || table == nil {
		return nil, err
	}

	var defs []TaskDef
	for _, name := range slices.Sorted(maps.Keys(table)) {
		where := field("tasks", name)
		if strings.Contains(name, ":") {
			return nil, fmt.Errorf("%s: the engine puts %s: in front of the plugin's task names itself, so name it %q.", where, p.TaskNamespace(), name[strings.LastIndex(name, ":")+1:])
		}
		if !taskNameRx.MatchString(name) {
			return nil, fmt.Errorf("%s isn't a valid task name. Task names are lowercase letters, digits, - and _, starting with a letter, like rebuild or import-areas.", where)
		}

		def := TaskDef{Name: p.TaskNamespace() + ":" + name, Plugin: p.ID, Path: where}
		switch v := table[name].(type) {
		case scripting.Function:
			def.Run = v
		case map[string]any:
			if err := checkKeys(where, v, taskKeys, map[string]string{
				"live": "live tasks run inside the running game through the admin API, which isn't built yet. Remove live = true; the task runs offline, against the game's database.",
			}); err != nil {
				return nil, err
			}
			fn, ok := v["run"].(scripting.Function)
			if !ok {
				return nil, fmt.Errorf("%s needs run = function(args, out) ... end, the function the task runs.", where)
			}
			def.Run = fn
			if def.Desc, err = optional[string](where, v, "desc", "a string"); err != nil {
				return nil, err
			}
			if def.Depends, err = p.taskDepends(where, v["depends"]); err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("%s must be a function(args, out), or a table like { desc = \"...\", run = function(args, out) ... end }, not a %s.",
				where, scripting.TypeName(table[name]))
		}

		defs = append(defs, def)
	}

	return defs, nil
}

// taskDepends reads a task's depends: tasks named bare are the plugin's
// own, and the rest carry their plugin's namespace.
func (p *Plugin) taskDepends(where string, raw any) ([]string, error) {
	const shape = `depends = { "clear", "rooms:check" }`
	if raw == nil {
		return nil, nil
	}
	if s, ok := raw.(string); ok {
		return nil, fmt.Errorf("%s: depends must be a list of tasks. Write depends = { %q }.", where, s)
	}
	if m, ok := raw.(map[string]any); ok && len(m) == 0 {
		return nil, nil
	}
	list, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("%s: depends must be a list of the tasks that run first, like %s, not a %s.", where, shape, scripting.TypeName(raw))
	}

	full := make([]string, len(list))
	for i, v := range list {
		name, ok := v.(string)
		if !ok || !taskRefRx.MatchString(name) {
			return nil, fmt.Errorf("%s: depends #%d isn't a task name. Name a task of this plugin, like \"clear\", or another plugin's, like \"rooms:check\".", where, i+1)
		}
		if !strings.Contains(name, ":") {
			name = p.TaskNamespace() + ":" + name
		}
		full[i] = name
	}

	return full, nil
}
