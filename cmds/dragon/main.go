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

	"bbuck.dev/dragon-mud/builtin"
	"bbuck.dev/dragon-mud/command"
	"bbuck.dev/dragon-mud/config"
	"bbuck.dev/dragon-mud/game"
	"bbuck.dev/dragon-mud/hook"
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
  dragon hooks [<name>] [-dir <directory>]     list hooks, or show the order
                                               a hook's handlers run in
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
	case "hooks":
		err = runHooks(os.Args[2:], os.Stdout)
	case "version":
		fmt.Println("dragon", version)
	case "help", "-h", "--help":
		fmt.Print(usage)
	default:
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

	sources, err := pluginSources(*dir, cfg.Builtins)
	if err != nil {
		return err
	}

	db, err := store.Open(ctx, filepath.Join(*dir, "data", "world.db"))
	if err != nil {
		return err
	}
	defer db.Close()

	records, err := db.Load(ctx)
	if err != nil {
		return err
	}
	w, err := world.Load(records)
	if err != nil {
		return fmt.Errorf("loading the world: %w", err)
	}
	log.Info("loaded world", termlog.PrefixKey, "store", "objects", w.Len())
	d.greet(cfg.Name, w.Len())

	g, err := game.New(ctx, game.Options{
		Name:      cfg.Name,
		NewEngine: func() scripting.Engine { return lua.New() },
		Plugins:   sources,
		World:     w,
		Store:     db,
		Log:       log.With(termlog.PrefixKey, "game"),
	})
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var tasks []func() error
	tasks = append(tasks, func() error { return g.Run(ctx) })
	tasks = append(tasks, func() error { return d.keepWatch(ctx, activity, idleAfter) })
	if gameDir := filepath.Join(*dir, "game"); isDir(gameDir) {
		tasks = append(tasks, func() error {
			return watch.Poll(ctx, os.DirFS(gameDir), []string{"*.lua", "*.tmpl"}, reloadInterval, func() {
				log.Info("scripts changed; reloading", termlog.PrefixKey, "watch")
				d.reloaded()
				g.Reload()
			})
		})
	}
	if cfg.Telnet.Enabled {
		tasks = append(tasks, func() error {
			return telnet.Serve(ctx, cfg.Telnet.Address, g, log.With(termlog.PrefixKey, "telnet"))
		})
	}
	if cfg.Web.Client.Enabled {
		tasks = append(tasks, func() error {
			return web.Serve(ctx, web.Options{Address: cfg.Web.Address, GameName: cfg.Name}, g, log.With(termlog.PrefixKey, "web"))
		})
	}

	if err := runAll(cancel, tasks); err != nil {
		return err
	}
	d.farewell()

	return nil
}

func runHooks(args []string, out io.Writer) error {
	flags := flag.NewFlagSet("hooks", flag.ContinueOnError)
	dir := flags.String("dir", ".", "the game directory")

	// Allow the hook name before or after flags.
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

	hooks, err := game.Hooks(context.Background(), game.Options{
		Name:      cfg.Name,
		NewEngine: func() scripting.Engine { return lua.New() },
		Plugins:   sources,
	})
	if err != nil {
		return err
	}

	if name == "" {
		return listHooks(out, hooks)
	}

	return showHook(out, hooks, name)
}

// listHooks prints every hook with handlers and who handles it.
func listHooks(out io.Writer, hooks *hook.Registry) error {
	names := hooks.Names()
	if len(names) == 0 {
		fmt.Fprintln(out, "No plugin handles any hooks yet. Add handlers in game/hooks.lua.")
		return nil
	}

	fmt.Fprintln(out, "Hooks and notifications, with their handlers in the order they run:")
	fmt.Fprintln(out)

	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	for _, name := range names {
		c, _ := hooks.Chain(name)
		plugins := make([]string, len(c.Handlers))
		for i, h := range c.Handlers {
			plugins[i] = h.Plugin
		}
		line := strings.Join(plugins, ", ")
		if len(c.Disabled) > 0 {
			line += fmt.Sprintf(" (%d disabled)", len(c.Disabled))
		}
		fmt.Fprintf(w, "  %s\t%s\n", name, line)
	}
	w.Flush()

	fmt.Fprintln(out)
	fmt.Fprintln(out, "Run dragon hooks <name> to see why a hook's handlers run in that order.")

	return nil
}

// showHook prints one hook's handlers in order and why they're in it.
func showHook(out io.Writer, hooks *hook.Registry, name string) error {
	c, ok := hooks.Chain(name)
	if !ok {
		names := hooks.Names()
		if len(names) == 0 {
			return fmt.Errorf("no plugin handles %q; no plugin handles any hooks yet", name)
		}
		return fmt.Errorf("no plugin handles %q.%s Hooks with handlers: %s.",
			name, command.DidYouMean(name, names), strings.Join(names, ", "))
	}

	fmt.Fprintf(out, "%s runs these handlers in order:\n\n", name)

	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	for i, h := range c.Handlers {
		why := hooks.Explain(c, h)
		if why == "" && !c.Wired {
			why = "load order"
		}
		fmt.Fprintf(w, "  %d.\t%s\t%s\t%s\n", i+1, h.Plugin, h.File(), why)
	}
	w.Flush()

	if c.Wired {
		fmt.Fprintf(out, "\nThe order is set in %s.\n", hooks.WiringFile())
	}
	if len(c.Disabled) > 0 {
		plugins := make([]string, len(c.Disabled))
		for i, h := range c.Disabled {
			plugins[i] = h.Plugin
		}
		fmt.Fprintf(out, "\nDisabled in %s: %s\n", hooks.WiringFile(), strings.Join(plugins, ", "))
	}

	return nil
}

// pluginSources lists the built-in plugins the game loads, in the engine's
// order, then the game's own plugin.
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
