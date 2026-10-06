package game

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"strings"

	"bbuck.dev/dragon-mud/command"
	"bbuck.dev/dragon-mud/plugin"
	"bbuck.dev/dragon-mud/scripting"
)

// taskEvent asks the loop to run a task and the tasks it depends on.
type taskEvent struct {
	ctx  context.Context
	name string
	args []string
	out  func(string)
	done chan error
}

// addTasks adds p's tasks to s.
func (s *scripts) addTasks(p *plugin.Plugin) error {
	defs, err := p.Tasks()
	if err != nil {
		return err
	}
	for _, def := range defs {
		if other, ok := s.tasks[def.Name]; ok {
			return fmt.Errorf("%s: %s and %s both have a task called %s, since both plugins are named %s. Rename one of the plugins in its %s.",
				def.Path, other.Plugin, def.Plugin, def.Name, p.Namespace(), plugin.ManifestFile)
		}
		s.tasks[def.Name] = def
	}

	return nil
}

// checkTasks checks that every task's dependencies exist and don't go in
// a circle.
func (s *scripts) checkTasks() error {
	names := slices.Sorted(maps.Keys(s.tasks))
	for _, name := range names {
		t := s.tasks[name]
		for _, dep := range t.Depends {
			if dep == name {
				return fmt.Errorf("%s: %s depends on itself. Remove it from depends.", t.Path, name)
			}
			if _, ok := s.tasks[dep]; !ok {
				return fmt.Errorf("%s depends on %s, but there's no such task.%s Run dragon tasks to list them.", t.Path, dep, command.DidYouMean(dep, names))
			}
		}
	}

	for _, name := range names {
		if _, err := s.taskOrder(name); err != nil {
			return err
		}
	}

	return nil
}

// taskOrder returns the tasks to run for name, each once, prerequisites
// first and in the order depends lists them, ending with name.
func (s *scripts) taskOrder(name string) ([]plugin.TaskDef, error) {
	var order []plugin.TaskDef
	done := make(map[string]bool)
	var path []string

	var visit func(name string) error
	visit = func(name string) error {
		if done[name] {
			return nil
		}
		if i := slices.Index(path, name); i >= 0 {
			cycle := append(slices.Clone(path[i:]), name)
			return fmt.Errorf("tasks depend on each other in a circle: %s. Remove one of those from its depends.", strings.Join(cycle, " → "))
		}
		path = append(path, name)
		t := s.tasks[name]
		for _, dep := range t.Depends {
			if err := visit(dep); err != nil {
				return err
			}
		}
		path = path[:len(path)-1]
		done[name] = true
		order = append(order, t)
		return nil
	}

	return order, visit(name)
}

// Tasks loads the plugins in opts without starting a game and returns
// their tasks, sorted by name, as dragon tasks lists them. Only Name,
// NewEngine, Plugins and Log are used.
func Tasks(ctx context.Context, opts Options) ([]plugin.TaskDef, error) {
	if opts.Log == nil {
		opts.Log = slog.New(slog.DiscardHandler)
	}

	s, err := fromOptions(opts).load(ctx)
	if err != nil {
		return nil, err
	}
	s.engine.Close()

	var tasks []plugin.TaskDef
	for _, name := range slices.Sorted(maps.Keys(s.tasks)) {
		tasks = append(tasks, s.tasks[name])
	}

	return tasks, nil
}

// RunTask runs the task name, after the tasks it depends on, on the game
// loop, which must be running. args are the words after the task's name
// on the command line; out prints what the task says. Each task runs
// until it returns or ctx ends, with no deadline of its own, and what the
// tasks change is saved together once they've all run.
func (g *Game) RunTask(ctx context.Context, name string, args []string, out func(string)) error {
	e := taskEvent{ctx: ctx, name: name, args: args, out: out, done: make(chan error, 1)}
	g.post(e)

	select {
	case err := <-e.done:
		return err
	case <-g.stopped:
		return fmt.Errorf("the game stopped before %s finished", name)
	}
}

// runTask runs a task event's task and its prerequisites.
func (g *Game) runTask(e taskEvent) {
	e.done <- g.runTasks(e)
}

func (g *Game) runTasks(e taskEvent) error {
	if _, ok := g.tasks[e.name]; !ok {
		names := slices.Sorted(maps.Keys(g.tasks))
		if len(names) == 0 {
			return fmt.Errorf("there's no task called %s, and no plugin the game loads has any tasks.", e.name)
		}
		return fmt.Errorf("there's no task called %s.%s Run dragon tasks to list them.", e.name, command.DidYouMean(e.name, names))
	}

	order, err := g.taskOrder(e.name)
	if err != nil {
		return err
	}

	words := make([]any, len(e.args))
	for i, arg := range e.args {
		words[i] = arg
	}
	out := scripting.Func(func(args scripting.Args) (any, error) {
		parts := make([]string, args.Len())
		for i := range parts {
			switch v := args[i].(type) {
			case scripting.Handle:
				parts[i] = v.Describe()
			case nil:
				parts[i] = "nil"
			default:
				parts[i] = fmt.Sprint(v)
			}
		}
		e.out(strings.Join(parts, " "))
		return nil, nil
	})

	for _, t := range order {
		args := []any{}
		if t.Name == e.name {
			args = words
		}
		if _, err := t.Run.Call(e.ctx, args, out); err != nil {
			return fmt.Errorf("task %s failed: %w", t.Name, err)
		}
	}

	return nil
}
