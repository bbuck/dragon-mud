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
- [x] Message kinds: `messages/<kind>.txt.tmpl` and `.html.tmpl`, with
      blocks the sender picks.
- [x] Entities in messages: `{{entity .x}}`, `<dragon-entity>`, tooltips
      (`get_tooltip`) and default actions (`get_default_action`).
- [x] `#id` object references in command input.
- [x] Message sections other plugins add to (`{{section "exits"}}`,
      filled by the `section:room.exits` hook).
- [x] Hooks and notifications with plugin ordering (`before`/`after`)
      and game wiring (`order`, `disable`).
- [x] `dragon hooks [<name>]`.
- [ ] Hook redirection in game wiring.
- [x] Input parser: command forms, slot types (`slots.lua`), maximal
      munch, quoting, additive forms with `replace`.
- [x] `builtins` in `dragon.toml`: which built-in plugins a game loads.
- [x] Input modes (editors, menus, pending prompts); login as a mode, then
      character select and creation as game-controlled modes
      (`dragon:characters`, `dragon:character-creation`).
- [x] Text layout: telnet wrapping, and layout helpers (`columns`,
      `table`, ...) with HTML equivalents.
- [ ] Forms contributed by objects in scope (exits, verbs on held things).
- [x] Feed message kinds: `room`, `say`, `emote`, `ambient`, `echo`.

## Milestone 3: Plugins for real

- [ ] Full manifest: provides, depends, capabilities.
- [ ] `plugin.require` for plugin APIs.
- [ ] Schema types and extensions; namespaced added fields. Open: can an
      object have several types (a bag is an item and a container)?
- [ ] Tasks (`dragon <plugin>:<task>`, `dragon tasks`).
- [ ] `dragon add/update/remove/list`, `dragon.lock`, vendoring.
- [ ] Import maps, plugin JS, the `dragon` client API, client events.
- [ ] `dragon test` with scripted sessions.
- [ ] Plugin introspection: `dragon plugin <name>` shows everything a
      plugin provides (commands and forms, slot types, hook handlers, the
      hooks and notifications it runs, and later messages, schema and
      tasks), so a game author knows what they can add to, replace or wire
      from `game/`. Hooks a plugin runs need declaring, since they can't be
      found statically. The same data generates each plugin's reference
      docs.

## Milestone 4: World

- [ ] `dragon:rooms`: rooms, exits, movement, `can_move`.
- [ ] `dragon:items`.
- [ ] `dragon:mapping` (validates the plugin design end to end).
- [ ] State updates, slots, web panels, tooltips; telnet prompt and GMCP.
- [ ] Game-defined vitals.
- [ ] Fixtures in `#context`: scope, priority, fallback, dismissal.
- [ ] `game/web/layout.html` and `layout.css`: copied by `dragon new`,
      validated at startup, `{{.Head}}` injection, CSS theme properties.
- [ ] The default layout (two sidebars, book-style feed) and its phone
      version.
- [ ] Layouts as plugins; `dragon layout:diff`.
- [ ] Reconnect with resume.
- [ ] `world:export` / `world:import` (JSONL).

## Milestone 5: Building

- [ ] Admin UI generated from schemas.
- [ ] Admin extensions (map editor).
- [ ] Tasks page in admin.
- [ ] `dragon console`.

## Later

- Kits (`--kit diku|mush|moo`) and `dragon eject`.
- Trust tiers: in-game player code with quotas.
- Importers (`dragon:import-diku`, `dragon:import-circle`).
- Dice notation: modifiers, keep highest, drop lowest; dice objects.
- Fairness features: rate limits, cooldowns.
- Engine upgrades: deprecations, `dragon upgrade`.
- Go extensions and `xdragon build`.
- A second scripting language behind `scripting`, and a conformance suite
  every scripting engine must pass.
- TLS for telnet.
- Plugin index.

## Documentation to write

The engine is only as usable as its docs. Each of these needs a reference,
plus guides for the common paths.

- **Getting started:** install, `dragon new`, `dragon serve`, the 15-minute
  MUD.
- **Game directory:** layout, `dragon.toml`, the game as a plugin, overrides.
- **Lua API reference:** every module and function.
- **Manifest reference:** fields, provides, depends, capabilities.
- **Messages and templates:** kinds, sections, telnet and HTML templates,
  color codes.
- **Hooks and wiring:** hook kinds, ordering, `dragon hooks`.
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
