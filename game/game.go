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
	"bbuck.dev/dragon-mud/command"
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

// player is a connected session and who it is playing as. Its input goes
// to its modes, top first, then to commands once character is set. A new
// player's only mode is the engine's login.
type player struct {
	s     *session.Session
	modes []*frame

	account   store.Account
	character *world.Object

	// closing is set while the player's modes are told it disconnected.
	closing bool
}

// displayName is the character's name property, falling back to the
// account name.
func (p *player) displayName() string {
	if p.character != nil {
		if name, ok := p.character.Get("name"); ok {
			if s, ok := name.(string); ok && s != "" {
				return s
			}
		}
	}

	return p.account.Name
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

	world       *world.World
	objType     *scripting.Type
	sessionType *scripting.Type
	accountType *scripting.Type

	// resolving is true while slot resolvers run; the world is read-only.
	resolving bool

	*scripts

	events  chan event
	stopped chan struct{}
	players map[session.ID]*player
}

// scripts is everything loaded from plugins. Its parts are replaced
// together on reload: commands and hooks hold functions that belong to
// engine.
type scripts struct {
	engine   scripting.Engine
	commands *command.Registry
	hooks    *hook.Registry
	modes    map[string]*mode

	// messages are the message kinds, and templates the other templates
	// the engine renders, such as entity tooltips.
	messages  *message.Templates
	templates *message.Templates
}

// New returns a game with its plugins loaded. The game closes its engine
// when Run returns.
func New(ctx context.Context, opts Options) (*Game, error) {
	if opts.Store == nil {
		return nil, errors.New("game: no store")
	}

	g := fromOptions(opts)
	s, err := g.load(ctx)
	if err != nil {
		return nil, err
	}
	g.scripts = s

	return g, nil
}

// Hooks loads the plugins in opts without starting a game and returns their
// hooks, as dragon hooks shows them. Only Name, NewEngine, Plugins and Log
// are used. The handlers' engine is closed, so they can't be run.
func Hooks(ctx context.Context, opts Options) (*hook.Registry, error) {
	if opts.Log == nil {
		opts.Log = slog.New(slog.DiscardHandler)
	}

	s, err := fromOptions(opts).load(ctx)
	if err != nil {
		return nil, err
	}
	s.engine.Close()

	return s.hooks, nil
}

func fromOptions(opts Options) *Game {
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

	if g.world == nil {
		g.world = world.New()
	}
	g.objType = g.objectType()
	g.sessionType = g.makeSessionType()
	g.accountType = g.makeAccountType()
	if g.hasher == nil {
		g.hasher = auth.NewHasher(auth.DefaultParams, hashConcurrency)
	}

	return g
}

// load loads every plugin into a new engine, command registry and hook
// registry. On error the new engine is closed and the game is left as it
// was.
func (g *Game) load(ctx context.Context) (*scripts, error) {
	ctx, cancel := context.WithTimeout(ctx, loadTimeout)
	defer cancel()

	s := &scripts{
		engine:    g.newEngine(),
		commands:  command.NewRegistry(),
		modes:     make(map[string]*mode),
		messages:  message.NewTemplates(),
		templates: message.NewTemplates(),
	}
	if err := g.loadInto(ctx, s); err != nil {
		s.engine.Close()
		return nil, err
	}

	return s, nil
}

func (g *Game) loadInto(ctx context.Context, s *scripts) error {
	for _, m := range []scripting.Module{g.module(), g.worldModule(), g.hooksModule(s)} {
		if err := s.engine.Load(m); err != nil {
			return err
		}
	}
	if err := s.commands.AddSlot(g.objectSlot(), false); err != nil {
		return err
	}

	var hooks hook.Config
	for _, src := range g.sources {
		if err := g.loadPlugin(ctx, s, src, &hooks); err != nil {
			return fmt.Errorf("%s: %w", src.Origin, err)
		}
	}

	if err := s.messages.Validate(plugin.MessagesDir); err != nil {
		return err
	}
	if err := s.checkModes(); err != nil {
		return err
	}

	var err error
	s.hooks, err = hook.New(hooks)

	return err
}

// loadPlugin loads src's slots and commands into s, and adds its hook
// handlers and wiring to hooks.
func (g *Game) loadPlugin(ctx context.Context, s *scripts, src plugin.Source, hooks *hook.Config) error {
	engine, commands := s.engine, s.commands

	p, err := plugin.Open(ctx, engine, src.Files, src.Builtin)
	if err != nil {
		return err
	}

	slots, err := p.Slots(ctx, engine)
	if err != nil {
		return err
	}
	for _, def := range slots {
		if err := commands.AddSlot(g.scriptSlot(p.ID, def), def.Replace); err != nil {
			return err
		}
	}

	cmds, err := p.Commands(ctx, engine)
	if err != nil {
		return err
	}
	for _, def := range cmds {
		if err := commands.Add(def); err != nil {
			return err
		}
	}

	modes, err := p.Modes(ctx, engine)
	if err != nil {
		return err
	}
	for _, def := range modes {
		if err := s.addMode(def, g.builtin); err != nil {
			return err
		}
	}

	handlers, err := p.Hooks(ctx, engine)
	if err != nil {
		return err
	}
	hooks.Plugins = append(hooks.Plugins, p.ID)
	hooks.Handlers = append(hooks.Handlers, handlers...)

	switch {
	case src.Game:
		if hooks.Wiring, err = p.Wiring(ctx, engine); err != nil {
			return err
		}
		hooks.WiringFile = p.WiringFile()
	case p.HasWiring():
		return fmt.Errorf("%s: only the game's own plugin can wire hooks. A plugin orders its handlers with before and after in hooks.lua, like before_say = { after = { \"dragon:chat\" }, handler = function(event) ... end }.",
			p.WiringFile())
	}

	kinds, err := addTemplates(p, plugin.MessagesDir, s.messages)
	if err != nil {
		return err
	}
	if _, err := addTemplates(p, plugin.TemplatesDir, s.templates); err != nil {
		return err
	}

	g.log.Info("loaded plugin", "plugin", p.ID, "version", p.Manifest.Version, "commands", len(cmds), "modes", len(modes), "slots", len(slots), "hooks", len(handlers), "messages", kinds)

	return nil
}

// builtin reports whether the plugin pluginID is embedded in the engine.
func (g *Game) builtin(pluginID string) bool {
	return strings.HasPrefix(pluginID, plugin.BuiltinPrefix)
}

// addTemplates adds the template files in the plugin's dir to templates and
// returns how many it added.
func addTemplates(p *plugin.Plugin, dir string, templates *message.Templates) (int, error) {
	files, err := p.Templates(dir)
	if err != nil {
		return 0, err
	}
	for _, f := range files {
		if err := templates.Add(f); err != nil {
			return 0, err
		}
	}

	return len(files), nil
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
			g.handleEvent(ctx, e)
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

func (g *Game) handleEvent(ctx context.Context, e event) {
	switch e := e.(type) {
	case connectEvent:
		p := &player{s: e.s}
		g.players[e.s.ID()] = p
		e.s.Send(message.System(fmt.Sprintf("[Y]Welcome to %s![x]", g.name)))
		g.askName(p)

	case inputEvent:
		if p, ok := g.players[e.s.ID()]; ok {
			g.input(ctx, p, e.line)
		}

	case requestEvent:
		if p, ok := g.players[e.s.ID()]; ok && p.character != nil {
			g.request(ctx, p, e.r)
		} else if e.r.ID != "" {
			e.s.Send(message.Message{Kind: e.r.Name, Reply: e.r.ID})
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
		p.closing = true
		g.clearModes(ctx, p, leaveDisconnected)
		delete(g.players, e.s.ID())
		if p.character != nil {
			g.log.Info("player left", "name", p.displayName())
			g.notify(ctx, "player_left", map[string]any{"player": g.handle(p.character)})
		}

	case reloadEvent:
		g.reload(ctx)
	}
}

// reload swaps in freshly loaded plugins. Scripts keep no game state (see
// docs/design.md §9), so throwing the old engine away loses nothing.
func (g *Game) reload(ctx context.Context) {
	s, err := g.load(ctx)
	if err != nil {
		if !errors.Is(err, context.Canceled) {
			g.log.Error("reload failed; keeping the previous scripts", "error", err)
		}
		return
	}

	g.engine.Close()
	g.scripts = s
	g.log.Info("reloaded plugins")
	g.pruneModes(ctx)
}
