package game

import (
	"slices"
	"strings"

	"bbuck.dev/dragon-mud/message"
	"bbuck.dev/dragon-mud/scripting"
)

// module is the "game" scripting module. It's deliberately small; it will be
// replaced by object-based APIs (player:send) once objects exist.
//
//	game.name                      the game's name
//	game.send(id, text)            send text to one player
//	game.broadcast(text[, except]) send text to every player, optionally
//	                               skipping one
//	game.players()                 list of { id, name }, sorted by name
//	game.commands()                list of { name, desc, plugin }
//	game.disconnect(id[, text])    send an optional farewell and disconnect
func (g *Game) module() scripting.Module {
	return scripting.Module{
		Name:   "game",
		Values: map[string]any{"name": g.name},
		Funcs: map[string]scripting.Func{
			"send":       g.scriptSend,
			"broadcast":  g.scriptBroadcast,
			"players":    g.scriptPlayers,
			"commands":   g.scriptCommands,
			"disconnect": g.scriptDisconnect,
		},
	}
}

func (g *Game) scriptSend(args scripting.Args) (any, error) {
	id, err := args.Int(0)
	if err != nil {
		return nil, err
	}
	text, err := args.String(1)
	if err != nil {
		return nil, err
	}

	if p, ok := g.playerByID(id); ok {
		p.s.Send(message.Text(text))
	}

	return nil, nil
}

func (g *Game) scriptBroadcast(args scripting.Args) (any, error) {
	text, err := args.String(0)
	if err != nil {
		return nil, err
	}

	var skip *player
	if args.Len() > 1 && args[1] != nil {
		id, err := args.Int(1)
		if err != nil {
			return nil, err
		}
		skip, _ = g.playerByID(id)
	}

	g.broadcast(message.Text(text), skip)

	return nil, nil
}

func (g *Game) scriptPlayers(scripting.Args) (any, error) {
	var players []map[string]any
	for _, p := range g.players {
		if p.name != "" {
			players = append(players, map[string]any{"id": int64(p.s.ID()), "name": p.name})
		}
	}

	slices.SortFunc(players, func(a, b map[string]any) int {
		return strings.Compare(a["name"].(string), b["name"].(string))
	})

	return players, nil
}

func (g *Game) scriptCommands(scripting.Args) (any, error) {
	var commands []map[string]any
	for _, cmd := range g.commands.All() {
		commands = append(commands, map[string]any{
			"name":   cmd.Name,
			"desc":   cmd.Desc,
			"plugin": cmd.Plugin,
		})
	}

	return commands, nil
}

func (g *Game) scriptDisconnect(args scripting.Args) (any, error) {
	id, err := args.Int(0)
	if err != nil {
		return nil, err
	}

	p, ok := g.playerByID(id)
	if !ok {
		return nil, nil
	}

	if args.Len() > 1 {
		text, err := args.String(1)
		if err != nil {
			return nil, err
		}
		p.s.Send(message.Text(text))
	}

	p.s.Close()

	return nil, nil
}
