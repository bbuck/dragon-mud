package game

import (
	"context"
	"fmt"
	"strings"

	"bbuck.dev/dragon-mud/message"
	"bbuck.dev/dragon-mud/scripting"
	"bbuck.dev/dragon-mud/session"
	"bbuck.dev/dragon-mud/view"
	"bbuck.dev/dragon-mud/world"
)

// Requests the web client makes about a <dragon-entity>. See
// docs/design.md §5.
const (
	// requestTooltip asks for an entity's tooltip; the reply is its HTML,
	// or empty for none.
	requestTooltip = "entity_tooltip"

	// requestAction runs an entity's default action.
	requestAction = "entity_action"
)

// Hooks the engine runs for entities. Both get viewer and entity.
const (
	// hookTooltip may set event.block to render one block of the tooltip
	// template, add data for it, or cancel for no tooltip.
	hookTooltip = "get_tooltip"

	// hookAction may set event.command to what clicking the entity runs.
	hookAction = "get_default_action"
)

// tooltipTemplate is the template in a plugin's templates/ directory that
// renders entity tooltips. A text version works too: the web shows text
// templates as HTML.
const tooltipTemplate = "entity_tooltip"

type requestEvent struct {
	s *session.Session
	r session.Request
}

// Request passes something the session's client asked for to the game.
func (g *Game) Request(s *session.Session, r session.Request) {
	g.post(requestEvent{s: s, r: r})
}

// request answers a client request from a player in the game. Requests
// that want a reply always get one, even if it's empty.
func (g *Game) request(ctx context.Context, p *player, r session.Request) {
	ref, _ := r.Data["ref"].(string)
	o, _ := g.world.Get(world.ID(ref))

	switch r.Name {
	case requestTooltip:
		var html string
		if o != nil {
			html = g.tooltip(ctx, p, o)
		}
		p.s.Send(message.Message{Kind: requestTooltip, HTML: html, Reply: r.ID})

	case requestAction:
		if r.ID != "" {
			p.s.Send(message.Message{Kind: requestAction, Reply: r.ID})
		}
		if o == nil {
			return
		}
		if command := g.defaultAction(ctx, p, o); command != "" {
			p.s.Send(message.Echo(command))
			g.dispatch(ctx, p, command)
		}

	default:
		g.log.Debug("unknown client request", "request", r.Name)
		if r.ID != "" {
			p.s.Send(message.Message{Kind: r.Name, Reply: r.ID})
		}
	}
}

// tooltip renders o's tooltip for p, or returns "" when there's none.
// Mistakes in the hook or template are the game author's, so they're
// logged rather than shown to the player.
func (g *Game) tooltip(ctx context.Context, p *player, o *world.Object) string {
	if !g.templates.Has(tooltipTemplate) {
		return ""
	}

	event, ok := g.entityHook(ctx, hookTooltip, p, o)
	if !ok {
		return ""
	}

	block, err := hookString(event, hookTooltip, "block", "a block name in the tooltip template")
	if err != nil {
		g.log.Error("tooltip failed", "error", err)
		return ""
	}

	data, err := g.templateData(event, "")
	if err != nil {
		g.log.Error("tooltip failed", "error", fmt.Errorf("%s: event.%w", hookTooltip, err))
		return ""
	}

	html, _, err := g.templates.Render(tooltipTemplate, view.FormatHTML, block, data)
	if err != nil {
		g.log.Error("tooltip failed", "error", err)
		return ""
	}

	return html
}

// defaultAction returns the command clicking o runs for p, or "".
func (g *Game) defaultAction(ctx context.Context, p *player, o *world.Object) string {
	event, ok := g.entityHook(ctx, hookAction, p, o)
	if !ok {
		return ""
	}

	command, err := hookString(event, hookAction, "command", `a command to run, like "look #" .. event.entity.id`)
	if err != nil {
		g.log.Error("default action failed", "error", err)
		return ""
	}

	return strings.TrimSpace(command)
}

// entityHook runs the hook name with viewer and entity, returning the
// event the handlers left, or false if one cancelled or failed.
func (g *Game) entityHook(ctx context.Context, name string, p *player, o *world.Object) (map[string]any, bool) {
	ctx, cancel := context.WithTimeout(ctx, scriptTimeout)
	defer cancel()

	result, err := g.hooks.Run(ctx, name, map[string]any{
		"viewer": g.handle(p.character),
		"entity": g.handle(o),
	})
	if err != nil {
		g.log.Error("hook failed", "hook", name, "error", err)
		return nil, false
	}

	return result.Payload, !result.Cancelled
}

// hookString reads the optional string field from a hook's event.
func hookString(event map[string]any, hook, field, want string) (string, error) {
	value, ok := event[field]
	if !ok || value == nil {
		return "", nil
	}

	s, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("%s: event.%s must be %s, not a %s", hook, field, want, scripting.TypeName(value))
	}

	return s, nil
}
