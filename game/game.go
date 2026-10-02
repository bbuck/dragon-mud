// Package game owns the game world. A single goroutine runs the game loop,
// taking events from a channel and handling each one to completion before
// the next. Nothing outside the loop reads or writes game state.
// See docs/design.md §2.
package game

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"bbuck.dev/dragon-mud/auth"
	"bbuck.dev/dragon-mud/hook"
	"bbuck.dev/dragon-mud/message"
	"bbuck.dev/dragon-mud/plugin"
	"bbuck.dev/dragon-mud/scripting"
	"bbuck.dev/dragon-mud/session"
	"bbuck.dev/dragon-mud/store"
	"bbuck.dev/dragon-mud/world"
)

// scriptTimeout is how long a single script call may run before it's
// interrupted.
const scriptTimeout = 250 * time.Millisecond

// loadTimeout is how long loading every plugin may take.
const loadTimeout = 10 * time.Second

// saveTimeout is how long saving one event's changes may take.
const saveTimeout = 5 * time.Second

// hashConcurrency is how many passwords may be hashed at once by default.
// Each hash uses auth.DefaultParams.Memory while it runs.
const hashConcurrency = 4

var playerNameRx = regexp.MustCompile(`^[A-Za-z]{2,20}$`)

// event is something for the game loop to handle.
type event interface{}

type connectEvent struct{ s *session.Session }

type inputEvent struct {
	s    *session.Session
	line string
}

type disconnectEvent struct{ s *session.Session }

type reloadEvent struct{}

// player is a connected session and who it is playing as. A player is
// logging in while login is set and in the game while character is set;
// with neither, its connection is closing.
type player struct {
	s     *session.Session
	login *login

	account   store.Account
	character *world.Object
	name      string // the character's name
}

// Options configures a game.
type Options struct {
	// Name is the game's name.
	Name string

	// NewEngine returns a fresh scripting engine. The game makes a new one
	// each time it loads plugins.
	NewEngine func() scripting.Engine

	// Plugins are loaded in precedence order: built-ins first, the game's
	// own plugin last.
	Plugins []plugin.Source

	// World is every object in the game. Nil starts an empty world.
	World *world.World

	// Store holds accounts and saves the world after each event.
	Store Store

	// Hasher hashes passwords off the loop. Nil uses auth.DefaultParams,
	// a few hashes at a time.
	Hasher *auth.Hasher

	Log *slog.Logger
}

// Store holds accounts and saves the world. *store.Store implements it.
type Store interface {
	Save(ctx context.Context, changes world.Changes) error
	Account(ctx context.Context, name string) (store.Account, bool, error)
	CreateAccount(ctx context.Context, name, passwordHash string) (store.Account, error)
	SetPasswordHash(ctx context.Context, accountID, passwordHash string) error
	Characters(ctx context.Context, accountID string) ([]world.ID, error)
	AddCharacter(ctx context.Context, accountID string, id world.ID) error
}

// Game is a running game. Connect, Input, Disconnect and Reload may be
// called from any goroutine; everything else happens on the loop.
type Game struct {
	name      string
	newEngine func() scripting.Engine
	sources   []plugin.Source
	store     Store
	log       *slog.Logger

	hasher *auth.Hasher

	world *world.World

	// engine and commands are replaced together on reload: commands hold
	// functions that belong to engine.
	engine   scripting.Engine
	commands *hook.Commands

	events  chan event
	stopped chan struct{}
	players map[session.ID]*player
}

// New returns a game with its plugins loaded. The game closes its engine
// when Run returns.
func New(ctx context.Context, opts Options) (*Game, error) {
	g := &Game{
		name:      opts.Name,
		newEngine: opts.NewEngine,
		sources:   opts.Plugins,
		store:     opts.Store,
		log:       opts.Log,

		hasher:  opts.Hasher,
		world:   opts.World,
		events:  make(chan event, 1024),
		stopped: make(chan struct{}),
		players: make(map[session.ID]*player),
	}

	if g.store == nil {
		return nil, errors.New("game: no store")
	}
	if g.world == nil {
		g.world = world.New()
	}
	if g.hasher == nil {
		g.hasher = auth.NewHasher(auth.DefaultParams, hashConcurrency)
	}

	engine, commands, err := g.load(ctx)
	if err != nil {
		return nil, err
	}
	g.engine, g.commands = engine, commands

	return g, nil
}

// load loads every plugin into a new engine and command registry. On error
// the new engine is closed and the game is left as it was.
func (g *Game) load(ctx context.Context) (scripting.Engine, *hook.Commands, error) {
	ctx, cancel := context.WithTimeout(ctx, loadTimeout)
	defer cancel()

	engine := g.newEngine()
	commands := hook.NewCommands()

	err := g.loadInto(ctx, engine, commands)
	if err != nil {
		engine.Close()
		return nil, nil, err
	}

	return engine, commands, nil
}

func (g *Game) loadInto(ctx context.Context, engine scripting.Engine, commands *hook.Commands) error {
	if err := engine.Load(g.module()); err != nil {
		return err
	}

	for _, src := range g.sources {
		if err := g.loadPlugin(ctx, engine, commands, src); err != nil {
			return fmt.Errorf("%s: %w", src.Origin, err)
		}
	}

	return nil
}

func (g *Game) loadPlugin(ctx context.Context, engine scripting.Engine, commands *hook.Commands, src plugin.Source) error {
	p, err := plugin.Open(ctx, engine, src.Files, src.Builtin)
	if err != nil {
		return err
	}

	cmds, err := p.Commands(ctx, engine)
	if err != nil {
		return err
	}

	for _, cmd := range cmds {
		if err := commands.Register(cmd); err != nil {
			return err
		}
	}

	g.log.Info("loaded plugin", "plugin", p.ID, "version", p.Manifest.Version, "commands", len(cmds))

	return nil
}

// Reload asks the game to reload every plugin. If loading fails, the error
// is logged and the game keeps running the scripts it had.
func (g *Game) Reload() {
	g.post(reloadEvent{})
}

// Connect tells the game a new session has connected.
func (g *Game) Connect(s *session.Session) {
	g.post(connectEvent{s: s})
}

// Input passes a line the session typed to the game.
func (g *Game) Input(s *session.Session, line string) {
	g.post(inputEvent{s: s, line: line})
}

// Disconnect tells the game a session's connection has ended.
func (g *Game) Disconnect(s *session.Session) {
	g.post(disconnectEvent{s: s})
}

func (g *Game) post(e event) {
	select {
	case g.events <- e:
	case <-g.stopped:
	}
}

// Run runs the game loop until ctx is cancelled.
func (g *Game) Run(ctx context.Context) error {
	defer close(g.stopped)
	defer func() { g.engine.Close() }()

	for {
		select {
		case <-ctx.Done():
			for _, p := range g.players {
				p.s.Send(message.System("[Y]The server is shutting down. Farewell![x]"))
				p.s.Close()
			}
			g.save(ctx)
			return nil
		case e := <-g.events:
			g.handle(ctx, e)
			g.save(ctx)
		}
	}
}

// save writes what the last event changed. If saving fails, the changes are
// kept and retried after the next event.
func (g *Game) save(ctx context.Context) {
	changes := g.world.Changes()
	if changes.Empty() {
		return
	}

	// Save even while shutting down.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), saveTimeout)
	defer cancel()

	if err := g.store.Save(ctx, changes); err != nil {
		g.world.Unsaved(changes)
		g.log.Error("saving the world failed; will retry", "error", err)
	}
}

func (g *Game) handle(ctx context.Context, e event) {
	switch e := e.(type) {
	case connectEvent:
		p := &player{s: e.s}
		g.players[e.s.ID()] = p
		e.s.Send(message.System(fmt.Sprintf("[Y]Welcome to %s![x]", g.name)))
		g.askName(p)

	case inputEvent:
		p, ok := g.players[e.s.ID()]
		switch {
		case !ok:
		case p.login != nil:
			// Not trimmed: passwords may have spaces at either end.
			g.login(ctx, p, e.line)
		case p.character != nil:
			g.dispatch(ctx, p, strings.TrimSpace(e.line))
		}

	case checkedEvent:
		g.checked(ctx, e)

	case hashedEvent:
		g.hashed(ctx, e)

	case rehashedEvent:
		g.rehashed(ctx, e)

	case disconnectEvent:
		p, ok := g.players[e.s.ID()]
		if !ok {
			return
		}
		delete(g.players, e.s.ID())
		if p.character != nil {
			g.broadcast(message.Text(p.name+" has left."), p)
			g.log.Info("player left", "name", p.name)
		}

	case reloadEvent:
		g.reload(ctx)
	}
}

// reload swaps in freshly loaded plugins. Scripts keep no game state (see
// docs/design.md §9), so throwing the old engine away loses nothing.
func (g *Game) reload(ctx context.Context) {
	engine, commands, err := g.load(ctx)
	if err != nil {
		if !errors.Is(err, context.Canceled) {
			g.log.Error("reload failed; keeping the previous scripts", "error", err)
		}
		return
	}

	g.engine.Close()
	g.engine, g.commands = engine, commands
	g.log.Info("reloaded plugins")
}

// dispatch runs a command a player typed.
func (g *Game) dispatch(ctx context.Context, p *player, line string) {
	if line == "" {
		return
	}

	verb, args, _ := strings.Cut(line, " ")
	if strings.HasPrefix(line, "'") {
		verb, args = "say", line[1:]
	}
	args = strings.TrimSpace(args)

	cmd, ok := g.commands.Lookup(verb)
	if !ok {
		p.s.Send(message.System("Huh? Type [c]help[x] for a list of commands."))
		return
	}

	callCtx, cancel := context.WithTimeout(ctx, scriptTimeout)
	defer cancel()

	actor := map[string]any{"id": int64(p.s.ID()), "name": p.name}
	if _, err := cmd.Execute.Call(callCtx, actor, args); err != nil {
		g.log.Error("command failed", "command", cmd.Name, "plugin", cmd.Plugin, "error", err)
		p.s.Send(message.System(fmt.Sprintf("[R]%s (from %s) failed: %v[x]", cmd.Name, cmd.Plugin, err)))
	}
}

// broadcast sends m to every player in the game except skip.
func (g *Game) broadcast(m message.Message, skip *player) {
	for _, p := range g.players {
		if p != skip && p.character != nil {
			p.s.Send(m)
		}
	}
}

// playerByID returns the named player for a script-facing id.
func (g *Game) playerByID(id int) (*player, bool) {
	p, ok := g.players[session.ID(id)]
	if !ok || p.character == nil {
		return nil, false
	}

	return p, true
}
