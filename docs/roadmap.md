# Roadmap

See [vision.md](vision.md) for the goal, [design.md](design.md) for the core
and [plugins.md](plugins.md) for the plugin system.

## Milestone 0: Foundation

- [x] Restart the codebase (Go modules).
- [x] Write the design docs.
- [x] Port reusable packages: `ansi`, `random`, `auth`.
- [x] Language-neutral scripting layer; gopher-lua engine with sandbox and
      interruption.

## Milestone 1: Something to see

`dragon new` makes a game directory; `dragon serve` runs it; players connect
over telnet and the web and talk to each other.

- [x] `dragon new` and `dragon serve`, `dragon.toml` with transport toggles.
- [x] Game loop, sessions, messages.
- [x] Telnet transport with ANSI color.
- [x] Web transport: page, WebSocket, htmx client, color as HTML.
- [x] Plugin loading: embedded built-ins plus the game's own plugin.
- [x] Commands from Lua (`commands.lua` returns a table), with explicit
      `override` (now `replace`).
- [x] Hot reload of Lua on file change.

## Milestone 2: Core model

- [x] Objects: id, parent, location, properties; SQLite storage.
- [x] Accounts and login with `auth`.
- [x] Objects across the scripting boundary (`player:send(...)`).
- [x] Views: `views/<name>.txt.tmpl` and `.html.tmpl`, with
      blocks the sender picks.
- [x] Entities in messages: `{{entity .x}}`, `<dragon-entity>`, tooltips
      (`dragon:get_tooltip`) and default actions (`dragon:get_default_action`).
- [x] `#id` object references in command input.
- [x] Message sections other plugins add to (`{{section "exits"}}`,
      filled by the `section:room.exits` hook).
- [x] Hooks and notifications with plugin ordering (`before`/`after`)
      and game wiring (`order`, `disable`).
- [x] `dragon events [<name>]` (was `dragon hooks`).
- [x] Hook redirection in game wiring (`redirect`).
- [x] Input parser: command forms, slot types (`slots.lua`), maximal
      munch, quoting, additive forms with `replace`.
- [x] `builtins` in `dragon.toml`: which built-in plugins a game loads.
- [x] Input modes (editors, menus, pending prompts); login as a mode, then
      character select and creation as game-controlled modes
      (`dragon:characters`, with creation steps from `dragon:character_steps`).
- [x] Text layout: telnet wrapping, and layout helpers (`columns`,
      `table`, ...) with HTML equivalents.
- [x] `dragon:unmatched_input` hook: input no command matches is offered to
      plugins before the player sees an error (design.md §2).
- [x] Form sets in Lua (`forms.new`, `set:parse`): the command parser for
      entity scripts and anything else with its own vocabulary.
- [x] `actor:is_player()`.
- [x] Feed views: `room`, `say`, `emote`, `ambient`, `echo`.
- [x] `game.run(actor, line)`: commands by another name, and NPCs acting
      through the commands players use.
- [x] `game.broadcast_to(location, ...)`: messages to what's directly
      inside an object.
- [x] Article and case helpers in templates (`{{the .x}}`, `{{A .x}}`,
      `{{cap}}`).
- [x] Declared hook fields: one name per role (`actor` for whoever acts),
      fields documented by `dragon events <name>`, and events that error
      on a field the hook doesn't have.

## Milestone 3: Plugins for real

- [x] Local plugins in `game/plugins/<name>/`.
- [x] `require` for a plugin's own modules.
- [x] Manifests are `plugin.toml`, read without running the plugin; the
      game's settings are `dragon.toml`.
- [x] `init.lua` returns everything a plugin provides, built from its
      modules in `lua/`, so file names are the plugin's own business.
- [x] "Event" names both kinds, hooks and notifications: `dragon.events`,
      `events.declare`, `events.handlers`, `dragon events`.
- [x] Manifest `[provides]` and `[uses]`: API versions and Cargo-style
      constraints, checked at startup, one provider per API.
- [x] Manifest capabilities, checked where a system feature is used
      (tasks first). The world-state check in bindings comes with entity
      scripts.
- [x] `require("@name")` imports a plugin API by name, never by plugin;
      `init.lua` names the API's module (`api = "api"`). API names are
      namespaced by the contract's owner (`johns:skills`), and `dragon:`
      APIs are the built-ins'. First use: `dragon:chat` provides
      `dragon:chat` with `chat.say(actor, message, target)` and
      `chat.emote`, so games stop reusing its `say` view and coupling to
      its data shape (lesson 3 below).
- [x] Schema types and extensions; namespaced added fields
      (`mapping.coords`). An object can have several types, and inherits
      its parents'; a typed object only takes its types' fields, which
      catches typos like `descrition` (lesson 6 below).
- [x] Tasks (`dragon <task>`, `dragon tasks`), named as written with
      rake-style namespaces, offline. Live tasks
      wait for the admin API (Milestone 5).
- [x] `dragon add/update/remove/list`, `dragon.lock`, vendoring.
      `[dependencies]` in `dragon.toml` and `plugin.toml` list sources and
      constraints, resolved together into `dragon.lock`.
- [x] Import maps, plugin JS, the `dragon` client API, client events.
- [x] `dragon test` with scripted sessions. Conformance suites for APIs
      are still to come.
- [x] Plugin introspection: `dragon plugin <name>` shows everything a
      plugin provides (APIs, dependencies, capabilities, commands and
      forms, slot types, modes, the events it sends and handles and where
      its handlers run, views and templates, types and added fields,
      tasks, and what it adds to the web client), so a game author knows
      what they can add to, replace or wire from `game/`. `dragon plugin`
      lists every plugin. Generating reference docs from the same data is
      still to come.

- [ ] No automatic prefixes: client events (`mapping:pan`) and fields added
      to another plugin's type (`mapping.coords`) get their plugin's name
      put in front today. Names should be as written, like events, modes
      and tasks, so anyone reading the code knows what to reference: a
      plugin extending a type writes the field's full name
      (`["mapping.coords"]`), and two plugins adding the same field to a
      type is a startup error naming both.

## Milestone 4: World

- [ ] Entity scripts (design.md §2): stored on objects, edited in-game and
      in the admin editor, `o:handle` with parent inheritance, `o:send`
      calling the view's handler.
- [ ] Event audiences (design.md §2): a hook's declaration names who hears
      it (`audience = { "room.contents", "actor" }`), the engine calls
      `o:handle` on each, and games adjust it in wiring with `deliver`.
      Builds on declared hook fields and entity scripts.
- [ ] `dragon:rooms`: rooms, exits, movement, `can_move`; rooms deliver
      views to their contents and offer unmatched input to them. Moving
      is `rooms:can_move` (hook), `rooms:left` in the old room, `move_to`,
      then `rooms:entered` in the new one (notifications carrying `actor`,
      so mobs moving count too).
- [x] Built-in actions come in pairs: a hook before (change or cancel)
      and a notification after (react). `dragon:chat` gets `dragon:said`
      and `dragon:emoted`, so NPCs answer after the player's line, not
      inside `dragon:before_say` (lesson 2 below).
- [x] `dragon:chat` and `dragon:presence` reach the actor's location with
      `game.broadcast_to`, falling back to the whole game for actors who
      are nowhere (lesson 4 below).
- [ ] Speech triggers: a plugin helper for "react when someone says X"
      (lesson 5 below).
- [ ] `dragon:items`.
- [ ] `dragon:mapping` (validates the plugin design end to end).
- [ ] State updates, slots, web panels, tooltips; telnet prompt and GMCP.
- [ ] Game-defined vitals.
- [ ] Fixtures in `#context`: scope, priority, fallback, dismissal.
- [ ] Windows (design.md §6): sending a view to a window or slot, windows
      updating in place, `<ui-window>`; settle whether fixtures are windows.
- [ ] Web forms that send an event (`<form action="mygame:register">`)
      with their inputs as fields, and screens per mode, so login and
      character select can be pages before the game view.
- [ ] A rich text editor component that writes color codes, for
      `describe` and anything else using the editor mode.
- [ ] `game/web/layout.html` and `layout.css`: copied by `dragon new`,
      validated at startup, `{{.Head}}` injection, CSS theme properties.
- [ ] The default layout: a terminal in the web with richer features
      (clickable entities, prompts as buttons, windows), and its phone
      version. Book-style layouts are games' own.
- [ ] Layouts as plugins; `dragon layout:diff`.
- [ ] Reconnect with resume.
- [ ] `world:export` / `world:import` (JSONL).

## Milestone 5: Building

- [ ] Admin UI generated from schemas.
- [ ] Admin extensions (map editor).
- [ ] Tasks page in admin.
- [ ] `dragon console`.

## Later

- Kits (`--kit diku|mush|moo`) and `dragon eject`. The MOO kit finds verbs
  on the objects a line names; the MUSH kit has `$`-commands and exits
  matched by name.
- Script limits a game can set for its builders' entity scripts (quotas,
  module sets), for games that open scripting widely.
- Importers (`dragon:import-diku`, `dragon:import-circle`).
- A `dragon` command that generates a docs site from doc comments
  (LuaDoc or LuaLS annotations) in plugins' API modules, so `dragon:chat`'s
  API can be read without opening `lua/api.lua`.
- Dice notation: modifiers, keep highest, drop lowest; dice objects.
- Fairness features: rate limits, cooldowns.
- Transactions scripts opt into, like `store.transaction(function() ...
  end)`: if the function fails, nothing it changed is saved. Today a
  failed task still saves what it changed, so tasks are written to run
  again safely.
- Engine upgrades: deprecations, `dragon upgrade`.
- Go extensions and `xdragon build`.
- A second scripting language behind `scripting`, and a conformance suite
  every scripting engine must pass.
- TLS for telnet.
- Plugin index.

## Lessons from test games

What building `test_dragon` (a two-room tavern with a bartender and a
hermit, built on the engine as it was on 2026-10-03) showed.

1. **NPCs had to send views by hand to speak.** The keeper rendered a
   `hail` view to each player itself. NPCs should act through the same
   commands players use: now `game.run(npc, "say ...")`.
2. **NPC reactions ran before what they reacted to.** The only hook on
   speech was `dragon:before_say`, so the keeper answered before the
   player's line appeared. Every built-in action needs a notification
   after it, not only a hook before.
3. **Reusing another plugin's view couples you to its data.** Sending
   `say` with `speaker` instead of `actor` broke twice. Missing data is now
   an error, but plugins should offer functions (`chat.say`) rather than
   games reusing their views.
4. **Speech reached the whole game.** `say`, `emote` and custom actions
   used `game.broadcast`, so the cellar heard the tavern. Locations are
   core, so `game.broadcast_to` now exists; the built-ins should use it.
5. **Matching speech was hand-rolled** (`string.find(message, "hail")`).
   Keyword triggers are worth a helper, as a plugin.
6. **Freeform properties hide typos.** `descrition` went unnoticed;
   schemas will catch it.
7. **Silent failures cost the most time.** A template reading data that
   wasn't sent, a blocks-only view sent without a block, `actor.name`
   for a property, and `<no value>` in telnet all looked like other
   bugs. Each is now an error saying what to change; keep finding these.
8. **Names carried their articles** ("the bartender", "a dingy hermit"),
   which reads wrong as soon as a sentence needs the other article or a
   capital. Names should be bare, with article helpers in templates.

## Documentation to write

The engine is only as usable as its docs. Each of these needs a reference,
plus guides for the common paths.

- **Getting started:** install, `dragon new`, `dragon serve`, the 15-minute
  MUD.
- **Game directory:** layout, `dragon.toml`, the game as a plugin, overrides.
- **Lua API reference:** every module and function.
- **Manifest reference:** fields, provides, uses, capabilities, dependencies.
- **Views:** sections, telnet and HTML templates, layout helpers,
  color codes.
- **Events and wiring:** hooks and notifications, declarations, ordering,
  `dragon events`.
- **Web client:** slots, the `dragon` JS API, import maps, custom elements,
  client events.
- **Layouts and themes:** editing `layout.html`, required and optional slots,
  CSS theme properties, fixtures, sharing a layout as a plugin.
- **Admin and schemas:** schema format, type extensions, admin components.
- **Tasks and CLI:** every core command, writing tasks.
- **Plugin authoring and publishing:** layout, versioning, conformance tests.
- **Export and import:** JSONL format, importers and mapping configs.
- **Upgrading:** engine versions and deprecations.

## Salvaged from the old engine

The pre-restart engine is in git history before commit `0ad383a`.

| Old package | New package | Changes |
| ----------- | ----------- | ------- |
| `ansi` | `ansi` | Same behavior; tests moved to `testing`; HTML output added. |
| `random` | `random` | Seedable `Rand`; fixed `Range` never returning `max`. |
| `scripting/modules/password.go` | `auth` | Plain Go API; fixed fallback cost of up to 31. |

Worth consulting later: `scripting/modules/*` (Lua APIs for `events`, `log`,
`config`, `die`, `tmpl`), `scripting/modules/cli.go` (the original plugin
CLI commands), `scripting/lua/repl.go` (console), `fs` (game directory
layout).

Not coming back: `talon` (Neo4j), `events` (replaced by `hook`), the Lua
engine pool, `text/tmpl` (velvet is archived), `logger` (use `log/slog`).
