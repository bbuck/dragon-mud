# Plugins

Everything a game does beyond the core comes from plugins, including the
game's own code. This document covers what a plugin is, how plugins extend
each other, how they're distributed, and the APIs they build on. Core
contracts are in [design.md](design.md).

## Conventions

**Every plugin file is a Lua module that returns a table.** The engine reads
what it returns and registers it. Loading a file has no side effects, which
keeps hot reload simple and maps cleanly to other languages
(`export default { ... }` in JavaScript).

```lua
-- tasks/layout.lua
return {
  rebuild = {
    desc = "Recompute map layouts",
    args = { area = "string" },
    execute = function(args) end,
  },
}
```

**Names defined in Lua are used exactly as written.** A mode, hook or slot
type is called what its file calls it; the engine never renames it, so the
name in a plugin's code is the name everything else uses. Plugins should
namespace their modes and hooks (`mapping:edit_map`, `mapping:map_drawn`)
so they don't collide with other plugins'; the game's own plugin doesn't
need to. (Slot types can't hold a `:`, which separates a slot's parts in
patterns, so they stay plain.)
`dragon:` is reserved for the engine and its built-ins.

Names the engine makes up for a plugin are namespaced automatically:
tasks in `mapping` become `mapping:rebuild`, client events become
`mapping:pan`, and so on.

## Package layout

A plugin is a directory. Every part is optional except the manifest.

```
mapping/
  plugin.lua        manifest: name, version, provides, depends, capabilities
  commands.lua      player commands and their forms
  slots.lua         slot types for command patterns
  modes.lua         input modes: prompts, menus, editors (see design.md §4)
  hooks.lua         hook and notification handlers (see design.md §4)
  lua/              the plugin's own modules, loaded with require
  views/            views: templates scripts send (*.txt.tmpl, *.html.tmpl)
  templates/        other templates, such as entity_tooltip.html.tmpl
  schema.lua        data types it defines or extends
  tasks/            CLI tasks
  client.lua        handlers for events pushed from the web client
  web/              ES modules, CSS, assets for the game client
  admin/            builder UI extensions
  routes.lua        HTTP handlers
```

### The game is a plugin

A game directory has the same layout under `game/`. It's the top-level
plugin and always wins: its wiring, overrides and templates take precedence
over every installed plugin. Only the game has a `wiring.lua`, which
reorders or disables other plugins' hook handlers.

**Local plugins** live in `game/plugins/<name>/`. They're the game's own
code, split out the way a game would split out combat or crafting: edited
in place, committed with the game, never in `dragon.lock`. They load after
built-in and installed plugins and before the game itself, in directory
name order (until `depends` orders them). Moving one to its own repository
and installing it with `dragon add` is how a game shares it. Plugin names
must be unique across every plugin a game loads.

```
mygame/
  dragon.toml       config: transports, engine version, kit
  dragon.lock       pinned plugin versions and hashes
  game/             the game's own plugin
    plugins/        local plugins: part of the game, not installed
  plugins/          installed plugins, vendored
  world/            exported world data
  data/             database, logs (not committed)
```

## Built-in plugins and kits

Built-in plugins (`dragon:chat`, `dragon:presence`, `dragon:help`,
`dragon:rooms`, `dragon:items`, `dragon:mapping`, ...) are embedded in the
binary and upgrade with it. They use only the public plugin API. Each covers
one concern and is named for it. `dragon eject <name>` copies one into the game to
customize it, opting it out of engine upgrades.

A game picks which built-ins load with `builtins` in `dragon.toml`. `dragon
new` lists every one, and leaving the setting out loads them all. Removing
one is how a game replaces it: leave out `presence` and announce arrivals
from `game/hooks.lua` instead. Built-ins always load in the engine's
order, whatever order they're listed in.

```toml
builtins = [
  "chat",
  "help",
  "presence",
]
```

**Kits** are curated sets of built-ins plus wiring: `dragon new mygame --kit
diku|mush|moo`.

`dragon:mapping` is the plugin that validates this design: it uses commands,
hooks, per-player data, messages, both renderers, a web component, an admin
component, HTTP routes, another plugin's API, and extensions to another
plugin's types and messages.

## Extending other plugins

Only through extension points a plugin offers. **No monkeypatching**: another
plugin's module is read-only. If a plugin isn't extensible enough, it needs a
new extension point.

1. **Behavior**: hooks, notifications, command forms (additive; replacing
   is declared).
2. **Data**: add fields to another plugin's types. Added fields are namespaced
   by the adding plugin (`room.mapping.coords`), appear in their own admin
   form section, and are included in export.
3. **Output**: add sections to another plugin's messages; the game can
   override any template or component.
4. **Client UI**: put components into the core client's slots, ordered like
   hooks.
5. **Tooling**: tasks, importers, export formats.
6. **In-world**: objects inherit from parent objects (builder and player
   level, not plugin code).

## APIs and "provides"

Plugins depend on **APIs**, not on particular plugins.

```lua
-- grid-rooms/plugin.lua
return { name = "grid-rooms", version = "0.4.0", provides = { rooms = "1.3" } }

-- mapping/plugin.lua
return { name = "mapping", depends = { rooms = "^1.2" } }
```

- `dragon:rooms` is the reference implementation of the `rooms` API. Plugin
  versions and API versions are independent.
- One provider per API per game; if two are installed, the game config picks.
- Code asks for the API: `plugin.require("rooms")` in Lua, `import ... from
  "rooms/..."` in JavaScript.
- Each API has a written contract and a **conformance test suite**:
  `dragon test --conformance rooms@1`.

Prefer extending to replacing. Replace a plugin only for a genuinely
different model (rooms on a grid instead of a graph).

## Capabilities

Plugins declare what they need, and the engine only grants that:

```lua
capabilities = { "game", "store", "sql", "tasks", "live_tasks",
                 "web_client", "client_events", "web_routes", "admin_ui" }
```

`dragon add` shows capabilities before installing, and `dragon update`
points out new ones. `admin_ui` gets an extra warning: its JavaScript runs
with a builder's privileges.

## Distribution

```sh
dragon add github.com/usera/pluginb[@v1.2.0]
dragon update github.com/usera/pluginb
dragon remove github.com/usera/pluginb
dragon list
```

- Any git host. Versions are semver tags.
- `dragon.lock` pins the commit and a content hash. The server won't start if
  files don't match.
- Plugins are vendored into `plugins/` and committed with the game.
- Version conflicts are resolved like Go modules (minimum version that
  satisfies everyone, one version per game).
- A plugin's identity is its path; its short name comes from the manifest.
  Short name clashes are resolved with an alias in the game config.
- A searchable index can come later.

## Tasks

Rake-style tasks, defined in `tasks/*.lua` (see Conventions).

- Invoked as `dragon <plugin>:<task>`; listed with `dragon tasks`.
- `depends` runs prerequisites once, in order.
- **Offline** by default (own engine and the store); `live = true` runs inside
  the running game through the admin API.
- Also runnable from the admin UI's Tasks page and the console.
- The engine's own operations are tasks too: `db:migrate`, `world:export`,
  `world:import`.

### Importers

Built-in importer plugins bring existing worlds in: `dragon:import-diku`
(Merc, ROM, Smaug `.are`) and `dragon:import-circle` (CircleMUD and tbaMUD
files). A mapping config translates each codebase's item types, flags and
values onto `dragon:items`:

```sh
dragon import-diku:import area/midgaard.are --map import/rom.toml
```

## Web client

### JavaScript modules

Plain ES modules, no build step. The engine generates an import map from the
installed plugins:

```html
<script type="importmap">
{ "imports": {
  "dragon":   "/assets/dragon/client.mjs",
  "mapping/": "/plugins/mapping/9f8e7d/web/",
  "rooms/":   "/plugins/grid-rooms/a1b2c3/web/"
} }
</script>
```

- API names resolve to the providing plugin, so "provides" works in JS too.
- The lock hash in the path gives immutable caching; dev mode disables it.
- Only a plugin's `web/` directory is served.
- No CDNs (`script-src 'self'`); vendor dependencies into `web/vendor/`.
- Custom element names start with the plugin name (`<mapping-map>`).

### The client API

```js
import { client } from "dragon";   // also window.dragon

client.send("cast fireball goblin");                 // a command
client.push("mapping:pan", { x: 4, y: -2 });         // a plugin event
await client.request("mapping:area", { id: "riverside" });
client.on("mapping:path_found", handler);            // server-pushed event
client.onMessage("room", handler);                   // message data
client.connection.on("reconnecting", handler);
```

- Game actions are commands; push and request are for UI state and data.
- Custom elements' `connectedCallback` and `disconnectedCallback` are the
  lifecycle hooks; `data-dragon-hook` covers plain elements.
- Server side, `client.lua` returns typed handlers that run on the game loop;
  `session:push(name, data)` sends events to a player's client.
- Client input is validated and rate-limited; the session always comes from
  the server.

## Admin UI

- **Generated from schemas.** Declared types get list, form and validation
  pages without any plugin code.
- Complex editors (the map editor) ship a custom element in `admin/`.
- Plugin data lives in the object store or plugin-scoped storage by default;
  raw SQL tables are a capability.

## The public API surfaces

Each is versioned with the engine; a breaking change to any of them is a
major version. Each needs reference documentation.

1. Lua modules
2. Manifest format
3. Views, sections and templates
4. Client JS and layout (`dragon` module, slot ids, CSS theme properties,
   import map conventions)
5. Admin extensions and schema format
6. HTTP routes
7. Plugin-provided APIs (versioned by their providers)
8. Tasks

## Go extensions

For advanced users, a custom engine build can compile in Go packages without
forking, like `xcaddy`:

```sh
xdragon build --with github.com/usera/dragon-redis-store
```

Extensions register storage backends, transports, scripting languages or
fast modules. The only requirement now is that the engine registers these
through registries so this is possible later.
