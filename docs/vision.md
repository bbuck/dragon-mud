# Vision

DragonMUD is for people who want to **build text games, not write MUD
engines**.

A game is a directory. You create one with `dragon new`, run `dragon serve`
inside it, and it works. You never clone or fork the engine; you install it,
like any other tool, and upgrade it the same way.

```sh
dragon new mygame --kit diku
cd mygame
dragon serve
# open http://localhost:8080 or telnet localhost 4000
```

## Who it's for

- **Game builders** who write Lua, HTML templates and maybe a little
  JavaScript. They never write Go.
- **Plugin authors** who package features (rooms, combat, automapping) for
  other games to install with `dragon add`.
- **Players** on a modern web client (feed, windows, health bars, action bars,
  tooltips) or any classic telnet MUD client, playing the same game.

Advanced users can build a custom engine binary with extra Go extensions
(see [plugins.md](plugins.md#go-extensions)), but nothing in the normal path
requires it.

## Principles

- **The directory is the game.** Config, the game's own code, installed
  plugins and the lockfile all live in it and go into version control.
- **Everything is a plugin.** Rooms, items, combat, maps, even `look` and
  `say` are plugins. The core assumes only sessions, objects that can contain
  other objects, scripting, and messages. A Diku-style hack-and-slash, a MUSH
  or a MOO can all be built on it.
- **The plugin API is the product.** Game builders can only do what plugins
  can do. Built-in plugins use the same public API as everyone else.
- **Predictable composition.** Plugins declare their order and dependencies;
  the game can rewire anything. Nothing depends on directory order or timing.
- **Lua conventions.** Every plugin file is a module that returns a table; the
  engine registers what it returns. No global DSLs.
- **Just HTML.** The web client is server-rendered with htmx. Plugins provide
  HTML templates and, when they need it, plain JavaScript modules. There is no
  build step anywhere.
- **Classic is first-class.** Telnet players get the full game. Every web
  interaction is a command a telnet player could type.
- **One binary, no services.** SQLite, built in. No Node, no database server.

## The 15-minute MUD

The benchmark for the whole experience:

1. `dragon new mygame --kit diku` and `dragon serve`.
2. Open the browser, create a character, walk around.
3. Add a `dance` command to `game/commands.lua`, save, and use it without
   restarting.

When that is smooth, the vision is working.
