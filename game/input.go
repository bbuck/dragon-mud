package game

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"bbuck.dev/dragon-mud/command"
	"bbuck.dev/dragon-mud/message"
	"bbuck.dev/dragon-mud/plugin"
	"bbuck.dev/dragon-mud/scripting"
	"bbuck.dev/dragon-mud/world"
)

// hookUnmatched runs when no command matches what a player typed, with
// actor, line and the near miss's reason. A handler that deals with the
// line sets event.handled = true and returns the event; otherwise the
// player sees why nothing matched.
const hookUnmatched = "unmatched_input"

// errResolving is returned when a slot resolver tries to change the world.
// Resolvers run speculatively for every form that might match, so they may
// only look.
var errResolving = errors.New("can't change the world while resolving input; slot resolvers may only look things up")

// writable reports whether scripts may change the world right now.
func (g *Game) writable() error {
	if g.resolving {
		return errResolving
	}

	return nil
}

// dispatch runs what a player typed.
func (g *Game) dispatch(ctx context.Context, p *player, line string) {
	if strings.TrimSpace(line) == "" {
		return
	}

	ctx, cancel := context.WithTimeout(ctx, scriptTimeout)
	defer cancel()

	actor := g.handle(p.character)

	g.resolving = true
	match, miss, err := g.commands.Parse(ctx, actor, line)
	g.resolving = false

	if miss != nil && g.unmatched(ctx, actor, line, miss) {
		return
	}

	switch {
	case err != nil:
		g.log.Error("resolving input failed", "input", line, "error", err)
		p.s.Send(message.System(fmt.Sprintf("[R]%v[x]", err)))

	case miss != nil && miss.Reason != "":
		p.s.Send(message.System(miss.Reason))

	case miss != nil && len(miss.Usage) > 0:
		p.s.Send(message.System("Usage:\n  [c]" + strings.Join(miss.Usage, "[x]\n  [c]") + "[x]"))

	case miss != nil:
		p.s.Send(message.System("Huh? Type [c]help[x] for a list of commands."))

	default:
		form := match.Form
		if _, err := form.Execute.Call(ctx, actor, match.Args); err != nil {
			g.log.Error("command failed", "command", form.Command.Name, "form", form.Pattern.Source, "plugin", form.Plugin, "error", err)
			p.s.Send(message.System(fmt.Sprintf("[R]%s (%q from %s) failed: %v[x]", form.Command.Name, form.Pattern.Source, form.Plugin, err)))
		}
	}
}

// unmatched offers input no command matched to the unmatched_input hook,
// returning true if a handler dealt with it.
func (g *Game) unmatched(ctx context.Context, actor any, line string, miss *command.NoMatch) bool {
	event := map[string]any{"actor": actor, "line": line}
	if miss.Reason != "" {
		event["reason"] = miss.Reason
	}

	result, err := g.hooks.Run(ctx, hookUnmatched, event)
	if err != nil {
		g.log.Error("hook failed", "hook", hookUnmatched, "input", line, "error", err)
		return false
	}
	if result.Cancelled {
		return false
	}

	handled, _ := result.Payload["handled"].(bool)

	return handled
}

// scriptSlot adapts a slot type a plugin wrote in its scripting language.
// The resolver is called as resolve(actor, text, modifiers) and returns the
// value, or nil and a reason the player will see.
func (g *Game) scriptSlot(pluginID string, def plugin.SlotDef) command.SlotType {
	return command.SlotType{
		Name:      def.Name,
		Desc:      def.Desc,
		Plugin:    pluginID,
		Modifiers: def.Modifiers,
		Single:    def.Single,
		Resolve: func(ctx context.Context, actor any, text string, modifiers map[string]bool) (any, bool, string, error) {
			mods := make(map[string]any, len(modifiers))
			for m := range modifiers {
				mods[m] = true
			}

			results, err := def.Resolve.CallAll(ctx, actor, text, mods)
			if err != nil {
				return nil, false, "", err
			}

			if len(results) > 0 && results[0] != nil {
				return results[0], true, "", nil
			}

			reason := fmt.Sprintf("'%s' isn't a valid %s.", text, def.Name)
			if len(results) > 1 {
				if s, ok := results[1].(string); ok && s != "" {
					reason = s
				}
			}
			return nil, false, reason, nil
		},
	}
}

// objectSlot is the built-in "object" slot type. Its modifiers say where to
// look:
//
//	here      in the actor's location
//	held      in the actor's inventory (its contents)
//	online    players in the game
//	anywhere  any object, by key or id (for builders)
//
// With no modifiers it looks here and held. "me" and "self" are always the
// actor. Objects match by their name property or any of their aliases;
// "2.sword" picks the second match. "#id" picks exactly the object with that
// id, if it's somewhere the modifiers look; the web client sends these when
// players click things.
func (g *Game) objectSlot() command.SlotType {
	return command.SlotType{
		Name:      "object",
		Desc:      "An object, found by name.",
		Plugin:    "engine",
		Modifiers: []string{"here", "held", "online", "anywhere"},
		Resolve: func(_ context.Context, actor any, text string, mods map[string]bool) (any, bool, string, error) {
			h, ok := actor.(scripting.Handle)
			if !ok {
				return nil, false, "", errors.New("object slots only work for players in the game, so a mode that runs before session:play can't use them")
			}
			self, err := g.object(h.Key)
			if err != nil {
				return nil, false, "", err
			}

			if strings.EqualFold(text, "me") || strings.EqualFold(text, "self") {
				return g.handle(self), true, "", nil
			}

			id, byID := strings.CutPrefix(text, "#")
			if mods["anywhere"] {
				if o, ok := g.world.Get(world.ID(id)); ok && byID {
					return g.handle(o), true, "", nil
				}
				if o, ok := g.world.Keyed(text); ok {
					return g.handle(o), true, "", nil
				}
				if o, ok := g.world.Get(world.ID(text)); ok {
					return g.handle(o), true, "", nil
				}
			}

			var candidates []*world.Object
			if len(mods) == 0 || mods["here"] {
				if loc := self.Location(); loc != nil {
					for _, o := range loc.Contents() {
						if o != self {
							candidates = append(candidates, o)
						}
					}
				}
			}
			if len(mods) == 0 || mods["held"] {
				candidates = append(candidates, self.Contents()...)
			}
			if mods["online"] {
				for _, p := range g.players {
					if p.character != nil && !slices.Contains(candidates, p.character) {
						candidates = append(candidates, p.character)
					}
				}
			}

			if byID {
				for _, o := range candidates {
					if o.ID() == world.ID(id) {
						return g.handle(o), true, "", nil
					}
				}
				return nil, false, notFoundByID(mods), nil
			}

			o, reason := pick(candidates, text)
			if o == nil {
				return nil, false, notFound(text, mods, reason), nil
			}

			return g.handle(o), true, "", nil
		},
	}
}

// pick finds the object text names among candidates. "2.sword" picks the
// second match. More than one match without a number is ambiguous.
func pick(candidates []*world.Object, text string) (*world.Object, string) {
	nth := 0
	if before, after, ok := strings.Cut(text, "."); ok {
		if n, err := strconv.Atoi(before); err == nil && n > 0 {
			nth, text = n, after
		}
	}

	var matches []*world.Object
	for _, o := range candidates {
		if names(o, text) {
			matches = append(matches, o)
		}
	}

	switch {
	case len(matches) == 0:
		return nil, ""
	case nth > 0 && nth <= len(matches):
		return matches[nth-1], ""
	case nth > 0:
		return nil, fmt.Sprintf("There are only %d of '%s'.", len(matches), text)
	case len(matches) > 1:
		return nil, fmt.Sprintf("Which '%s' do you mean? There are %d; say 1.%s, 2.%s and so on.", text, len(matches), text, text)
	default:
		return matches[0], ""
	}
}

// names reports whether text names o: its name property or one of its
// aliases, ignoring case, or the start of its name.
func names(o *world.Object, text string) bool {
	text = strings.ToLower(text)

	if name, ok := o.Get("name"); ok {
		if s, ok := name.(string); ok {
			s = strings.ToLower(s)
			if s == text || strings.HasPrefix(s, text) {
				return true
			}
		}
	}

	if aliases, ok := o.Get("aliases"); ok {
		if list, ok := aliases.([]any); ok {
			for _, a := range list {
				if s, ok := a.(string); ok && strings.ToLower(s) == text {
					return true
				}
			}
		}
	}

	return false
}

func notFound(text string, mods map[string]bool, reason string) string {
	switch {
	case reason != "":
		return reason
	case mods["online"] && len(mods) == 1:
		return fmt.Sprintf("No one called '%s' is playing right now.", text)
	case mods["held"] && len(mods) == 1:
		return fmt.Sprintf("You aren't carrying '%s'.", text)
	default:
		return fmt.Sprintf("You don't see '%s' here.", text)
	}
}

// notFoundByID is notFound for "#id", which means nothing to players, so
// it isn't repeated back to them.
func notFoundByID(mods map[string]bool) string {
	switch {
	case mods["online"] && len(mods) == 1:
		return "They aren't playing right now."
	case mods["held"] && len(mods) == 1:
		return "You aren't carrying that."
	default:
		return "You don't see that here."
	}
}
