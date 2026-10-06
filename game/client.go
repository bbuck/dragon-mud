package game

import (
	"context"
	"fmt"
	"regexp"
	"slices"

	"bbuck.dev/dragon-mud/message"
	"bbuck.dev/dragon-mud/plugin"
	"bbuck.dev/dragon-mud/scripting"
	"bbuck.dev/dragon-mud/session"
)

// addClient adds p's client event handlers and web files to s.
func (s *scripts) addClient(p *plugin.Plugin) error {
	defs, err := p.Client()
	if err != nil {
		return err
	}
	for _, def := range defs {
		if other, ok := s.client[def.Name]; ok {
			return fmt.Errorf("%s: %s and %s both handle the client event %s, since both plugins are named %s. Rename one of the plugins in its %s.",
				def.Path, other.Plugin, def.Plugin, def.Name, p.Namespace(), plugin.ManifestFile)
		}
		s.client[def.Name] = def
	}

	web, err := p.Web()
	if err != nil || web == nil {
		return err
	}
	for _, other := range s.web {
		if other.Namespace == web.Namespace {
			return fmt.Errorf("%s/: %s and %s both serve web files as %s/, since both plugins are named %s. Rename one of the plugins in its %s.",
				plugin.WebDir, other.Plugin, web.Plugin, web.Namespace, web.Namespace, plugin.ManifestFile)
		}
	}
	s.web = append(s.web, *web)

	return nil
}

func (g *Game) setWeb(web []plugin.Web) {
	g.webMu.Lock()
	defer g.webMu.Unlock()

	g.web = web
}

// Web returns what the game's plugins serve to the web client, in load
// order, as of the last time they loaded. It's safe to call from any
// goroutine.
func (g *Game) Web() []plugin.Web {
	g.webMu.RLock()
	defer g.webMu.RUnlock()

	return slices.Clone(g.web)
}

// clientEvent runs a plugin's handler for an event the web client sent,
// and answers it with what the handler returns when the client asked for
// an answer. Mistakes are the plugin's, so they're logged, and the client
// gets an empty answer.
func (g *Game) clientEvent(ctx context.Context, p *player, r session.Request) {
	reply := func(data any) {
		if r.ID != "" {
			p.s.Send(message.Message{Kind: r.Name, Reply: r.ID, Data: data})
		}
	}

	def, ok := g.client[r.Name]
	if !ok {
		g.log.Warn("the web client sent "+r.Name+", which no plugin handles", "event", r.Name)
		reply(nil)
		return
	}

	ctx, cancel := context.WithTimeout(ctx, scriptTimeout)
	defer cancel()

	data := r.Data
	if data == nil {
		data = map[string]any{}
	}
	value, err := def.Fn.Call(ctx, scripting.Handle{Type: g.sessionType, Key: p.s.ID()}, data)
	if err == nil {
		value, err = g.plain(value)
	}
	if err != nil {
		g.log.Error("client event failed", "event", r.Name, "error", fmt.Errorf("%s: %w", def.Path, err))
		reply(nil)
		return
	}
	reply(value)
}

// clientEventRx matches the names of events sent to the web client: a
// name after the namespace of whoever sends it, like mapping:path_found.
var clientEventRx = regexp.MustCompile(`^[a-z][a-z0-9_-]*:[a-z][a-z0-9_]*$`)

// sessionPush sends an event to the session's web client, for plugins'
// JavaScript: session:push(name, data). Objects in data become their ids.
func (g *Game) sessionPush(key any, args scripting.Args) (any, error) {
	p, err := g.sessionPlayer(key)
	if err != nil {
		return nil, err
	}
	name, err := args.String(0)
	if err != nil {
		return nil, err
	}
	if !clientEventRx.MatchString(name) {
		return nil, fmt.Errorf("%q isn't a valid client event name. Name it with your plugin's name in front, like mapping:path_found, and listen for that name with client.on in your JavaScript.", name)
	}

	var data any
	if args.Len() > 1 {
		if data, err = g.plain(args[1]); err != nil {
			return nil, fmt.Errorf("argument #2: %w", err)
		}
	}
	p.s.Send(message.ClientEvent(name, data))

	return nil, nil
}
