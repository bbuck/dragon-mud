// Command dragon creates and runs DragonMUD games.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"bbuck.dev/dragon-mud/ansi"
	"bbuck.dev/dragon-mud/builtin"
	"bbuck.dev/dragon-mud/config"
	"bbuck.dev/dragon-mud/game"
	"bbuck.dev/dragon-mud/plugin"
	"bbuck.dev/dragon-mud/random"
	"bbuck.dev/dragon-mud/scaffold"
	"bbuck.dev/dragon-mud/scripting/lua"
	"bbuck.dev/dragon-mud/transport/telnet"
	"bbuck.dev/dragon-mud/transport/web"
)

const version = "0.1.0-dev"

const usage = `dragon creates and runs DragonMUD games.

Usage:
  dragon new <directory> [-name "Game Name"]   create a new game
  dragon serve [-dir <directory>]              run the game in a directory
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

	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	greet()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	engine := lua.New()
	defer engine.Close()

	g, err := game.New(cfg.Name, engine, log)
	if err != nil {
		return err
	}

	if err := loadPlugins(ctx, g, engine, *dir); err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var tasks []func() error
	tasks = append(tasks, func() error { return g.Run(ctx) })
	if cfg.Telnet.Enabled {
		tasks = append(tasks, func() error {
			return telnet.Serve(ctx, cfg.Telnet.Address, g, log)
		})
	}
	if cfg.Web.Client.Enabled {
		tasks = append(tasks, func() error {
			return web.Serve(ctx, web.Options{Address: cfg.Web.Address, GameName: cfg.Name}, g, log)
		})
	}

	return runAll(cancel, tasks)
}

// loadPlugins loads the built-in plugins, then the game's own plugin.
func loadPlugins(ctx context.Context, g *game.Game, engine *lua.Engine, dir string) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	for _, name := range builtin.Names {
		files, err := builtin.FS(name)
		if err != nil {
			return err
		}
		if err := loadPlugin(ctx, g, engine, files, true); err != nil {
			return fmt.Errorf("built-in plugin %s: %w", name, err)
		}
	}

	gameDir := filepath.Join(dir, "game")
	if _, err := os.Stat(gameDir); errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err := loadPlugin(ctx, g, engine, os.DirFS(gameDir), false); err != nil {
		return fmt.Errorf("%s: %w", gameDir, err)
	}

	return nil
}

func loadPlugin(ctx context.Context, g *game.Game, engine *lua.Engine, files fs.FS, isBuiltin bool) error {
	p, err := plugin.Open(ctx, engine, files, isBuiltin)
	if err != nil {
		return err
	}

	return g.LoadPlugin(ctx, p)
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

// greet prints a welcome from a randomly colored dragon, as the original
// engine did.
func greet() {
	dragons := []string{
		"[l][-W]black[x]", "[c220]brass[x]", "[R]red[x]", "[c208]bronze[x]", "[G]green[x]",
		"[Y]gold[x]", "[B]blue[x]", "[c202]copper[x]", "[W]white[x]", "[c250][u]silver[x]",
	}

	rng := random.New(uint64(time.Now().UnixNano()))
	dragon := dragons[rng.Range(0, len(dragons)-1)]

	fmt.Println(ansi.Colorize("A " + dragon + " dragon arrives to serve you today."))
}
