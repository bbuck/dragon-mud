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
const hookUnmatched = "dragon:unmatched_input"

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

	ran, miss, err := g.runCommand(ctx, g.handle(p.character), line)
	switch {
	case err != nil:
		g.log.Error("command failed", "input", line, "error", err)
		p.s.Send(message.System(fmt.Sprintf("[R]%v[x]", err)))
	case !ran:
		p.s.Send(message.System(miss))
	}
}

// runCommand runs line as if actor typed it: the most specific form that
// matches, or else the dragon:unmatched_input hook. When nothing ran, miss
// is what the actor would be told.
func (g *Game) runCommand(ctx context.Context, actor any, line string) (ran bool, miss string, err error) {
	g.resolving = true
	match, noMatch, err := g.commands.Parse(ctx, actor, line)
	g.resolving = false
	if err != nil {
		return false, "", err
	}

	if noMatch != nil {
		if g.unmatched(ctx, actor, line, noMatch) {
			return true, "", nil
		}
		return false, missMessage(noMatch), nil
	}

	form := match.Form
	if _, err := form.Execute.Call(ctx, actor, match.Args); err != nil {
		return false, "", fmt.Errorf("%s (%q from %s) failed: %w", form.Command.Name, form.Pattern.Source, form.Plugin, err)
	}

	return true, "", nil
}

// missMessage is what a player is told when nothing matched their input.
func missMessage(miss *command.NoMatch) string {
	switch {
	case miss.Reason != "":
		return miss.Reason
	case len(miss.Usage) > 0:
		return "Usage:\n  [c]" + strings.Join(miss.Usage, "[x]\n  [c]") + "[x]"
	default:
		return "Huh? Type [c]help[x] for a list of commands."
	}
}

// unmatched offers input no command matched to the dragon:unmatched_input hook,
// returning true if a handler dealt with it.
func (g *Game) unmatched(ctx context.Context, actor any, line string, miss *command.NoMatch) bool {
	event := map[string]any{"actor": actor, "line": line}
	if miss.Reason != "" {
		event["reason"] = miss.Reason
	}

	result, err := g.events.Run(ctx, hookUnmatched, event)
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
// The resolver is called as resolve(actor, text, modifiers, requirements)
// and returns the value, or nil and a reason the player will see.
// modifiers is the set of modifiers the slot names, { open = true }, which
// is all most slot types need. requirements is how they combine: a list
// of requirements that must all hold, each a list of alternatives, so
// here|held,online is { { "here", "held" }, { "online" } }.
func (g *Game) scriptSlot(pluginID string, def plugin.SlotDef) command.SlotType {
	return command.SlotType{
		Name:      def.Name,
		Desc:      def.Desc,
		Plugin:    pluginID,
		Modifiers: def.Modifiers,
		Single:    def.Single,
		Resolve: func(ctx context.Context, actor any, text string, modifiers command.Modifiers) (any, bool, string, error) {
			named := make(map[string]any)
			for _, m := range modifiers.Names() {
				named[m] = true
			}
			reqs := make([]any, len(modifiers))
			for i, alts := range modifiers {
				reqs[i] = slices.Clone(alts)
			}

			results, err := def.Resolve.CallAll(ctx, actor, text, named, reqs)
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

// objectSlot is the built-in "object" slot type. Its modifiers say where an
// object may be:
//
//	here      in the actor's location
//	held      in the actor's inventory (its contents)
//	online    a character someone is playing
//	anywhere  any object; by name only through the other modifiers, but by
//	          key or id on its own (for builders)
//
// Modifiers combine as patterns write them: here|held is either, and
// here,online is both, so an online player in the room. With no modifiers
// it's here|held. "me", "self" and the actor's own #id are always the
// actor. Objects match by
// their name property or any of their aliases; "2.sword" picks the second
// match. "#id" picks exactly the object with that id, if the modifiers
// allow it; the web client sends these when players click things.
func (g *Game) objectSlot() command.SlotType {
	return command.SlotType{
		Name:      "object",
		Desc:      "An object, found by name.",
		Plugin:    "engine",
		Modifiers: []string{"here", "held", "online", "anywhere"},
		Resolve: func(_ context.Context, actor any, text string, mods command.Modifiers) (any, bool, string, error) {
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

			if len(mods) == 0 {
				mods = command.Modifiers{{"here", "held"}}
			}
			allowed := func(o *world.Object) bool {
				for _, alts := range mods {
					if !slices.ContainsFunc(alts, func(m string) bool { return g.objectIs(m, self, o) }) {
						return false
					}
				}
				return true
			}

			id, byID := strings.CutPrefix(text, "#")
			if byID && world.ID(id) == self.ID() {
				// Clicking your own name is "me".
				return g.handle(self), true, "", nil
			}
			if byID {
				if o, ok := g.world.Get(world.ID(id)); ok && allowed(o) {
					return g.handle(o), true, "", nil
				}
				return nil, false, notFoundByID(mods), nil
			}
			if mods.Has("anywhere") {
				if o, ok := g.world.Keyed(text); ok && allowed(o) {
					return g.handle(o), true, "", nil
				}
				if o, ok := g.world.Get(world.ID(text)); ok && allowed(o) {
					return g.handle(o), true, "", nil
				}
			}

			var candidates []*world.Object
			for _, o := range g.objectCandidates(mods, self) {
				if allowed(o) {
					candidates = append(candidates, o)
				}
			}

			o, reason := pick(candidates, text)
			if o == nil {
				return nil, false, notFound(text, mods, reason), nil
			}

			return g.handle(o), true, "", nil
		},
	}
}

// objectIs reports whether o is where the object slot modifier m says,
// for actor self.
func (g *Game) objectIs(m string, self, o *world.Object) bool {
	switch m {
	case "here":
		return o != self && self.Location() != nil && o.Location() == self.Location()
	case "held":
		return o.Location() == self
	case "online":
		for _, p := range g.players {
			if p.character == o {
				return true
			}
		}
		return false
	case "anywhere":
		return true
	}

	return false
}

// objectCandidates lists the objects to look through by name: those of the
// first requirement that names only places it can list. anywhere can't be
// listed, so a slot that's only anywhere finds objects by key or id alone.
func (g *Game) objectCandidates(mods command.Modifiers, self *world.Object) []*world.Object {
	for _, alts := range mods {
		if slices.Contains(alts, "anywhere") {
			continue
		}

		var list []*world.Object
		add := func(o *world.Object) {
			if !slices.Contains(list, o) {
				list = append(list, o)
			}
		}
		for _, m := range alts {
			switch m {
			case "here":
				if loc := self.Location(); loc != nil {
					for _, o := range loc.Contents() {
						if o != self {
							add(o)
						}
					}
				}
			case "held":
				for _, o := range self.Contents() {
					add(o)
				}
			case "online":
				for _, p := range g.players {
					if p.character != nil {
						add(p.character)
					}
				}
			}
		}
		return list
	}

	return nil
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

func notFound(text string, mods command.Modifiers, reason string) string {
	switch {
	case reason != "":
		return reason
	case mods.Only("online"):
		return fmt.Sprintf("No one called '%s' is playing right now.", text)
	case mods.Only("held"):
		return fmt.Sprintf("You aren't carrying '%s'.", text)
	default:
		return fmt.Sprintf("You don't see '%s' here.", text)
	}
}

// notFoundByID is notFound for "#id", which means nothing to players, so
// it isn't repeated back to them.
func notFoundByID(mods command.Modifiers) string {
	switch {
	case mods.Only("online"):
		return "They aren't playing right now."
	case mods.Only("held"):
		return "You aren't carrying that."
	default:
		return "You don't see that here."
	}
}
