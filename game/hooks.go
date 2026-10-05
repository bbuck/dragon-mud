package game

import (
	"context"
	"errors"

	"bbuck.dev/dragon-mud/scripting"
)

// hooksModule is the "hooks" scripting module, for plugins to run their own
// hooks and notifications. It uses the hooks loaded into s, so scripts
// always reach the handlers that share their engine.
//
//	hooks.run(name[, event])     run a hook's handlers on event, returning
//	                             the event as they left it, or nil and the
//	                             reason one cancelled
//	hooks.notify(name[, event])  tell every handler of a notification;
//	                             failures are logged, not raised
func (g *Game) hooksModule(s *scripts) scripting.Module {
	args := func(args scripting.Args) (string, map[string]any, error) {
		if s.hooks == nil {
			return "", nil, errors.New("hooks can't run while plugins are loading; run them from a command or another handler")
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
		Name: "dragon.hooks",
		Funcs: map[string]scripting.Func{
			"run": func(a scripting.Args) (any, error) {
				name, event, err := args(a)
				if err != nil {
					return nil, err
				}

				// Called from a running script, whose deadline applies.
				result, err := s.hooks.Run(context.Background(), name, event)
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

				if err := s.hooks.Notify(context.Background(), name, event); err != nil {
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

	if err := g.hooks.Notify(ctx, name, event); err != nil {
		g.log.Error("notification failed", "notification", name, "error", err)
	}
}
