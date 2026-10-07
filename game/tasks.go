package game

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"strings"

	"bbuck.dev/dragon-mud/command"
	"bbuck.dev/dragon-mud/plugin"
	"bbuck.dev/dragon-mud/scripting"
)

// ErrNoTask is returned by RunTask for a task no plugin has.
var ErrNoTask = errors.New("there's no task")

// TaskFailed is returned by RunTask when a task calls task:fail: it can't
// succeed, for a reason it gives, as opposed to a bug raising an error.
type TaskFailed struct {
	Task    string
	Message string

	// Code is the exit status dragon ends with, 1 to 125.
	Code int
}

func (f *TaskFailed) Error() string {
	return f.Message
}

// taskEvent asks the loop to run a task and the tasks it depends on.
type taskEvent struct {
	ctx  context.Context
	name string
	args []string
	out  TaskOutput
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
			return fmt.Errorf("%s: %s and %s both have a task called %s. Task names are as written, so rename one, or put it in a namespace of its plugin's own, like %s:%s.",
				def.Path, other.Plugin, def.Plugin, def.Name, p.Manifest.Name, def.Name)
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

// TaskOutput is where a task's lines go: Print for its results, which
// dragon prints on stdout, and Warn for problems and progress, on stderr.
type TaskOutput struct {
	Print func(line string)
	Warn  func(line string)
}

// RunTask runs the task name, after the tasks it depends on, on the game
// loop, which must be running. args are the words after the task's name
// on the command line. Each task runs until it returns or ctx ends, with
// no deadline of its own, and what the tasks change is saved together
// once they've all run.
func (g *Game) RunTask(ctx context.Context, name string, args []string, out TaskOutput) error {
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
			return fmt.Errorf("%w called %s, and no plugin the game loads has any tasks.", ErrNoTask, e.name)
		}
		return fmt.Errorf("%w called %s.%s Run dragon tasks to list them.", ErrNoTask, e.name, command.DidYouMean(e.name, names))
	}

	order, err := g.taskOrder(e.name)
	if err != nil {
		return err
	}

	for _, t := range order {
		run := &taskRun{name: t.Name, out: e.out}
		if t.Name == e.name {
			run.args = e.args
		}
		_, err := t.Execute.Call(e.ctx, scripting.Handle{Type: g.taskType, Key: run})
		// A task that called fail has failed, even if it caught the
		// error fail raised.
		if run.failed != nil {
			return run.failed
		}
		if err != nil {
			return fmt.Errorf("task %s failed: %w", t.Name, err)
		}
	}

	return nil
}

// taskRun is one task running, which its task object refers to.
type taskRun struct {
	name string
	args []string
	out  TaskOutput

	// failed is set once the task calls fail.
	failed *TaskFailed
}

// makeTaskType is the object a task's execute gets.
//
//	task.args             the words after the task's name on the command
//	                      line; none for a task run as a prerequisite
//	task.name             the task's full name
//	task:print(...)       a line on stdout: the task's results
//	task:warn(...)        a line on stderr: problems and progress
//	task:fail(message[, code])
//	                      stop: the task can't succeed. dragon prints
//	                      message on stderr and exits with code, 1 by
//	                      default
func (g *Game) makeTaskType() *scripting.Type {
	run := func(key any) *taskRun { return key.(*taskRun) }
	line := func(args scripting.Args) string {
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
		return strings.Join(parts, " ")
	}

	return &scripting.Type{
		Name: "task",
		Fields: map[string]scripting.Field{
			"args": func(key any) (any, error) {
				words := []any{}
				for _, w := range run(key).args {
					words = append(words, w)
				}
				return words, nil
			},
			"name": func(key any) (any, error) { return run(key).name, nil },
		},
		Methods: map[string]scripting.Method{
			"print": func(key any, args scripting.Args) (any, error) {
				run(key).out.Print(line(args))
				return nil, nil
			},
			"warn": func(key any, args scripting.Args) (any, error) {
				run(key).out.Warn(line(args))
				return nil, nil
			},
			"fail": func(key any, args scripting.Args) (any, error) {
				message, err := args.String(0)
				if err != nil {
					return nil, fmt.Errorf("%w; task:fail takes the reason the task can't succeed, like task:fail(\"there's no area called riverside\")", err)
				}
				code := 1
				if args.Len() > 1 && args[1] != nil {
					if code, err = args.Int(1); err != nil {
						return nil, err
					}
					if code < 1 || code > 125 {
						return nil, fmt.Errorf("task:fail's exit code must be 1 to 125, not %d: 0 means success, and shells give higher codes their own meanings", code)
					}
				}
				r := run(key)
				r.failed = &TaskFailed{Task: r.name, Message: message, Code: code}
				return nil, errors.New("task:fail: " + message)
			},
		},
		String: func(key any) string { return "task " + run(key).name },
	}
}
