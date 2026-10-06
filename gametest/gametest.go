// Package gametest runs a game's tests: files ending in _test.lua in a
// plugin's tests/ directory, each returning a table of named test
// functions. Other files there are helpers tests require. Every test
// gets a fresh game, with the game's plugins and an empty world in a
// database of its own, and drives it the way players do, through scripted
// sessions that send lines and expect output. dragon test runs them.
//
//	-- game/tests/chat_test.lua
//	return {
//	  ["say reaches the room"] = function(t)
//	    local alice, bob = t:connect(), t:connect()
//	    alice:login("Alice")
//	    bob:login("Bob")
//	    bob:send("say hi")
//	    alice:expect('Bob says, "hi"')
//	  end,
//	}
//
// Tests run in a Lua state of their own, apart from the game's, so a test
// reaches the game only through sessions and t:eval, which runs code in
// the game's own plugin. See docs/plugins.md.
package gametest

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"bbuck.dev/dragon-mud/ansi"
	"bbuck.dev/dragon-mud/auth"
	"bbuck.dev/dragon-mud/game"
	"bbuck.dev/dragon-mud/message"
	"bbuck.dev/dragon-mud/plugin"
	"bbuck.dev/dragon-mud/scripting"
	"bbuck.dev/dragon-mud/session"
	"bbuck.dev/dragon-mud/store"
	"bbuck.dev/dragon-mud/world"
)

// Dir is the directory in a plugin that holds its tests.
const Dir = "tests"

// testTimeout is how long one test may run.
const testTimeout = 30 * time.Second

// expectTimeout is how long expect waits for output by default.
const expectTimeout = 2 * time.Second

// cheapParams hash test accounts' passwords quickly; tests never hold real
// passwords.
var cheapParams = auth.Params{Memory: 64, Time: 1, Threads: 1}

// Suite is one plugin's tests.
type Suite struct {
	// Origin names the tests' directory, such as game/tests, in output.
	Origin string

	// Files is the tests directory.
	Files fs.FS
}

// Options configure a run.
type Options struct {
	// Name is the game's name.
	Name string

	// NewEngine returns a fresh scripting engine, for each game and for
	// the tests themselves.
	NewEngine func() scripting.Engine

	// Plugins are the game's plugins, as dragon serve loads them.
	Plugins []plugin.Source

	// TextWidth is the width text templates lay text out to.
	TextWidth int

	// Suites are the tests to run.
	Suites []Suite

	// Run, if not nil, runs only the tests whose name it matches.
	Run *regexp.Regexp

	// Out is where results go.
	Out io.Writer
}

// Result counts what a run did.
type Result struct {
	Passed, Failed int
}

// Run runs every test in opts.Suites, writing each result to opts.Out.
// It returns an error only when a tests directory can't be read; failing
// tests, and test files that don't load, are counted in the result.
func Run(ctx context.Context, opts Options) (Result, error) {
	var result Result
	for _, suite := range opts.Suites {
		files, err := testFiles(suite.Files)
		if err != nil {
			return result, fmt.Errorf("%s: %w", suite.Origin, err)
		}
		for _, file := range files {
			if err := runFile(ctx, opts, suite, file, &result); err != nil {
				result.Failed++
				fmt.Fprintf(opts.Out, "%s/%s\n  FAIL  the file doesn't load\n", suite.Origin, file)
				for line := range strings.SplitSeq(err.Error(), "\n") {
					fmt.Fprintf(opts.Out, "        %s\n", line)
				}
			}
		}
	}

	return result, nil
}

// TestSuffix ends the name of every test file; other files in tests/ are
// helpers.
const TestSuffix = "_test.lua"

// testFiles lists the test files at the top of a tests directory, sorted.
func testFiles(fsys fs.FS) ([]string, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, err
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), TestSuffix) {
			files = append(files, e.Name())
		}
	}

	return files, nil
}

// runFile runs one test file's tests.
func runFile(ctx context.Context, opts Options, suite Suite, file string, result *Result) error {
	where := suite.Origin + "/" + file
	engine := opts.NewEngine()
	defer engine.Close()

	scope, err := engine.Scope(suite.Origin, suite.Files, nil, nil)
	if err != nil {
		return err
	}
	source, err := fs.ReadFile(suite.Files, file)
	if err != nil {
		return err
	}
	value, err := scope.Eval(ctx, where, string(source))
	if err != nil {
		return err
	}

	tests, ok := value.(map[string]any)
	if list, isList := value.([]any); isList && len(list) == 0 {
		tests, ok = map[string]any{}, true
	}
	if !ok {
		return fmt.Errorf("%s returns a %s, but a test file returns a table of tests, like return { [\"say reaches the room\"] = function(t) ... end }.", where, scripting.TypeName(value))
	}

	names := slices.Sorted(maps.Keys(tests))
	var selected []string
	for _, name := range names {
		if opts.Run == nil || opts.Run.MatchString(name) {
			selected = append(selected, name)
		}
	}
	if len(selected) == 0 {
		return nil
	}

	fmt.Fprintln(opts.Out, where)
	for _, name := range selected {
		fn, ok := tests[name].(scripting.Function)
		if !ok {
			return fmt.Errorf("%s: test %q is a %s, but each test is a function(t).", where, name, scripting.TypeName(tests[name]))
		}

		start := time.Now()
		err := runTest(ctx, opts, fn)
		elapsed := time.Since(start).Round(time.Millisecond)
		if err != nil {
			result.Failed++
			fmt.Fprintf(opts.Out, "  FAIL  %s (%s)\n", name, elapsed)
			for line := range strings.SplitSeq(err.Error(), "\n") {
				fmt.Fprintf(opts.Out, "        %s\n", line)
			}
			continue
		}
		result.Passed++
		fmt.Fprintf(opts.Out, "  ok    %s (%s)\n", name, elapsed)
	}

	return nil
}

// runTest runs fn against a fresh game.
func runTest(ctx context.Context, opts Options, fn scripting.Function) error {
	dir, err := os.MkdirTemp("", "dragon-test-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)

	db, err := store.Open(ctx, filepath.Join(dir, "world.db"))
	if err != nil {
		return err
	}
	defer db.Close()

	g, err := game.New(ctx, game.Options{
		Name:      opts.Name,
		NewEngine: opts.NewEngine,
		Plugins:   opts.Plugins,
		TextWidth: opts.TextWidth,
		World:     world.New(),
		Store:     db,
		Hasher:    auth.NewHasher(cheapParams, 4),
		Log:       slog.New(slog.DiscardHandler),
	})
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, testTimeout)
	defer cancel()
	stopped := make(chan struct{})
	go func() {
		g.Run(ctx)
		close(stopped)
	}()
	defer func() {
		cancel()
		<-stopped
	}()

	t := newRun(ctx, g)
	defer t.close()
	_, err = fn.Call(ctx, t.handle())
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("the test took longer than %s: %w", testTimeout, err)
	}

	return err
}

// run is one test's state: the game and the sessions the test connected.
type run struct {
	ctx      context.Context
	g        *game.Game
	testType *scripting.Type
	connType *scripting.Type
	conns    []*conn
}

func newRun(ctx context.Context, g *game.Game) *run {
	r := &run{ctx: ctx, g: g}
	r.connType = r.makeConnType()
	r.testType = r.makeTestType()

	return r
}

func (r *run) handle() scripting.Handle {
	return scripting.Handle{Type: r.testType, Key: 0}
}

// close disconnects every session the test left connected.
func (r *run) close() {
	for _, c := range r.conns {
		c.s.Close()
	}
}

// makeTestType is the t a test gets.
//
//	t:connect()        a new session, at the login prompt
//	t:eval(source)     run Lua in the game's own plugin and return what it
//	                   returns, with objects as their ids
func (r *run) makeTestType() *scripting.Type {
	return &scripting.Type{
		Name: "test",
		Methods: map[string]scripting.Method{
			"connect": func(_ any, _ scripting.Args) (any, error) {
				c := &conn{messages: make(chan message.Message, 1000), closed: make(chan struct{})}
				c.s = session.New(c)
				r.conns = append(r.conns, c)
				r.g.Connect(c.s)
				go func() {
					<-c.closed
					r.g.Disconnect(c.s)
				}()
				return scripting.Handle{Type: r.connType, Key: len(r.conns) - 1}, nil
			},
			"eval": func(_ any, args scripting.Args) (any, error) {
				source, err := args.String(0)
				if err != nil {
					return nil, err
				}
				return r.g.Eval(r.ctx, "t:eval", source)
			},
		},
	}
}

// makeConnType is a session a test connected.
//
//	p:send(line)               type line
//	p:expect(text[, seconds])  wait for output containing text, color codes
//	                           removed, and return that line
//	p:expect_without(text, forbidden...)
//	                           expect, failing if any forbidden text comes
//	                           first
//	p:login(name)              create an account called name and log in
//	p:disconnect()             close the connection
//	p:output()                 every line received so far
func (r *run) makeConnType() *scripting.Type {
	conn := func(key any) *conn { return r.conns[key.(int)] }

	return &scripting.Type{
		Name: "session",
		Methods: map[string]scripting.Method{
			"send": func(key any, args scripting.Args) (any, error) {
				line, err := args.String(0)
				if err != nil {
					return nil, err
				}
				r.g.Input(conn(key).s, line)
				return nil, nil
			},
			"expect": func(key any, args scripting.Args) (any, error) {
				want, err := args.String(0)
				if err != nil {
					return nil, err
				}
				timeout := expectTimeout
				if args.Len() > 1 && args[1] != nil {
					seconds, err := args.Float(1)
					if err != nil {
						return nil, err
					}
					timeout = time.Duration(seconds * float64(time.Second))
				}
				return conn(key).expect(r.ctx, want, nil, timeout)
			},
			"expect_without": func(key any, args scripting.Args) (any, error) {
				want, err := args.String(0)
				if err != nil {
					return nil, err
				}
				var forbidden []string
				for i := 1; i < args.Len(); i++ {
					text, err := args.String(i)
					if err != nil {
						return nil, err
					}
					forbidden = append(forbidden, text)
				}
				return conn(key).expect(r.ctx, want, forbidden, expectTimeout)
			},
			"login": func(key any, args scripting.Args) (any, error) {
				name, err := args.String(0)
				if err != nil {
					return nil, err
				}
				return nil, conn(key).login(r.ctx, r.g, name)
			},
			"disconnect": func(key any, _ scripting.Args) (any, error) {
				conn(key).s.Close()
				return nil, nil
			},
			"output": func(key any, _ scripting.Args) (any, error) {
				c := conn(key)
				c.drain()
				lines := make([]any, len(c.seen))
				for i, line := range c.seen {
					lines[i] = line
				}
				return lines, nil
			},
		},
	}
}

// conn is a scripted session's connection: what the game sends it waits
// in messages until the test expects it.
type conn struct {
	s        *session.Session
	messages chan message.Message
	closed   chan struct{}

	// seen is every line received, color codes removed; next is where
	// the next expect starts reading.
	seen []string
	next int
}

func (c *conn) Write(m message.Message) error {
	select {
	case c.messages <- m:
	default:
		// A test that never reads its output shouldn't stall the game.
	}
	return nil
}

func (c *conn) Close() error {
	close(c.closed)
	return nil
}

// drain moves every message waiting into seen.
func (c *conn) drain() {
	for {
		select {
		case m := <-c.messages:
			c.seen = append(c.seen, ansi.Purge(m.Text))
		default:
			return
		}
	}
}

// expect waits until a line containing want arrives after the last one
// expect returned, and returns it. A line containing any of forbidden
// first is an error, and so is running out of time.
func (c *conn) expect(ctx context.Context, want string, forbidden []string, timeout time.Duration) (string, error) {
	deadline := time.After(timeout)
	for {
		for ; c.next < len(c.seen); c.next++ {
			line := c.seen[c.next]
			for _, bad := range forbidden {
				if strings.Contains(line, bad) {
					return "", fmt.Errorf("received %q before %q: %q", bad, want, line)
				}
			}
			if strings.Contains(line, want) {
				c.next++
				return line, nil
			}
		}

		select {
		case m := <-c.messages:
			c.seen = append(c.seen, ansi.Purge(m.Text))
		case <-deadline:
			return "", fmt.Errorf("never received %q within %s; received %s", want, timeout, quoted(c.seen))
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
}

// login goes through the engine's login to create an account called name.
// It returns once the account exists, where the game's characters mode
// takes over.
func (c *conn) login(ctx context.Context, g *game.Game, name string) error {
	steps := []struct{ expect, send string }{
		{"By what name", name},
		{"Create a new account?", "yes"},
		{"Choose a password", "test password"},
		{"Type it again", "test password"},
	}
	for _, step := range steps {
		if _, err := c.expect(ctx, step.expect, nil, expectTimeout); err != nil {
			return fmt.Errorf("logging in as %s: %w", name, err)
		}
		g.Input(c.s, step.send)
	}
	if _, err := c.expect(ctx, "Welcome, ", nil, expectTimeout); err != nil {
		return fmt.Errorf("logging in as %s: %w", name, err)
	}

	return nil
}

func quoted(lines []string) string {
	if len(lines) == 0 {
		return "nothing"
	}
	q := make([]string, len(lines))
	for i, line := range lines {
		q[i] = fmt.Sprintf("%q", line)
	}

	return strings.Join(q, ", ")
}
