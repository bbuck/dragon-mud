package game

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"bbuck.dev/dragon-mud/auth"
	"bbuck.dev/dragon-mud/builtin"
	"bbuck.dev/dragon-mud/message"
	"bbuck.dev/dragon-mud/plugin"
	"bbuck.dev/dragon-mud/scripting"
	"bbuck.dev/dragon-mud/scripting/lua"
	"bbuck.dev/dragon-mud/session"
	"bbuck.dev/dragon-mud/store"
	"bbuck.dev/dragon-mud/world"
)

// cheapParams keep tests fast; never use them for real passwords.
var cheapParams = auth.Params{Memory: 64, Time: 1, Threads: 1}

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

// expect waits for a message containing want and returns it.
func (c *client) expect(want string) message.Message {
	c.t.Helper()

	timeout := time.After(2 * time.Second)
	var seen []string
	for {
		select {
		case m := <-c.conn.messages:
			if strings.Contains(m.Text, want) {
				return m
			}
			seen = append(seen, m.Text)
		case <-timeout:
			c.t.Fatalf("never received %q; got %q", want, seen)
		}
	}
}

// login creates an account called name and enters the game.
func (c *client) login(name string) {
	c.t.Helper()

	c.expect("By what name")
	c.send(name)
	c.expect("Create a new account?")
	c.send("yes")
	c.expect("Choose a password")
	c.send(" secret pass ")
	c.expect("Type it again")
	c.send(" secret pass ")
	c.expect("Welcome, [W]")
}

// relogin logs in to an existing account.
func (c *client) relogin(name, password string) {
	c.t.Helper()

	c.expect("By what name")
	c.send(name)
	c.expect("Password:")
	c.send(password)
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

func openStore(t *testing.T) *store.Store {
	t.Helper()

	db, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "world.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	return db
}

func newGame(t *testing.T, gameFiles fstest.MapFS) (*Game, error) {
	t.Helper()

	return newGameWith(t, openStore(t), gameFiles)
}

// newGameWith returns a game whose world is loaded from db, as dragon serve
// does.
func newGameWith(t *testing.T, db *store.Store, gameFiles fstest.MapFS) (*Game, error) {
	t.Helper()

	records, err := db.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	w, err := world.Load(records)
	if err != nil {
		t.Fatal(err)
	}

	return New(context.Background(), Options{
		Name:      "Test Realm",
		NewEngine: func() scripting.Engine { return lua.New() },
		Plugins:   sources(t, gameFiles),
		World:     w,
		Store:     db,
		Hasher:    auth.NewHasher(cheapParams, 4),
		Log:       slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
}

// startGame runs a game with dragon:basics and, if given, a game plugin.
func startGame(t *testing.T, gameFiles fstest.MapFS) *Game {
	t.Helper()

	return startGameWith(t, openStore(t), gameFiles)
}

func startGameWith(t *testing.T, db *store.Store, gameFiles fstest.MapFS) *Game {
	t.Helper()

	g, err := newGameWith(t, db, gameFiles)
	if err != nil {
		t.Fatal(err)
	}
	runGame(t, g)

	return g
}

// runGame runs g until the test ends or the returned function is called.
func runGame(t *testing.T, g *Game) (stop func()) {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		g.Run(ctx)
		close(done)
	}()

	var once sync.Once
	stop = func() {
		once.Do(func() {
			cancel()
			<-done
		})
	}
	t.Cleanup(stop)

	return stop
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

	c := connect(t, g)
	c.expect("By what name")
	c.send("x")
	c.expect("2 to 20 letters")

	c.send("alice")
	c.expect("Create a new account?")
	c.send("maybe")
	c.expect("yes or no")
	c.send("no")
	c.expect("By what name")

	c.send("alice")
	c.expect("Create a new account?")
	c.send("y")
	if m := c.expect("Choose a password"); !m.Secret {
		t.Error("password prompt isn't secret")
	}
	c.send("short")
	c.expect("at least 8")
	c.send("long enough")
	c.send("different")
	c.expect("didn't match")
	c.send("long enough")
	c.send("long enough")
	c.expect("Welcome, [W]Alice[x]")
}

func TestReturningPlayer(t *testing.T) {
	g := startGame(t, nil)

	alice := connect(t, g)
	alice.login("alice")
	alice.send("quit")
	alice.expect("Farewell")

	again := connect(t, g)
	again.relogin("ALICE", "secret pass")
	again.expect("Wrong password")
	again.send(" secret pass ")
	again.expect("Welcome, [W]Alice[x]")
}

func TestTooManyWrongPasswords(t *testing.T) {
	g := startGame(t, nil)

	alice := connect(t, g)
	alice.login("alice")

	intruder := connect(t, g)
	intruder.relogin("alice", "guess one")
	for _, guess := range []string{"guess two", "guess three"} {
		intruder.expect("Wrong password")
		intruder.send(guess)
	}
	intruder.expect("Too many wrong passwords")

	select {
	case <-intruder.conn.closed:
	case <-time.After(time.Second):
		t.Error("connection not closed")
	}
}

func TestLoginTakesOver(t *testing.T) {
	g := startGame(t, nil)

	alice := connect(t, g)
	alice.login("alice")
	bob := connect(t, g)
	bob.login("bob")

	again := connect(t, g)
	again.relogin("alice", " secret pass ")
	again.expect("Welcome, [W]Alice[x]")
	alice.expect("connected from somewhere else")

	again.send("say back again")
	bob.expect(`Alice says, "back again"`)

	bob.send("who")
	bob.expect("2 players.")
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
					replace = true,
					execute = function(actor) actor:send("A cozy tavern.") end,
				},
				dance = {
					desc = "Dance!",
					execute = function(actor) game.broadcast(actor:get("name") .. " dances.") end,
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
			return { dance = { execute = function(actor) actor:send("You waltz.") end } }
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
		return { dance = { execute = function(actor) actor:send("You tango.") end } }
	`)}
	g.Reload()
	alice.send("dance")
	alice.expect("You tango.")
}

func TestFailedReloadKeepsScripts(t *testing.T) {
	files := fstest.MapFS{
		"plugin.lua": {Data: []byte(`return { name = "game" }`)},
		"commands.lua": {Data: []byte(`
			return { dance = { execute = function(actor) actor:send("You waltz.") end } }
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

func TestLoginUpgradesOldHashes(t *testing.T) {
	ctx := context.Background()
	db := openStore(t)

	old := auth.Params{Memory: 32, Time: 1, Threads: 1}
	hash, err := auth.HashPassword("old password", old)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.CreateAccount(ctx, "Alice", hash); err != nil {
		t.Fatal(err)
	}

	g := startGameWith(t, db, nil)
	alice := connect(t, g)
	alice.relogin("alice", "old password")
	alice.expect("Welcome, [W]Alice[x]")

	deadline := time.Now().Add(2 * time.Second)
	for {
		account, _, err := db.Account(ctx, "alice")
		if err != nil {
			t.Fatal(err)
		}
		if !auth.NeedsRehash(account.PasswordHash, cheapParams) {
			if ok, _ := auth.CheckPassword("old password", account.PasswordHash); !ok {
				t.Fatal("upgraded hash doesn't match the password")
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("hash was never upgraded")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// builder is a game plugin that exercises objects from Lua.
var builder = fstest.MapFS{
	"plugin.lua": {Data: []byte(`return { name = "game" }`)},
	"commands.lua": {Data: []byte(`
		local function title(o)
			return o and o:get("title") or "nowhere"
		end

		return {
			dig = { forms = { { "dig <title>", function(actor, args)
				local room = world.create()
				room:set("title", args.title)
				actor:set("home", room)
				actor:move_to(room)
				actor:send("You dig " .. args.title .. ".")
			end } } },

			where = { execute = function(actor)
				actor:send("You are in " .. title(actor.location) .. "; home is " .. title(actor:get("home")) .. ".")
			end },

			look = { replace = true, execute = function(actor)
				local names = {}
				for _, thing in ipairs(actor.location and actor.location.contents or {}) do
					table.insert(names, thing:get("name") or thing.id)
				end
				actor:send("Here: " .. table.concat(names, ", "))
			end },

			crumble = { execute = function(actor)
				local room = actor.location
				actor:move_to(nil)
				world.destroy(room)
				actor:send("Home is now " .. tostring(actor:get("home")) .. ".")
			end },

			stale = { execute = function(actor)
				local o = world.create()
				world.destroy(o)
				o:set("x", 1)
			end },

			vanish = { execute = function(actor) world.destroy(actor) end },

			poke = { execute = function(actor)
				world.create():send("nobody hears this")
				actor:send("poked")
			end },
		}
	`)},
}

func TestObjectsFromLua(t *testing.T) {
	g := startGame(t, builder)

	alice := connect(t, g)
	alice.login("alice")
	alice.send("dig The Cellar")
	alice.expect("You dig The Cellar.")
	alice.send("where")
	alice.expect("You are in The Cellar; home is The Cellar.")

	bob := connect(t, g)
	bob.login("bob")
	bob.send("dig The Attic")
	bob.expect("You dig The Attic.")
	bob.send("look")
	bob.expect("Here: Bob")

	alice.send("stale")
	alice.expect("object has been destroyed")
	alice.send("vanish")
	alice.expect("can't destroy an object someone is playing")
	alice.send("poke")
	alice.expect("poked")

	alice.send("crumble")
	alice.expect("Home is now nil.")
	alice.send("where")
	alice.expect("You are in nowhere; home is nowhere.")
}

func TestObjectsPersist(t *testing.T) {
	db := openStore(t)

	g, err := newGameWith(t, db, builder)
	if err != nil {
		t.Fatal(err)
	}
	stop := runGame(t, g)
	alice := connect(t, g)
	alice.login("alice")
	alice.send("dig The Cellar")
	alice.expect("You dig The Cellar.")
	stop()

	g = startGameWith(t, db, builder)
	again := connect(t, g)
	again.relogin("alice", " secret pass ")
	again.expect("Welcome, [W]Alice[x]")
	again.send("where")
	again.expect("You are in The Cellar; home is The Cellar.")
}

func TestCreateWithOptions(t *testing.T) {
	g := startGame(t, fstest.MapFS{
		"plugin.lua": {Data: []byte(`return { name = "game" }`)},
		"commands.lua": {Data: []byte(`
			return {
				forge = { execute = function(actor)
					local sword = world.create{ properties = { damage = "1d8", weight = 3 } }
					local blade = world.create{
						parent = sword,
						location = actor,
						key = "blade",
						properties = { damage = "1d10", owner = actor },
					}
					actor:send(table.concat({
						blade:get("damage"), blade:get("weight"),
						tostring(blade.location == actor), tostring(blade:get("owner") == actor),
						tostring(world.keyed("blade") == blade),
					}, " "))
				end },

				typo = { execute = function() world.create{ parnet = 1 } end },

				taken = { execute = function(actor)
					local before = #actor.contents
					local ok = pcall(world.create, { key = "blade", location = actor })
					actor:send("failed=" .. tostring(not ok) .. " left=" .. (#actor.contents - before))
				end },
			}
		`)},
	})

	alice := connect(t, g)
	alice.login("alice")

	alice.send("forge")
	alice.expect("1d10 3 true true true")

	alice.send("typo")
	alice.expect(`unknown option "parnet"`)

	alice.send("taken")
	alice.expect("failed=true left=0")
}

func TestSayTo(t *testing.T) {
	g := startGame(t, nil)

	alice := connect(t, g)
	alice.login("alice")
	bob := connect(t, g)
	bob.login("bob")

	alice.send("say hi to bob")
	alice.expect(`You say to Bob, "hi"`)
	bob.expect(`Alice says to you, "hi"`)

	// No one called "the store": the whole line is the message.
	alice.send("say I went to the store")
	bob.expect(`Alice says, "I went to the store"`)

	alice.send(`say "hi to bob"`)
	bob.expect(`Alice says, "hi to bob"`)

	alice.send("'shortcut")
	bob.expect(`Alice says, "shortcut"`)

	alice.send("say")
	alice.expect("Usage:")

	alice.send("help say")
	alice.expect("say <message> to <target:object:here,online>")
}

// doors is a game plugin with its own slot type.
var doors = fstest.MapFS{
	"plugin.lua": {Data: []byte(`return { name = "game" }`)},
	"slots.lua": {Data: []byte(`
		local doors = { red = { open = true }, blue = { open = false } }
		return {
			door = {
				desc = "A door by color.",
				modifiers = { "open" },
				resolve = function(actor, text, modifiers)
					local door = doors[text]
					if not door then
						return nil, "There's no " .. text .. " door."
					end
					if modifiers.open and not door.open then
						return nil, "The " .. text .. " door is shut."
					end
					return text
				end,
			},
			sneaky = {
				resolve = function(actor, text)
					actor:set("tampered", true)
					return text
				end,
			},
		}
	`)},
	"commands.lua": {Data: []byte(`
		return {
			enter = { forms = {
				{ "enter <door:door:open>", function(actor, args) actor:send("You step through the " .. args.door .. " door.") end },
				{ "knock <door:door>", function(actor, args) actor:send("You knock on the " .. args.door .. " door.") end },
			} },
			tamper = { forms = { { "tamper <x:sneaky>", function() end } } },
			say = { forms = {
				{ "say <message> loudly", function(actor, args) actor:send("You shout: " .. args.message) end },
			} },
			get = { forms = {
				{ "get <thing:object:here>", function(actor, args) actor:send("You get " .. args.thing:get("name") .. " (" .. args.thing:get("n") .. ").") end },
			} },
			make = { forms = {
				{ "make <what>", function(actor, args)
					for i = 1, 2 do
						world.create{ location = actor.location, properties = { name = args.what, n = i } }
					end
					actor:send("Made two.")
				end },
			} },
			go = { execute = function(actor) actor:move_to(world.create()) actor:send("Moved.") end },
		}
	`)},
}

func TestPluginSlotTypes(t *testing.T) {
	g := startGame(t, doors)

	alice := connect(t, g)
	alice.login("alice")

	alice.send("enter red")
	alice.expect("You step through the red door.")
	alice.send("enter blue")
	alice.expect("The blue door is shut.")
	alice.send("knock blue")
	alice.expect("You knock on the blue door.")
	alice.send("enter green")
	alice.expect("There's no green door.")

	alice.send("tamper x")
	alice.expect("can't change the world while resolving input")

	// The game's form joined dragon:basics' say.
	alice.send("say hello loudly")
	alice.expect("You shout: hello")
	alice.send("say hello")
	alice.expect(`You say, "hello"`)
}

func TestObjectSlot(t *testing.T) {
	g := startGame(t, doors)

	alice := connect(t, g)
	alice.login("alice")
	alice.send("go")
	alice.expect("Moved.")
	alice.send("make sword")
	alice.expect("Made two.")

	alice.send("get sword")
	alice.expect("Which 'sword' do you mean? There are 2")
	alice.send("get 2.sword")
	alice.expect("You get sword (2).")
	alice.send("get 3.sword")
	alice.expect("There are only 2 of 'sword'.")
	alice.send("get axe")
	alice.expect("You don't see 'axe' here.")
}

func TestBadCommandsFileIsExplained(t *testing.T) {
	_, err := newGame(t, fstest.MapFS{
		"plugin.lua":   {Data: []byte(`return { name = "game" }`)},
		"commands.lua": {Data: []byte(`return { jump = { froms = {} } }`)},
	})
	want := `game/commands.lua: command "jump" has an unknown field "froms". Did you mean "forms"? Allowed fields: desc, forms, execute, replace.`
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Errorf("error = %v\nwant it to contain %q", err, want)
	}
}
