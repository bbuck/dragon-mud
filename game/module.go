package game

import (
	"fmt"
	"slices"
	"strings"

	"bbuck.dev/dragon-mud/message"
	"bbuck.dev/dragon-mud/scripting"
	"bbuck.dev/dragon-mud/world"
)

// module is the "game" scripting module. Players are their character
// objects; send to one with player:send(text).
//
//	game.name                      the game's name
//	game.broadcast(text[, except]) send text to every player, optionally
//	                               skipping a player object or a list of
//	                               them
//	game.broadcast(kind, data[, block[, except]])
//	                               the same for a message kind
//	game.players()                 list of player objects, sorted by name
//	game.commands()                list of { name, desc, plugin, forms },
//	                               where forms is a list of { pattern, desc }
//	game.disconnect(player[, text]) send an optional farewell and disconnect
func (g *Game) module() scripting.Module {
	return scripting.Module{
		Name:   "game",
		Values: map[string]any{"name": g.name},
		Funcs: map[string]scripting.Func{
			"broadcast":  g.mutatingFunc(g.scriptBroadcast),
			"players":    g.scriptPlayers,
			"commands":   g.scriptCommands,
			"disconnect": g.mutatingFunc(g.scriptDisconnect),
		},
	}
}

func (g *Game) scriptBroadcast(args scripting.Args) (any, error) {
	m, used, err := g.outgoing(args)
	if err != nil {
		return nil, err
	}

	except, err := g.objectsArg(args, used)
	if err != nil {
		return nil, err
	}

	for _, p := range g.players {
		if p.character != nil && !slices.Contains(except, p.character) {
			p.s.Send(m)
		}
	}

	return nil, nil
}

// objectsArg returns argument i as a list of objects: nil, one object, or
// a list of them.
func (g *Game) objectsArg(args scripting.Args, i int) ([]*world.Object, error) {
	var list []any
	if i < args.Len() {
		list, _ = args[i].([]any)
	}
	if list == nil {
		o, err := g.optionalObjectArg(args, i)
		if err != nil || o == nil {
			return nil, err
		}
		return []*world.Object{o}, nil
	}

	objects := make([]*world.Object, len(list))
	for j, item := range list {
		h, ok := item.(scripting.Handle)
		if !ok || h.Type != g.objType {
			return nil, fmt.Errorf("argument #%d: item %d: expected object, got %s", i+1, j+1, scripting.TypeName(item))
		}
		o, err := g.object(h.Key)
		if err != nil {
			return nil, fmt.Errorf("argument #%d: item %d: %w", i+1, j+1, err)
		}
		objects[j] = o
	}

	return objects, nil
}

func (g *Game) scriptPlayers(scripting.Args) (any, error) {
	var playing []*player
	for _, p := range g.players {
		if p.character != nil {
			playing = append(playing, p)
		}
	}

	slices.SortFunc(playing, func(a, b *player) int {
		return strings.Compare(a.displayName(), b.displayName())
	})

	players := make([]any, len(playing))
	for i, p := range playing {
		players[i] = g.handle(p.character)
	}

	return players, nil
}

func (g *Game) scriptCommands(scripting.Args) (any, error) {
	var commands []map[string]any
	for _, cmd := range g.commands.All() {
		var forms []map[string]any
		seen := make(map[string]bool)
		for _, f := range cmd.Forms {
			// Optional groups expand into several patterns from one source.
			if !seen[f.Pattern.Source] {
				seen[f.Pattern.Source] = true
				forms = append(forms, map[string]any{"pattern": f.Pattern.Source, "desc": f.Desc})
			}
		}
		commands = append(commands, map[string]any{
			"name":   cmd.Name,
			"desc":   cmd.Desc,
			"plugin": cmd.Plugin,
			"forms":  forms,
		})
	}

	return commands, nil
}

func (g *Game) scriptDisconnect(args scripting.Args) (any, error) {
	o, err := g.objectArg(args, 0)
	if err != nil {
		return nil, err
	}

	var text string
	if args.Len() > 1 {
		if text, err = args.String(1); err != nil {
			return nil, err
		}
	}

	for _, p := range g.players {
		if p.character == o {
			if text != "" {
				p.s.Send(message.Text(text))
			}
			p.s.Close()
		}
	}

	return nil, nil
}
