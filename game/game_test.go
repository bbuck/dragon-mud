package game

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"bbuck.dev/dragon-mud/builtin"
	"bbuck.dev/dragon-mud/message"
	"bbuck.dev/dragon-mud/plugin"
	"bbuck.dev/dragon-mud/scripting"
	"bbuck.dev/dragon-mud/scripting/lua"
	"bbuck.dev/dragon-mud/session"
)

type fakeConn struct {
	messages chan message.Message
	closed   chan struct{}
}

func (c *fakeConn) Write(m message.Message) error {
	c.messages <- m
	return nil
}

func (c *fakeConn) Close() error {
	close(c.closed)
	return nil
}

type client struct {
	t    *testing.T
	g    *Game
	conn *fakeConn
	s    *session.Session
}

func connect(t *testing.T, g *Game) *client {
	t.Helper()

	conn := &fakeConn{messages: make(chan message.Message, 100), closed: make(chan struct{})}
	c := &client{t: t, g: g, conn: conn, s: session.New(conn)}
	g.Connect(c.s)

	// A real transport reports the disconnect when its connection closes.
	go func() {
		<-conn.closed
		g.Disconnect(c.s)
	}()

	return c
}

func (c *client) send(line string) {
	c.g.Input(c.s, line)
}

// expect waits for a message containing want.
func (c *client) expect(want string) {
	c.t.Helper()

	timeout := time.After(time.Second)
	var seen []string
	for {
		select {
		case m := <-c.conn.messages:
			if strings.Contains(m.Text, want) {
				return
			}
			seen = append(seen, m.Text)
		case <-timeout:
			c.t.Fatalf("never received %q; got %q", want, seen)
		}
	}
}

func (c *client) login(name string) {
	c.t.Helper()

	c.expect("By what name")
	c.send(name)
	c.expect("Welcome, [W]")
}

// sources returns dragon:basics and, if given, a game plugin.
func sources(t *testing.T, gameFiles fstest.MapFS) []plugin.Source {
	t.Helper()

	basics, err := builtin.FS("basics")
	if err != nil {
		t.Fatal(err)
	}

	sources := []plugin.Source{{Origin: "basics", Files: basics, Builtin: true}}
	if gameFiles != nil {
		sources = append(sources, plugin.Source{Origin: "game", Files: gameFiles})
	}

	return sources
}

func newGame(t *testing.T, gameFiles fstest.MapFS) (*Game, error) {
	t.Helper()

	return New(context.Background(), Options{
		Name:      "Test Realm",
		NewEngine: func() scripting.Engine { return lua.New() },
		Plugins:   sources(t, gameFiles),
		Log:       slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
}

// startGame runs a game with dragon:basics and, if given, a game plugin.
func startGame(t *testing.T, gameFiles fstest.MapFS) *Game {
	t.Helper()

	g, err := newGame(t, gameFiles)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		g.Run(ctx)
		close(done)
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})

	return g
}

func TestPlayersTalk(t *testing.T) {
	g := startGame(t, nil)

	alice := connect(t, g)
	alice.login("alice")
	alice.expect("The Void")

	bob := connect(t, g)
	bob.login("BOB")
	alice.expect("Bob has arrived.")

	bob.send("say hi there")
	bob.expect(`You say, "hi there"`)
	alice.expect(`Bob says, "hi there"`)

	alice.send("'hello")
	bob.expect(`Alice says, "hello"`)

	alice.send("who")
	alice.expect("2 players.")
}

func TestLoginValidation(t *testing.T) {
	g := startGame(t, nil)

	alice := connect(t, g)
	alice.login("Alice")

	imposter := connect(t, g)
	imposter.expect("By what name")
	imposter.send("x")
	imposter.expect("2 to 20 letters")
	imposter.send("ALICE")
	imposter.expect("already here")
}

func TestUnknownCommand(t *testing.T) {
	g := startGame(t, nil)

	alice := connect(t, g)
	alice.login("Alice")
	alice.send("fly")
	alice.expect("Huh?")
}

func TestQuit(t *testing.T) {
	g := startGame(t, nil)

	alice := connect(t, g)
	alice.login("Alice")
	bob := connect(t, g)
	bob.login("Bob")

	bob.send("quit")
	bob.expect("Farewell, Bob!")
	alice.expect("Bob has left.")
}

func TestGamePluginOverridesLook(t *testing.T) {
	g := startGame(t, fstest.MapFS{
		"plugin.lua": {Data: []byte(`return { name = "game", version = "0.1.0" }`)},
		"commands.lua": {Data: []byte(`
			return {
				look = {
					override = true,
					execute = function(actor) game.send(actor.id, "A cozy tavern.") end,
				},
				dance = {
					desc = "Dance!",
					execute = function(actor) game.broadcast(actor.name .. " dances.") end,
				},
			}
		`)},
	})

	alice := connect(t, g)
	alice.login("Alice")
	alice.expect("A cozy tavern.")

	alice.send("dance")
	alice.expect("Alice dances.")

	alice.send("help")
	alice.expect("Dance!")
}

func TestOverrideMustBeDeclared(t *testing.T) {
	_, err := newGame(t, fstest.MapFS{
		"plugin.lua":   {Data: []byte(`return { name = "game" }`)},
		"commands.lua": {Data: []byte(`return { look = { execute = function() end } }`)},
	})
	if err == nil || !strings.Contains(err.Error(), "dragon:basics") {
		t.Errorf("New error = %v, want a conflict with dragon:basics", err)
	}
}

func TestScriptErrorsAreReported(t *testing.T) {
	g := startGame(t, fstest.MapFS{
		"plugin.lua": {Data: []byte(`return { name = "game" }`)},
		"commands.lua": {Data: []byte(`
			return {
				broken = { execute = function() error("oops") end },
				spin = { execute = function() while true do end end },
			}
		`)},
	})

	alice := connect(t, g)
	alice.login("Alice")

	alice.send("broken")
	alice.expect("oops")

	alice.send("spin")
	alice.expect("interrupted")

	// The game is still running.
	alice.send("say still here")
	alice.expect("still here")
}

func TestReload(t *testing.T) {
	files := fstest.MapFS{
		"plugin.lua": {Data: []byte(`return { name = "game" }`)},
		"commands.lua": {Data: []byte(`
			return { dance = { execute = function(actor) game.send(actor.id, "You waltz.") end } }
		`)},
	}
	g := startGame(t, files)

	alice := connect(t, g)
	alice.login("Alice")
	alice.send("dance")
	alice.expect("You waltz.")

	// Events are handled in order, so the reload finishes before the next
	// command runs.
	files["commands.lua"] = &fstest.MapFile{Data: []byte(`
		return { dance = { execute = function(actor) game.send(actor.id, "You tango.") end } }
	`)}
	g.Reload()
	alice.send("dance")
	alice.expect("You tango.")
}

func TestFailedReloadKeepsScripts(t *testing.T) {
	files := fstest.MapFS{
		"plugin.lua": {Data: []byte(`return { name = "game" }`)},
		"commands.lua": {Data: []byte(`
			return { dance = { execute = function(actor) game.send(actor.id, "You waltz.") end } }
		`)},
	}
	g := startGame(t, files)

	alice := connect(t, g)
	alice.login("Alice")

	files["commands.lua"] = &fstest.MapFile{Data: []byte(`return { dance = `)}
	g.Reload()
	alice.send("dance")
	alice.expect("You waltz.")
}
