package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"text/tabwriter"

	"bbuck.dev/dragon-mud/command"
	"bbuck.dev/dragon-mud/config"
	"bbuck.dev/dragon-mud/game"
	"bbuck.dev/dragon-mud/plugin"
	"bbuck.dev/dragon-mud/scripting"
	"bbuck.dev/dragon-mud/scripting/lua"
	"bbuck.dev/dragon-mud/termlog"
)

// runTasks lists every task the game's plugins provide.
func runTasks(args []string, out io.Writer) error {
	flags := flag.NewFlagSet("tasks", flag.ContinueOnError)
	dir := flags.String("dir", ".", "the game directory")
	if err := flags.Parse(args); err != nil {
		return err
	}

	cfg, err := config.Load(*dir)
	if err != nil {
		return err
	}
	sources, err := pluginSources(*dir, cfg)
	if err != nil {
		return err
	}

	tasks, err := game.Tasks(context.Background(), game.Options{
		Name:      cfg.Name,
		NewEngine: func() scripting.Engine { return lua.New() },
		Plugins:   sources,
	})
	if err != nil {
		return err
	}

	if len(tasks) == 0 {
		fmt.Fprintln(out, `No plugin the game loads has any tasks. A plugin adds them as tasks in its init.lua, like tasks = { seed = function(task) ... end }.`)
		return nil
	}

	// Tasks with no namespace first, then each namespace, alphabetically.
	byNamespace := make(map[string][]plugin.TaskDef)
	for _, t := range tasks {
		byNamespace[t.Namespace] = append(byNamespace[t.Namespace], t)
	}
	namespaces := slices.Sorted(maps.Keys(byNamespace))

	fmt.Fprintln(out, "Tasks:")
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	for _, ns := range namespaces {
		fmt.Fprintln(w)
		if ns != "" {
			fmt.Fprintf(w, "%s\n", ns)
		}
		for _, t := range byNamespace[ns] {
			desc := t.Desc
			if len(t.Depends) > 0 {
				if desc != "" {
					desc += " "
				}
				desc += fmt.Sprintf("(runs %s first)", andList(t.Depends))
			}
			if slices.Contains(commands, t.Name) {
				desc += fmt.Sprintf(" (can't run: dragon %s is a dragon command; put the task in a namespace)", t.Name)
			}
			fmt.Fprintf(w, "  %s\t%s\n", t.Name, strings.TrimSpace(desc))
		}
	}
	w.Flush()

	fmt.Fprintln(out)
	fmt.Fprintln(out, "Run one with dragon <task>, followed by anything the task takes.")

	return nil
}

// runTask runs the task name with the game's database, outside the
// running game. What the task prints goes to out, and its warnings to
// errOut. Flags before the task's own arguments are dragon's;
// everything from the first argument that isn't one is the task's.
func runTask(name string, args []string, out, errOut io.Writer) error {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	dir := flags.String("dir", ".", "the game directory")
	if err := flags.Parse(args); err != nil {
		return err
	}

	if _, err := os.Stat(filepath.Join(*dir, config.FileName)); err != nil {
		return fmt.Errorf("there's no command called %s.%s Tasks are run from a game directory, and %s has no %s. Run dragon help to see the commands.",
			name, command.DidYouMean(name, commands), *dir, config.FileName)
	}
	cfg, err := config.Load(*dir)
	if err != nil {
		return err
	}
	if pid, ok := runningServer(*dir); ok {
		return fmt.Errorf("dragon serve is running this game, as process %d, and a task run from the command line changes the database underneath it: the server wouldn't see the changes, and could save over them. Stop the server, then run %s.", pid, name)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Only problems are worth showing around a task's own output.
	log := slog.New(termlog.New(os.Stderr, &termlog.Options{Level: slog.LevelWarn}))
	g, closeGame, err := openGame(ctx, *dir, cfg, log, nil)
	if err != nil {
		return err
	}
	defer closeGame()

	ctx, cancel := context.WithCancel(ctx)
	ran := make(chan error, 1)
	go func() { ran <- g.Run(ctx) }()

	err = g.RunTask(ctx, name, flags.Args(), game.TaskOutput{
		Print: func(line string) { fmt.Fprintln(out, line) },
		Warn:  func(line string) { fmt.Fprintln(errOut, line) },
	})
	if errors.Is(err, game.ErrNoTask) {
		if hint := command.DidYouMean(name, commands); hint != "" {
			err = fmt.Errorf("%w There's no command called %s either.%s", err, name, hint)
		}
	}
	cancel()
	if runErr := <-ran; err == nil {
		err = runErr
	}

	return err
}

func andList(items []string) string {
	if len(items) < 2 {
		return strings.Join(items, "")
	}

	return strings.Join(items[:len(items)-1], ", ") + " and " + items[len(items)-1]
}
