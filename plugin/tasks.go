package plugin

import (
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"bbuck.dev/dragon-mud/scripting"
)

// taskPartRx matches one part of a task's name: its own name, or one
// part of a namespace.
var taskPartRx = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)

// taskRefRx matches a task's full name, as depends names it.
var taskRefRx = regexp.MustCompile(`^[a-z][a-z0-9_-]*(:[a-z][a-z0-9_-]*)*$`)

// taskKeys are the fields a task may have, and groupKeys those of a group
// of tasks under a namespace.
var (
	taskKeys  = []string{"name", "desc", "depends", "execute"}
	groupKeys = []string{"namespace", "tasks"}
)

// TaskDef is a task as a plugin declares it: something run from the
// command line with dragon <task>, outside the running game.
type TaskDef struct {
	// Name is the task's full name, its namespace and its own name joined
	// with colons, such as mapping:rebuild, or just its name, such as
	// seed, for a task with no namespace.
	Name string

	// Namespace is the part of Name before the task's own name, or "".
	Namespace string

	Plugin string
	Desc   string

	// Path is where the plugin exports it, such as tasks.seed or
	// tasks[1].tasks.rebuild.
	Path string

	// Depends are the full names of the tasks that run before this one,
	// in order.
	Depends []string

	// Execute is called as execute(task), with the task object: its
	// args, and print and warn for its output.
	Execute scripting.Function
}

// Namespace is the namespace the engine puts in front of names it makes
// up for the plugin: its manifest name, such as mapping for the client
// event mapping:pan and the added field mapping.coords, or game for the
// game's own. Built-ins use their name without dragon:.
func (p *Plugin) Namespace() string {
	return p.Manifest.Name
}

// Tasks loads the tasks the plugin exports as tasks, sorted by name.
// Names are used as written: a plugin puts its tasks in a namespace with
// a group, like rake's, and groups nest. A task is a function, a table
// keyed by its name, or a table in a list with its name inside.
//
//	tasks = {
//	  seed = function(task) ... end,             -- dragon seed
//	  {
//	    namespace = "mapping",
//	    tasks = {
//	      clear = function(task) ... end,         -- dragon mapping:clear
//	      {
//	        name = "rebuild",                          -- dragon mapping:rebuild
//	        desc = "Redraw every map.",
//	        depends = { "mapping:clear" },
//	        execute = function(task) ... end,
//	      },
//	      { namespace = { "areas", "river" }, tasks = { ... } }, -- mapping:areas:river:...
//	    },
//	  },
//	}
func (p *Plugin) Tasks() ([]TaskDef, error) {
	raw, ok := p.exports["tasks"]
	if !ok || raw == nil {
		return nil, nil
	}
	if err := p.need("tasks", CapTasks, "exporting tasks"); err != nil {
		return nil, err
	}

	var defs []TaskDef
	if err := p.taskGroup("tasks", nil, raw, &defs); err != nil {
		return nil, err
	}
	seen := make(map[string]string)
	for _, def := range defs {
		if other, ok := seen[def.Name]; ok {
			return nil, fmt.Errorf("%s and %s are both the task %s. Rename one of them, or put it in another namespace.", other, def.Path, def.Name)
		}
		seen[def.Name] = def.Path
	}
	slices.SortFunc(defs, func(a, b TaskDef) int { return strings.Compare(a.Name, b.Name) })

	return defs, nil
}

// taskGroup reads the tasks in a group's tasks table, at where, under the
// namespace parts.
func (p *Plugin) taskGroup(where string, namespace []string, raw any, defs *[]TaskDef) error {
	const shape = `tasks = { seed = function(task) ... end, { namespace = "mapping", tasks = { ... } } }`

	var entries map[string]any
	switch v := raw.(type) {
	case map[string]any:
		entries = v
	case []any:
		entries = make(map[string]any, len(v))
		for i, item := range v {
			entries[fmt.Sprint(i+1)] = item
		}
	default:
		return fmt.Errorf("%s must be a table of tasks, like %s, not a %s.", where, shape, scripting.TypeName(raw))
	}

	for _, key := range slices.Sorted(maps.Keys(entries)) {
		value := entries[key]
		if _, err := strconv.Atoi(key); err == nil {
			// A list entry: a group, or a task with its name inside.
			at := fmt.Sprintf("%s[%s]", where, key)
			entry, ok := value.(map[string]any)
			if !ok {
				return fmt.Errorf("%s must be a task like { name = \"seed\", execute = function(task) ... end }, or a group like { namespace = \"mapping\", tasks = { ... } }, not a %s.", at, scripting.TypeName(value))
			}
			if _, isGroup := entry["namespace"]; isGroup {
				if err := p.taskNamespace(at, namespace, entry, defs); err != nil {
					return err
				}
				continue
			}
			name, ok := entry["name"].(string)
			if !ok {
				return fmt.Errorf("%s needs a name, like name = \"seed\", or a namespace and tasks if it's a group.", at)
			}
			if err := p.task(at, namespace, name, entry, defs); err != nil {
				return err
			}
			continue
		}

		at := field(where, key)
		switch v := value.(type) {
		case scripting.Function:
			if err := p.task(at, namespace, key, map[string]any{"execute": v}, defs); err != nil {
				return err
			}
		case map[string]any:
			if _, named := v["name"]; named {
				return fmt.Errorf("%s: a task keyed by its name doesn't set name too. Remove name, or put the task in the list part of tasks.", at)
			}
			if err := p.task(at, namespace, key, v, defs); err != nil {
				return err
			}
		default:
			return fmt.Errorf("%s must be a function(task), or a table like { desc = \"...\", execute = function(task) ... end }, not a %s.", at, scripting.TypeName(value))
		}
	}

	return nil
}

// taskNamespace reads a group: a namespace, a name or a list of names,
// and the tasks under it.
func (p *Plugin) taskNamespace(where string, outer []string, entry map[string]any, defs *[]TaskDef) error {
	if err := checkKeys(where, entry, groupKeys, nil); err != nil {
		return err
	}

	var parts []string
	switch v := entry["namespace"].(type) {
	case string:
		parts = []string{v}
	case []any:
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				return fmt.Errorf("%s: namespace lists names, like namespace = { \"mapping\", \"areas\" }, not a %s.", where, scripting.TypeName(item))
			}
			parts = append(parts, s)
		}
	default:
		return fmt.Errorf("%s: namespace must be a name, like namespace = \"mapping\", or a list of them, not a %s.", where, scripting.TypeName(entry["namespace"]))
	}
	if len(parts) == 0 {
		return fmt.Errorf("%s: namespace is empty. Name it, like namespace = \"mapping\", or move its tasks up a level.", where)
	}
	for _, part := range parts {
		if !taskPartRx.MatchString(part) {
			return fmt.Errorf("%s: %q isn't a valid namespace. Namespaces are lowercase letters, digits, - and _, starting with a letter, like mapping; nest them with a list, like { \"mapping\", \"areas\" }.", where, part)
		}
	}
	namespace := append(slices.Clone(outer), parts...)
	if namespace[0]+":" == BuiltinPrefix && !strings.HasPrefix(p.ID, BuiltinPrefix) {
		return fmt.Errorf("%s uses the %q namespace, which is reserved for the engine's built-in plugins. Use your plugin's name instead, like namespace = %q.", where, strings.TrimSuffix(BuiltinPrefix, ":"), p.Manifest.Name)
	}
	if entry["tasks"] == nil {
		return fmt.Errorf("%s has a namespace but no tasks. Add tasks = { ... }.", where)
	}

	return p.taskGroup(where+".tasks", namespace, entry["tasks"], defs)
}

// task reads one task called name, under namespace.
func (p *Plugin) task(where string, namespace []string, name string, entry map[string]any, defs *[]TaskDef) error {
	if strings.Contains(name, ":") {
		return fmt.Errorf("%s: a task's name is one word; put it in a namespace with a group instead, like { namespace = %q, tasks = { %s = ... } }.", where, name[:strings.LastIndex(name, ":")], name[strings.LastIndex(name, ":")+1:])
	}
	if !taskPartRx.MatchString(name) {
		return fmt.Errorf("%s: %q isn't a valid task name. Task names are lowercase letters, digits, - and _, starting with a letter, like rebuild or import-areas.", where, name)
	}
	if len(namespace) == 0 && name+":" == BuiltinPrefix {
		return fmt.Errorf("%s: dragon is reserved for the engine's built-in plugins. Name the task something else.", where)
	}
	if err := checkKeys(where, entry, taskKeys, map[string]string{
		"run":  "tasks call execute now: execute = function(task) ... end.",
		"live": "live tasks run inside the running game through the admin API, which isn't built yet. Remove live = true; the task runs in its own copy of the game, while the server isn't running.",
	}); err != nil {
		return err
	}

	def := TaskDef{Name: strings.Join(append(slices.Clone(namespace), name), ":"), Namespace: strings.Join(namespace, ":"), Plugin: p.ID, Path: where}
	fn, ok := entry["execute"].(scripting.Function)
	if !ok {
		return fmt.Errorf("%s needs execute = function(task) ... end, the function the task runs.", where)
	}
	def.Execute = fn

	var err error
	if def.Desc, err = optional[string](where, entry, "desc", "a string"); err != nil {
		return err
	}
	if def.Depends, err = taskDepends(where, entry["depends"]); err != nil {
		return err
	}
	*defs = append(*defs, def)

	return nil
}

// taskDepends reads a task's depends: the full names of the tasks that run
// first, as written.
func taskDepends(where string, raw any) ([]string, error) {
	const shape = `depends = { "mapping:clear", "seed" }`
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

	names := make([]string, len(list))
	for i, v := range list {
		name, ok := v.(string)
		if !ok || !taskRefRx.MatchString(name) {
			return nil, fmt.Errorf("%s: depends #%d isn't a task name. Name each task in full, with its namespace, like \"mapping:clear\".", where, i+1)
		}
		names[i] = name
	}

	return names, nil
}
