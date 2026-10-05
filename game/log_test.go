package game

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"testing/fstest"

	"bbuck.dev/dragon-mud/auth"
	"bbuck.dev/dragon-mud/scripting"
	"bbuck.dev/dragon-mud/scripting/lua"
	"bbuck.dev/dragon-mud/world"
)

// syncBuffer is a buffer the game loop writes while the test reads.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func TestScriptsLog(t *testing.T) {
	var logs syncBuffer
	g, err := New(context.Background(), Options{
		Name:      "Test Realm",
		NewEngine: func() scripting.Engine { return lua.New() },
		Plugins: sources(t, fstest.MapFS{
			"plugin.lua": file(`return { name = "game" }`),
			"commands.lua": file(`
				return {
					note = { execute = function(actor)
						log.debug("looking around", { where = "nowhere" })
						log.warn("no room", { player = actor, tries = 2, tags = { "lost", actor } })
						actor:send("noted")
					end },
					bad = { execute = function(actor) log.info({ "not a message" }) end },
				}
			`),
		}),
		World:  world.New(),
		Store:  openStore(t),
		Hasher: auth.NewHasher(cheapParams, 4),
		Log:    slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug})),
	})
	if err != nil {
		t.Fatal(err)
	}
	runGame(t, g)

	alice := connect(t, g)
	alice.login("Alice")
	alice.send("note")
	alice.expect("noted")

	got := logs.String()
	for _, want := range []string{
		`level=DEBUG msg="looking around" plugin=game where=nowhere`,
		`level=WARN msg="no room" plugin=game player="object `,
		`tags="[lost object `,
		`tries=2`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("log is missing %q:\n%s", want, got)
		}
	}

	alice.send("bad")
	alice.expect(`log.info: takes a message, like log.info("placed player", { player = p })`)
}
