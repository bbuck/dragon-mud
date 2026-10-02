# DragonMUD

DragonMUD is an engine for text-based multiplayer games, written in Go and
extended with Lua.

- **One binary, no setup.** Storage is SQLite; there are no external services.
- **Modern and classic clients.** Players can use a web client with a text
  feed, tooltips, health bars and action bars, or any telnet MUD client. Both
  play the same game.
- **Plugins in Lua.** Game rules come from plugins. Each plugin declares
  where its hook handlers run, and the game author can rewire them, so
  unrelated plugins layer predictably.

DragonMUD is being rebuilt from scratch. It's early: there's one room and a
handful of commands, but you can already create a game and play it from a
browser and a telnet client at the same time.

## Try it

```sh
go install ./cmds/dragon
dragon new mygame -name "The Dragon's Rest"
cd mygame
dragon serve
```

Then open http://localhost:8080, or `telnet localhost 4000`. Edit
`game/commands.lua` to add commands; changes load as soon as you save.

## Documentation

- [Vision](docs/vision.md): who it's for and what "the directory is the
  game" means.
- [Design](docs/design.md): the core (game loop, objects, hooks, messages,
  transports, scripting).
- [Plugins](docs/plugins.md): packages, extension, distribution, the web
  client API.
- [Roadmap](docs/roadmap.md): milestones, docs still to write, and what's been
  salvaged from the original engine.

## Development

```sh
go build ./...
go test ./...
go run ./cmds/dragon serve -dir path/to/game
```

Requires Go 1.26 or newer.
