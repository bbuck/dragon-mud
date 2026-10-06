// Command dragon creates and runs DragonMUD games.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	"bbuck.dev/dragon-mud/ansi"
	"bbuck.dev/dragon-mud/builtin"
	"bbuck.dev/dragon-mud/command"
	"bbuck.dev/dragon-mud/config"
	"bbuck.dev/dragon-mud/event"
	"bbuck.dev/dragon-mud/game"
	"bbuck.dev/dragon-mud/plugin"
	"bbuck.dev/dragon-mud/random"
	"bbuck.dev/dragon-mud/scaffold"
	"bbuck.dev/dragon-mud/scripting"
	"bbuck.dev/dragon-mud/scripting/lua"
	"bbuck.dev/dragon-mud/store"
	"bbuck.dev/dragon-mud/termlog"
	"bbuck.dev/dragon-mud/transport/telnet"
	"bbuck.dev/dragon-mud/transport/web"
	"bbuck.dev/dragon-mud/watch"
	"bbuck.dev/dragon-mud/world"
)

// reloadInterval is how often the game directory is checked for changed
// scripts.
const reloadInterval = 500 * time.Millisecond

const version = "0.1.0-dev"

const usage = `dragon creates and runs DragonMUD games.

Usage:
  dragon new <directory> [-name "Game Name"]   create a new game
  dragon serve [-dir <directory>]              run the game in a directory
  dragon events [<name>] [-dir <directory>]    list events, or show one's fields
                                               and the order its handlers run in
  dragon tasks [-dir <directory>]              list the tasks plugins provide
  dragon test [-dir <directory>] [-run <regexp>]
                                               run the game's tests
  dragon <plugin>:<task> [-dir <directory>] [args...]
                                               run a task, after the tasks it
                                               depends on
  dragon version                               print the engine version
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}

	var err error
	switch os.Args[1] {
	case "new":
		err = runNew(os.Args[2:])
	case "serve":
		err = runServe(os.Args[2:])
	case "events":
		err = runEvents(os.Args[2:], os.Stdout)
	case "version":
		fmt.Println("dragon", version)
	case "help", "-h", "--help":
		fmt.Print(usage)
	case "tasks":
		err = runTasks(os.Args[2:], os.Stdout)
	case "test":
		err = runTest(os.Args[2:], os.Stdout)
	default:
		if strings.Contains(os.Args[1], ":") {
			err = runTask(os.Args[1], os.Args[2:], os.Stdout)
			break
		}
		fmt.Fprintf(os.Stderr, "dragon: unknown command %q\n\n%s", os.Args[1], usage)
		os.Exit(2)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, "dragon:", err)
		os.Exit(1)
	}
}

func runNew(args []string) error {
	flags := flag.NewFlagSet("new", flag.ContinueOnError)
	name := flags.String("name", "", "the game's name (defaults to the directory name)")

	// Allow the directory before or after flags.
	var dir string
	if len(args) > 0 && args[0] != "" && args[0][0] != '-' {
		dir, args = args[0], args[1:]
	}
	if err := flags.Parse(args); err != nil {
		return err
	}
	if dir == "" {
		dir = flags.Arg(0)
	}
	if dir == "" {
		return errors.New("usage: dragon new <directory>")
	}

	if *name == "" {
		*name = filepath.Base(dir)
	}

	if err := scaffold.New(dir, scaffold.Data{Name: *name}); err != nil {
		return err
	}

	fmt.Printf("Created %s in %s\n\n  cd %s\n  dragon serve\n\n", *name, dir, dir)

	return nil
}

func runServe(args []string) error {
	flags := flag.NewFlagSet("serve", flag.ContinueOnError)
	dir := flags.String("dir", ".", "the game directory")
	if err := flags.Parse(args); err != nil {
		return err
	}

	cfg, err := config.Load(*dir)
	if err != nil {
		return err
	}

	logs, closeLogs, err := openLogs(*dir, cfg.Log)
	if err != nil {
		return err
	}
	defer closeLogs()
	activity := newActivity(logs)
	log := slog.New(activity)

	var d *dragon
	if cfg.Dragon {
		d = summon(random.New(uint64(time.Now().UnixNano())), os.Stderr, termlog.UseColor(os.Stderr))
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	g, closeGame, err := openGame(ctx, *dir, cfg, log, func(objects int) { d.greet(cfg.Name, objects) })
	if err != nil {
		return err
	}
	defer closeGame()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var tasks []func() error
	tasks = append(tasks, func() error { return g.Run(ctx) })
	tasks = append(tasks, func() error { return d.keepWatch(ctx, activity, idleAfter) })
	if gameDir := filepath.Join(*dir, "game"); isDir(gameDir) {
		tasks = append(tasks, func() error {
			return watch.Poll(ctx, os.DirFS(gameDir), []string{"*.lua", "*.tmpl", plugin.ManifestFile}, reloadInterval, func() {
				log.Info("scripts changed; reloading", termlog.PrefixKey, "watch")
				d.reloaded()
				g.Reload()
			})
		})
	}
	if cfg.Telnet.Enabled {
		tasks = append(tasks, func() error {
			return telnet.Serve(ctx, telnet.Options{Address: cfg.Telnet.Address, Wrap: cfg.Telnet.Wrap}, g, log.With(termlog.PrefixKey, "telnet"))
		})
	}
	if cfg.Web.Client.Enabled {
		tasks = append(tasks, func() error {
			return web.Serve(ctx, web.Options{Address: cfg.Web.Address, GameName: cfg.Name, Plugins: g.Web}, g, log.With(termlog.PrefixKey, "web"))
		})
	}

	if err := runAll(cancel, tasks); err != nil {
		return err
	}
	d.farewell()

	return nil
}

// openGame opens the game in dir with its database and world, ready to
// run. loaded, if not nil, is told how many objects the world has once
// it's loaded. close closes the database.
func openGame(ctx context.Context, dir string, cfg config.Config, log *slog.Logger, loaded func(objects int)) (g *game.Game, close func() error, err error) {
	sources, err := pluginSources(dir, cfg.Builtins)
	if err != nil {
		return nil, nil, err
	}

	db, err := store.Open(ctx, filepath.Join(dir, "data", "world.db"))
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		if err != nil {
			db.Close()
		}
	}()

	records, err := db.Load(ctx)
	if err != nil {
		return nil, nil, err
	}
	w, err := world.Load(records)
	if err != nil {
		return nil, nil, fmt.Errorf("loading the world: %w", err)
	}
	log.Info("loaded world", termlog.PrefixKey, "store", "objects", w.Len())
	if loaded != nil {
		loaded(w.Len())
	}

	g, err = game.New(ctx, game.Options{
		Name:      cfg.Name,
		NewEngine: func() scripting.Engine { return lua.New() },
		Plugins:   sources,
		TextWidth: cfg.TextWidth(),
		World:     w,
		Store:     db,
		Log:       log.With(termlog.PrefixKey, "game"),
	})
	if err != nil {
		return nil, nil, err
	}

	return g, db.Close, nil
}

func runEvents(args []string, out io.Writer) error {
	flags := flag.NewFlagSet("events", flag.ContinueOnError)
	dir := flags.String("dir", ".", "the game directory")

	// Allow the event name before or after flags.
	var name string
	if len(args) > 0 && args[0] != "" && args[0][0] != '-' {
		name, args = args[0], args[1:]
	}
	if err := flags.Parse(args); err != nil {
		return err
	}
	if name == "" {
		name = flags.Arg(0)
	}

	cfg, err := config.Load(*dir)
	if err != nil {
		return err
	}
	sources, err := pluginSources(*dir, cfg.Builtins)
	if err != nil {
		return err
	}

	events, err := game.Events(context.Background(), game.Options{
		Name:      cfg.Name,
		NewEngine: func() scripting.Engine { return lua.New() },
		Plugins:   sources,
	})
	if err != nil {
		return err
	}

	if name == "" {
		return listEvents(out, events)
	}

	return showEvent(out, events, name)
}

// eventNames returns every event that's declared or has handlers, sorted.
func eventNames(events *event.Registry) []string {
	names := append(events.Declared(), events.Names()...)
	slices.Sort(names)

	return slices.Compact(names)
}

// listEvents prints every event and who handles it.
func listEvents(out io.Writer, events *event.Registry) error {
	names := eventNames(events)
	if len(names) == 0 {
		fmt.Fprintln(out, "No plugin declares or handles any events yet. Declare the ones a plugin sends in its events.declare, and add handlers in events.handlers.")
		return nil
	}

	fmt.Fprintln(out, "Events, with their handlers in the order they run:")
	fmt.Fprintln(out)

	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	for _, name := range names {
		c, _ := events.Chain(name)
		plugins := make([]string, len(c.Handlers))
		for i, h := range c.Handlers {
			plugins[i] = h.Plugin
		}
		line := strings.Join(plugins, ", ")
		if line == "" {
			line = "(no handlers)"
		}
		if len(c.Disabled) > 0 {
			line += fmt.Sprintf(" (%d disabled)", len(c.Disabled))
		}
		if _, ok := events.Decl(name); !ok {
			line += " (not declared, so never run)"
		}
		fmt.Fprintf(w, "  %s\t%s\n", name, line)
	}
	w.Flush()

	fmt.Fprintln(out)
	fmt.Fprintln(out, "Run dragon events <name> to see an event's fields, and why its handlers run in that order.")

	return nil
}

// showEvent prints one event's declaration, and its handlers in order and
// why they're in it.
func showEvent(out io.Writer, events *event.Registry, name string) error {
	d, declared := events.Decl(name)
	c, handled := events.Chain(name)
	if !declared && !handled {
		names := eventNames(events)
		if len(names) == 0 {
			return fmt.Errorf("no plugin declares or handles %q; no plugin declares or handles any events yet", name)
		}
		return fmt.Errorf("no plugin declares or handles %q.%s Events: %s.",
			name, command.DidYouMean(name, names), strings.Join(names, ", "))
	}

	if declared {
		where := "by the engine"
		if d.Plugin != "" {
			where = "by " + d.Plugin
		}
		fmt.Fprintf(out, "%s, declared %s.\n", name, where)
		if d.Desc != "" {
			fmt.Fprintln(out, ansi.Wrap("  "+d.Desc, 78))
		}

		fmt.Fprintln(out)
		if len(d.Fields) == 0 && d.Extra == "" {
			fmt.Fprintln(out, "Its event has no fields.")
		} else {
			fmt.Fprintln(out, "Fields:")
			w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
			for _, f := range d.Fields {
				desc := f.Desc
				if f.Optional {
					desc += " (optional)"
				}
				fmt.Fprintf(w, "  %s\t%s\n", f.Name, desc)
			}
			if d.Extra != "" {
				fmt.Fprintf(w, "  anything else\t%s\n", d.Extra)
			}
			w.Flush()
		}
	} else {
		fmt.Fprintf(out, "No plugin declares %s, so nothing runs it and its handlers never run. It may be misspelled, or from a plugin the game doesn't load.%s\n",
			name, command.DidYouMean(name, events.Declared()))
	}

	fmt.Fprintln(out)
	redirected := events.Redirected(name)
	if len(redirected) > 0 {
		fmt.Fprintf(out, "Redirected in %s:\n", event.WiringWhere)
		w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
		for _, h := range redirected {
			fmt.Fprintf(w, "  %s\truns on %s instead\n", h.Plugin, h.Event)
		}
		w.Flush()
		fmt.Fprintln(out)
	}
	if !handled {
		if len(redirected) > 0 {
			fmt.Fprintln(out, "No other plugin handles it.")
			return nil
		}
		fmt.Fprintln(out, "No plugin handles it yet. Add a handler in a plugin's events.handlers.")
		return nil
	}

	fmt.Fprintln(out, "Handlers, in the order they run:")
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	for i, h := range c.Handlers {
		why := events.Explain(c, h)
		if why == "" && !c.Wired {
			why = "load order"
		}
		fmt.Fprintf(w, "  %d.\t%s\t%s\t%s\n", i+1, h.Plugin, h.Where(), why)
	}
	w.Flush()

	if c.Wired {
		fmt.Fprintf(out, "\nThe order is set in %s.\n", event.WiringWhere)
	}
	if len(c.Disabled) > 0 {
		plugins := make([]string, len(c.Disabled))
		for i, h := range c.Disabled {
			plugins[i] = h.Plugin
		}
		fmt.Fprintf(out, "\nDisabled in %s: %s\n", event.WiringWhere, strings.Join(plugins, ", "))
	}

	return nil
}

// pluginSources lists the built-in plugins the game loads, in the engine's
// order, then the game's local plugins in game/plugins by directory name,
// then the game's own plugin.
func pluginSources(dir string, builtins []string) ([]plugin.Source, error) {
	var sources []plugin.Source
	for _, name := range builtin.Names {
		if !slices.Contains(builtins, name) {
			continue
		}
		files, err := builtin.FS(name)
		if err != nil {
			return nil, err
		}
		sources = append(sources, plugin.Source{
			Origin:  "built-in plugin " + name,
			Files:   files,
			Builtin: true,
		})
	}

	gameDir := filepath.Join(dir, "game")
	localDir := filepath.Join(gameDir, plugin.LocalDir)
	if isDir(localDir) {
		entries, err := os.ReadDir(localDir)
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
				continue
			}
			pluginDir := filepath.Join(localDir, e.Name())
			sources = append(sources, plugin.Source{Origin: pluginDir, Files: os.DirFS(pluginDir)})
		}
	}

	if isDir(gameDir) {
		sources = append(sources, plugin.Source{Origin: gameDir, Files: os.DirFS(gameDir), Game: true})
	}

	return sources, nil
}

func isDir(path string) bool {
	info, err := os.Stat(path)

	return err == nil && info.IsDir()
}

// runAll runs tasks until they all return. The first error cancels the rest
// and is returned.
func runAll(cancel context.CancelFunc, tasks []func() error) error {
	errs := make(chan error, len(tasks))
	for _, task := range tasks {
		go func() { errs <- task() }()
	}

	var first error
	for range tasks {
		if err := <-errs; err != nil && first == nil {
			first = err
			cancel()
		}
	}

	return first
}
