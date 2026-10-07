package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"syscall"

	"bbuck.dev/dragon-mud/config"
	"bbuck.dev/dragon-mud/gametest"
	"bbuck.dev/dragon-mud/scripting"
	"bbuck.dev/dragon-mud/scripting/lua"
)

// runTest runs the tests in the game's tests/ directory and its local
// plugins', each against a fresh copy of the game.
func runTest(args []string, out io.Writer) error {
	flags := flag.NewFlagSet("test", flag.ContinueOnError)
	dir := flags.String("dir", ".", "the game directory")
	pattern := flags.String("run", "", "run only the tests whose name matches this regular expression")
	if err := flags.Parse(args); err != nil {
		return err
	}

	var run *regexp.Regexp
	if *pattern != "" {
		var err error
		if run, err = regexp.Compile(*pattern); err != nil {
			return fmt.Errorf("-run: %w", err)
		}
	}

	cfg, err := config.Load(*dir)
	if err != nil {
		return err
	}
	sources, err := pluginSources(*dir, cfg)
	if err != nil {
		return err
	}

	var suites []gametest.Suite
	for _, src := range sources {
		if src.Builtin {
			continue
		}
		testsDir := filepath.Join(src.Origin, gametest.Dir)
		if isDir(testsDir) {
			rel, err := filepath.Rel(*dir, testsDir)
			if err != nil {
				rel = testsDir
			}
			suites = append(suites, gametest.Suite{Origin: filepath.ToSlash(rel), Files: os.DirFS(testsDir)})
		}
	}
	if len(suites) == 0 {
		fmt.Fprintf(out, "There are no tests yet. Add files ending in %s to game/%s/, or to a local plugin's %s/.\n", gametest.TestSuffix, gametest.Dir, gametest.Dir)
		return nil
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	result, err := gametest.Run(ctx, gametest.Options{
		Name:      cfg.Name,
		NewEngine: func() scripting.Engine { return lua.New() },
		Plugins:   sources,
		TextWidth: cfg.TextWidth(),
		Suites:    suites,
		Run:       run,
		Out:       out,
	})
	if err != nil {
		return err
	}

	fmt.Fprintf(out, "\n%d passed, %d failed\n", result.Passed, result.Failed)
	if result.Failed > 0 {
		return errTestsFailed
	}

	return nil
}

// errTestsFailed makes dragon exit with an error after tests fail, which
// the results already explain.
var errTestsFailed = fmt.Errorf("tests failed")
