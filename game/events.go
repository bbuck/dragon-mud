package game

import (
	"context"
	"errors"

	"bbuck.dev/dragon-mud/event"
	"bbuck.dev/dragon-mud/scripting"
)

// Notifications the engine sends about players.
const (
	notifyConnected    = "dragon:player_connected"
	notifyDisconnected = "dragon:player_disconnected"
)

// engineEvents declares the events the engine sends itself. Plugins
// declare theirs in events.declare.
var engineEvents = []event.Decl{
	{
		Name: notifyBooted,
		Desc: "The server has started, before anyone can type: the place to make sure the world has what the game needs. Not sent on reload.",
	},
	{
		Name: notifyConnected,
		Desc: "A player has entered the game.",
		Fields: []event.Field{
			{Name: "actor", Desc: "the character they play"},
			{Name: "reconnected", Desc: "true when they took over their character from another connection, so to everyone else they never left"},
		},
	},
	{
		Name: notifyDisconnected,
		Desc: "A player has left the game.",
		Fields: []event.Field{
			{Name: "actor", Desc: "the character they played"},
		},
	},
	{
		Name: hookUnmatched,
		Desc: "Input no command matched, offered to plugins before the player is told why. A handler that deals with the line sets event.handled = true and returns the event.",
		Fields: []event.Field{
			{Name: "actor", Desc: "who typed it"},
			{Name: "line", Desc: "what they typed"},
			{Name: "reason", Desc: "why the nearest command didn't match, when one nearly did", Optional: true},
			{Name: "handled", Desc: "set to true by a handler that dealt with the line", Optional: true},
		},
	},
	{
		Name: hookTooltip,
		Desc: "A player in the web client wants an entity's tooltip, rendered from templates/entity_tooltip. Cancel for no tooltip.",
		Fields: []event.Field{
			{Name: "viewer", Desc: "the player looking"},
			{Name: "entity", Desc: "the thing they're looking at"},
			{Name: "block", Desc: "set to render one block of the tooltip template", Optional: true},
		},
		Extra: "data for the tooltip template",
	},
	{
		Name: hookAction,
		Desc: "A player in the web client clicked an entity. Set event.command to what that runs, as if they typed it.",
		Fields: []event.Field{
			{Name: "viewer", Desc: "the player clicking"},
			{Name: "entity", Desc: "the thing they clicked"},
			{Name: "command", Desc: `set to the command to run, like "look #" .. event.entity.id`, Optional: true},
		},
	},
	{
		Name:   "section:",
		Prefix: true,
		Desc:   "Fills a section of a view: section:<view>.<section>. Handlers add to event.parts.",
		Fields: []event.Field{
			{Name: "data", Desc: "the data the view was sent with"},
			{Name: "parts", Desc: `what handlers add: text, or a table like { view = "minimap", data = { ... } }`},
		},
	},
}

// eventsModule is the "dragon.events" scripting module, for plugins to
// send their own events. It uses the events loaded into s, so scripts
// always reach the handlers that share their engine.
//
//	events.run(name[, event])     run a hook's handlers on event, returning
//	                              the event as they left it, or nil and the
//	                              reason one cancelled
//	events.notify(name[, event])  tell every handler of a notification;
//	                              failures are logged, not raised
func (g *Game) eventsModule(s *scripts) scripting.Module {
	args := func(args scripting.Args) (string, map[string]any, error) {
		if s.events == nil {
			return "", nil, errors.New("events can't be sent while plugins are loading; send them from a command or a handler")
		}

		name, err := args.String(0)
		if err != nil {
			return "", nil, err
		}

		var event map[string]any
		if args.Len() > 1 && args[1] != nil {
			if list, ok := args[1].([]any); ok && len(list) == 0 {
				event = map[string]any{}
			} else if event, err = args.Map(1); err != nil {
				return "", nil, err
			}
		}

		return name, event, nil
	}

	return scripting.Module{
		Name: "dragon.events",
		Funcs: map[string]scripting.Func{
			"run": func(a scripting.Args) (any, error) {
				name, event, err := args(a)
				if err != nil {
					return nil, err
				}

				// Called from a running script, whose deadline applies.
				result, err := s.events.Run(context.Background(), name, event)
				if err != nil {
					return nil, err
				}
				if result.Cancelled {
					var reason any
					if result.Reason != "" {
						reason = result.Reason
					}
					return scripting.Results{nil, reason}, nil
				}

				return result.Payload, nil
			},
			"notify": func(a scripting.Args) (any, error) {
				name, event, err := args(a)
				if err != nil {
					return nil, err
				}

				// A mistake in the event is the caller's, so it's raised;
				// handlers failing is theirs, so it's logged.
				if err := s.events.Check(name, event); err != nil {
					return nil, err
				}
				if err := s.events.Notify(context.Background(), name, event); err != nil {
					g.log.Error("notification failed", "notification", name, "error", err)
				}

				return nil, nil
			},
		},
	}
}

// notify tells the handlers of the notification name what happened.
// Failures are logged.
func (g *Game) notify(ctx context.Context, name string, event map[string]any) {
	ctx, cancel := context.WithTimeout(ctx, scriptTimeout)
	defer cancel()

	if err := g.events.Notify(ctx, name, event); err != nil {
		g.log.Error("notification failed", "notification", name, "error", err)
	}
}
