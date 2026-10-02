// Package game owns the game world. A single goroutine runs the game loop,
// taking events from a channel and handling each one to completion before
// the next. Nothing outside the loop reads or writes game state.
// See docs/design.md §2.
package game

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"bbuck.dev/dragon-mud/hook"
	"bbuck.dev/dragon-mud/message"
	"bbuck.dev/dragon-mud/plugin"
	"bbuck.dev/dragon-mud/scripting"
	"bbuck.dev/dragon-mud/session"
)

// scriptTimeout is how long a single script call may run before it's
// interrupted.
const scriptTimeout = 250 * time.Millisecond

var playerNameRx = regexp.MustCompile(`^[A-Za-z]{2,20}$`)

// event is something for the game loop to handle.
type event interface{}

type connectEvent struct{ s *session.Session }

type inputEvent struct {
	s    *session.Session
	line string
}

type disconnectEvent struct{ s *session.Session }

// player is a connected session and who it is playing as.
type player struct {
	s    *session.Session
	name string // empty until the player has chosen a name
}

// Game is a running game. Connect, Input and Disconnect may be called from
// any goroutine; everything else happens on the loop.
type Game struct {
	name     string
	engine   scripting.Engine
	commands *hook.Commands
	log      *slog.Logger

	events  chan event
	stopped chan struct{}
	players map[session.ID]*player
}

// New returns a game named name that runs scripts on engine.
func New(name string, engine scripting.Engine, log *slog.Logger) (*Game, error) {
	g := &Game{
		name:     name,
		engine:   engine,
		commands: hook.NewCommands(),
		log:      log,
		events:   make(chan event, 1024),
		stopped:  make(chan struct{}),
		players:  make(map[session.ID]*player),
	}

	if err := engine.Load(g.module()); err != nil {
		return nil, err
	}

	return g, nil
}

// LoadPlugin registers everything p provides. Plugins must be loaded in
// precedence order: built-ins first, the game's own plugin last.
func (g *Game) LoadPlugin(ctx context.Context, p *plugin.Plugin) error {
	commands, err := p.Commands(ctx, g.engine)
	if err != nil {
		return err
	}

	for _, cmd := range commands {
		if err := g.commands.Register(cmd); err != nil {
			return err
		}
	}

	g.log.Info("loaded plugin", "plugin", p.ID, "version", p.Manifest.Version, "commands", len(commands))

	return nil
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

	for {
		select {
		case <-ctx.Done():
			for _, p := range g.players {
				p.s.Send(message.System("[Y]The server is shutting down. Farewell![x]"))
				p.s.Close()
			}
			return nil
		case e := <-g.events:
			g.handle(ctx, e)
		}
	}
}

func (g *Game) handle(ctx context.Context, e event) {
	switch e := e.(type) {
	case connectEvent:
		g.players[e.s.ID()] = &player{s: e.s}
		e.s.Send(message.System(fmt.Sprintf("[Y]Welcome to %s![x]\nBy what name shall we know you?", g.name)))

	case inputEvent:
		p, ok := g.players[e.s.ID()]
		if !ok {
			return
		}
		if p.name == "" {
			g.login(ctx, p, strings.TrimSpace(e.line))
			return
		}
		g.dispatch(ctx, p, strings.TrimSpace(e.line))

	case disconnectEvent:
		p, ok := g.players[e.s.ID()]
		if !ok {
			return
		}
		delete(g.players, e.s.ID())
		if p.name != "" {
			g.broadcast(message.Text(p.name+" has left."), p)
			g.log.Info("player left", "name", p.name)
		}
	}
}

// login handles input from a session that hasn't chosen a name yet.
func (g *Game) login(ctx context.Context, p *player, name string) {
	if !playerNameRx.MatchString(name) {
		p.s.Send(message.System("Names are 2 to 20 letters. Try again:"))
		return
	}

	name = strings.ToUpper(name[:1]) + strings.ToLower(name[1:])
	for _, other := range g.players {
		if strings.EqualFold(other.name, name) {
			p.s.Send(message.System("Someone by that name is already here. Try another:"))
			return
		}
	}

	p.name = name
	g.log.Info("player arrived", "name", name)
	p.s.Send(message.System(fmt.Sprintf("Welcome, [W]%s[x]! Type [c]help[x] to see what you can do.", name)))
	g.broadcast(message.Text(name+" has arrived."), p)

	if _, ok := g.commands.Lookup("look"); ok {
		g.dispatch(ctx, p, "look")
	}
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

// broadcast sends m to every named player except skip.
func (g *Game) broadcast(m message.Message, skip *player) {
	for _, p := range g.players {
		if p != skip && p.name != "" {
			p.s.Send(m)
		}
	}
}

// playerByID returns the named player for a script-facing id.
func (g *Game) playerByID(id int) (*player, bool) {
	p, ok := g.players[session.ID(id)]
	if !ok || p.name == "" {
		return nil, false
	}

	return p, true
}
