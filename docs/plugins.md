# Plugins

Everything a game does beyond the core comes from plugins, including the
game's own code. This document covers what a plugin is, how plugins extend
each other, how they're distributed, and the APIs they build on. Core
contracts are in [design.md](design.md).

## Conventions

**A plugin's `init.lua` returns everything it provides**, as one table.
Its other Lua files are modules in `lua/` that it loads with `require`,
as in a Neovim config, so their names are the plugin's own business: only
what `init.lua` returns matters.
Loading a file has no side effects, which keeps hot reload simple and maps
cleanly to other languages (`export default { ... }` in JavaScript).

```lua
-- mapping/init.lua
return {
  commands = require("commands"),  -- what players type
  slots = require("slots"),        -- slot types for command patterns
  modes = require("modes"),        -- input modes: prompts, menus, editors
  events = {
    declare = require("events"),   -- the events it sends, and their fields
    handlers = require("handlers"), -- what it does when they run
  },
  api = "api",                     -- the module others import, lua/api.lua
  tasks = require("tasks"),        -- run from the command line: dragon mapping:rebuild
  schema = require("schema"),      -- types of object, and fields added to others'
}
```

Each part is a table keyed by name. Errors name the part they're in, such
as `commands.look` or `events.handlers["dragon:said"]`, and a function's
file and line. `client` handles what the web client sends (see Client
events). Still to come: `routes` (HTTP handlers).

**Names defined in Lua are used exactly as written.** A mode, event or slot
type is called what its key says; the engine never renames it, so the
name in a plugin's code is the name everything else uses. Plugins should
namespace their modes and events (`mapping:edit_map`, `mapping:map_drawn`)
so they don't collide with other plugins'; the game's own plugin doesn't
need to. (Slot types can't hold a `:`, which separates a slot's parts in
patterns, so they stay plain.)
`dragon:` is reserved for the engine and its built-ins.

Names the engine makes up for a plugin are namespaced automatically:
client events in `mapping` become `mapping:pan`, and fields it adds to
another plugin's type become `mapping.coords`. Task names aren't made
up; they're written in full, with namespaces the plugin chooses.

## Package layout

A plugin is a directory. Every part is optional except the manifest. The
files in `lua/` are named however the plugin likes; these are the usual
names.

```
mapping/
  plugin.toml       manifest: name, version, provides, uses,
                    capabilities, dependencies
  init.lua          everything the plugin provides, from its modules
  lua/              modules, loaded with require
    commands.lua    player commands and their forms
    modes.lua       input modes (see design.md §4)
    events.lua      the events it sends, and their fields
    handlers.lua    handlers for events (see design.md §4)
  views/            views: templates scripts send (*.txt.tmpl, *.html.tmpl)
  tests/            tests dragon test runs (*_test.lua)
  templates/        other templates, such as entity_tooltip.html.tmpl
  web/              ES modules, CSS, assets for the game client
  admin/            builder UI extensions
```

`require("items")` loads `lua/items.lua` (or `lua/items/init.lua`), and
`require("items.find")` loads `lua/items/find.lua`. A `.lua` file next to
`init.lua` is an error, since `require` would never find it.
`require("@dragon:rooms")` imports another plugin's API (see "APIs and
provides" below).

### The game is a plugin

**The manifest is data, not code.** `plugin.toml` is read without running
any of the plugin, so `dragon add` can show what a plugin asks for before
it's trusted, and the engine knows which capabilities to grant before the
plugin's Lua loads. It's TOML, like `dragon.toml`.

A game directory has the same layout under `game/`, without a manifest:
the game's settings are in `dragon.toml`. It's the top-level
plugin and always wins: its wiring, overrides and templates take precedence
over every installed plugin. Only the game has `events.wiring`, which
reorders or disables other plugins' event handlers.

**Local plugins** live in `game/plugins/<name>/`. They're the game's own
code, split out the way a game would split out combat or crafting: edited
in place, committed with the game, never in `dragon.lock`. They load after
built-in and installed plugins and before the game itself, in directory
name order, except that a plugin loads after the plugins providing the
APIs it uses. Moving one to its own repository
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

### Seeing what a plugin provides

`dragon plugin` lists every plugin the game loads, in load order, and
`dragon plugin <name>` shows everything one provides: its APIs,
dependencies and capabilities; its commands and their forms, slot types
and modes; the events it sends and the ones it handles, with where each
handler runs after ordering and wiring; its views and templates; its
object types and the fields it adds to others; its tasks; and what it
adds to the web client. A built-in can be named without `dragon:` when
no other plugin shares its name (`dragon plugin chat`).

## Built-in plugins and kits

Built-in plugins (`dragon:chat`, `dragon:presence`, `dragon:help`,
`dragon:rooms`, `dragon:items`, `dragon:mapping`, ...) are embedded in the
binary and upgrade with it. They use only the public plugin API. Each covers
one concern and is named for it. `dragon eject <name>` copies one into the game to
customize it, opting it out of engine upgrades.

A game picks which built-ins load with `builtins` in `dragon.toml`. `dragon
new` lists every one, and leaving the setting out loads them all. Removing
one is how a game replaces it: leave out `presence` and announce arrivals
from the game's own handlers instead. Built-ins always load in the engine's
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
events, per-player data, messages, both renderers, a web component, an admin
component, HTTP routes, another plugin's API, and extensions to another
plugin's types and messages.

## Extending other plugins

Only through extension points a plugin offers. **No monkeypatching**: another
plugin's module is read-only. If a plugin isn't extensible enough, it needs a
new extension point.

1. **Behavior**: events (hooks and notifications), command forms (additive; replacing
   is declared).
2. **Data**: add fields to another plugin's types (see Schemas). Added
   fields are namespaced by the adding plugin (`mapping.coords` on a
   room), appear in their own admin form section, and are included in
   export.
3. **Output**: add sections to another plugin's messages; the game can
   override any template or component.
4. **Client UI**: put components into the core client's slots, ordered like
   hooks.
5. **Tooling**: tasks, importers, export formats.
6. **In-world**: objects inherit from parent objects (builder and player
   level, not plugin code).

## Schemas

A plugin declares the **types** of object it works with, and the fields
each has, as `schema` in `init.lua`. Types are how property typos get
caught, and what the admin UI and export will be generated from.

```lua
-- items/lua/schema.lua
return {
  types = {
    ["items:item"] = {
      desc = "Something that can be carried.",
      fields = {
        description = { desc = "what players see when they look at it", type = "text" },
        weight = { desc = "how heavy it is, in pounds", type = "number", default = 1 },
        notes = "anything builders want to remember",
      },
    },
    ["items:container"] = {
      fields = { capacity = { desc = "how much it holds", type = "integer", default = 10 } },
    },
  },
  extend = {
    ["rooms:room"] = { fields = { light = { desc = "how bright it is", type = "integer" } } },
  },
}
```

- **A field** is a description, or a table with `desc` and a `type` and
  `default`. Types are `any` (the default), `string`,
  `text`, `number`, `integer`, `boolean`, `object`, `list` and `table`,
  and every one also takes nil. Every type also has the fields the
  engine reads: `name` and `article` (design.md §5).
- **Names are as written**, like events: plugins namespace their types
  (`items:item`), and `dragon:` is for built-ins. Two plugins declaring
  one type is a startup error.
- **An object can have several types**, and has its parents' too: a bag
  is an `items:item` and an `items:container`, and every copy made from a
  wolf prototype is whatever the wolf is. `world.create{ types = {
  "items:item" } }` gives types at creation, `o:add_type(name)` and
  `o:remove_type(name)` change an object's own, `o.types` lists them all
  (its own, then its parents'), and `o:has_type(name)` checks one.
- **A typed object only takes its types' fields.** `o:set` and
  `o:delete` of a field none of its types has is an error with a
  suggestion (`"descrition" isn't a field of items:item. Did you mean
  "description"?`), and so is `o:get`, so a misspelled read fails too. A
  value of the wrong kind is an error naming the field. `o:get` of a field
  no object in the chain has gives its default.
- **Reading from any object** without knowing its types uses the safe
  reads: `o:try_get(name)` gives nil for a name its types don't declare;
  `o:get_or(name, default)` gives `default` too when there's no value;
  and `o:get_or_set(name, default)` sets `default` and gives it when
  there's no value (checked like `o:set`, since it writes). A field's
  declared default is its value, so it wins over `default`. Adding a type, removing
  one or changing an object's parent checks the object's own properties
  against the types it would have.
- **An object with no types takes any property**, as before types
  existed. So does one with a type no plugin declares any more, such as
  one from a plugin the game stopped loading; startup logs how many
  objects that affects.
- **`extend` adds fields to another plugin's type**, named for the adding
  plugin: `light` above is `items.light` on a `rooms:room`. A plugin's
  fields never collide with another's, and code reads them by their full
  name (`room:get("items.light")`). Extending a type no plugin declares is
  logged at startup and adds nothing, as with handlers for an undeclared
  event, so an optional plugin's absence breaks nothing.

## APIs and "provides"

Plugins use **APIs**, not particular plugins. A plugin lists the APIs it
uses in `[uses]`, and the APIs it provides in `[provides]`.

```toml
# grid-rooms/plugin.toml
name = "grid-rooms"
version = "0.4.0"

[provides]
"dragon:rooms" = "1.3"
```

```toml
# mapping/plugin.toml
name = "mapping"
version = "0.2.0"

[uses]
"dragon:rooms" = "^1.2"
"skywatch:weather" = { version = "^1.0", optional = true }
```

**An API is named for whoever owns its contract**, not for the plugin
providing it: `johns:skills` is John's skills API, whether his plugin
provides it or someone else's compatible one does. So a name says whose
contract it is, and two authors' unrelated `skills` APIs don't collide.
TOML needs quotes around a name with a namespace. As with events, names
are used as written and nothing is added to them. A bare name (`rooms`)
is allowed, for a game's own local plugins that nothing else will load.

- `dragon:` is the engine's namespace. Its APIs are the ones built-in
  plugins provide: `dragon:chat` provides `dragon:chat`, and a plugin
  may provide a `dragon:` API in a built-in's place (`grid-rooms` above,
  with the built-in dropped from `builtins`), but can't make up a new
  one.
- `dragon:rooms` is the reference implementation of the `dragon:rooms`
  API. Plugin versions and API versions are independent.
- A plugin provides as many APIs as it likes: a combat plugin might
  provide `johns:combat` and `johns:damage-types`.
- Versions are one to three numbers (`"1.3"` is 1.3.0). Constraints are
  Cargo's: `"^1.2"` (or `"1.2"`) accepts 1.2 up to 2.0, `"~1.2"` 1.2 up to
  1.3, and `"=1.2.3"` that version only. Below 1.0 each minor may break,
  so `"^0.2"` accepts 0.2 up to 0.3.
- One provider per API per game: two loaded plugins providing the same API
  is an error at startup. (Later, the game config may pick.)
- The game checks every manifest before running any plugin's Lua: each
  API in `[uses]` must have a provider whose version matches, unless it's
  optional.
- Code asks for the API by its name, never by the plugin providing it:
  `require("@dragon:rooms")` in Lua, and the same name in JavaScript's
  import map. So a game swaps `dragon:rooms` for `grid-rooms` without
  touching the plugins that use rooms.

A plugin's `init.lua` names the module that has each API it provides;
the engine loads it and returns the same table to everyone who imports
it.

```lua
-- grid-rooms/init.lua
return {
  commands = require("commands"),
  api = "api",  -- lua/api.lua
}

-- a plugin providing several APIs names a module for each
return {
  api = { ["johns:combat"] = "combat", ["johns:damage-types"] = "damage" },
}

-- mapping/lua/commands.lua
local rooms = require("@dragon:rooms")
```

The API is a module name rather than a table so that the engine can load
just that module into another Lua state, such as the one builders'
entity scripts run in (design.md §10), without running the rest of the
plugin.

- A plugin imports only what its `[uses]` lists, or its own APIs. The
  game's own plugin has no manifest and imports any API a loaded plugin
  provides, since the game chose what loads.
- An optional API no plugin provides imports as `nil`:
  `local weather = require("@skywatch:weather")`, then
  `if weather then ... end`.
- Plugins load in order, but importing an API whose plugin hasn't loaded
  yet loads it first. Two plugins importing each other at the top of a
  file is a loop, and an error; importing inside the function that uses
  the API defers it until it's called.
- Each API module is loaded when the game starts, so a broken one fails
  then, not when something first imports it.
- Each API will have a written contract and a **conformance test
  suite**: `dragon test --conformance dragon:rooms@1` (see Testing).

Prefer extending to replacing. Replace a plugin only for a genuinely
different model (rooms on a grid instead of a graph).

## Capabilities

Plugins declare the system features they need in `plugin.toml`, and the
engine grants only those:

```toml
capabilities = ["tasks", "web_client"]
```

Game features (the world, events, messages, commands, forms, logging)
need no capability: every plugin has them, and so do world scripts
(design.md §11). Capabilities are for what reaches past the game:

| Capability      | Grants                                              |
| --------------- | --------------------------------------------------- |
| `tasks`         | `tasks` in `init.lua`, run from the command line    |
| `live_tasks`    | tasks that run inside the running game (to come)    |
| `store`         | plugin-scoped storage (to come)                     |
| `sql`           | the database directly (to come)                     |
| `web_client`    | JavaScript and CSS in the game client, from `web/`  |
| `client_events` | `client` in `init.lua`: what the web client sends   |
| `web_routes`    | HTTP routes (to come)                               |
| `admin_ui`      | builder UI extensions (to come)                     |

- Using a feature the manifest doesn't declare is a startup error that
  names the capability and shows the line to add.
- An unknown capability is an error, with a suggestion.
- The game's own plugin has no manifest and every capability: it's the
  game owner's code. Local plugins declare theirs like any other, so
  moving one to its own repository changes nothing.
- `dragon add` shows capabilities before installing, and `dragon update`
  points out new ones. `admin_ui` gets an extra warning: its JavaScript
  runs with a builder's privileges.

## Distribution

There's no registry, so a dependency is a place and the versions
accepted: where git can fetch the plugin, and a constraint. The game lists
its own in `dragon.toml`, and a plugin lists the plugins it needs in its
`plugin.toml`, the same way:

```toml
# dragon.toml, or a plugin's plugin.toml
[dependencies]
"github.com/johns/rooms" = "^1.8"
"github.com/usera/mapping" = "~0.2"
```

```sh
dragon add github.com/johns/rooms[@1.8]   # add to dragon.toml and install
dragon install                            # exactly what dragon.lock pins
dragon update [rooms[@2.0]]               # newest versions allowed, of all or one
dragon remove rooms                       # remove from dragon.toml and uninstall
dragon list
```

- **Sources** are paths fetched over HTTPS (`github.com/johns/rooms`), or
  URLs, SSH addresses or local paths git can clone. Versions are tags of
  one to three numbers, with or without a `v` (`v1.2.0`); other tags,
  including pre-releases, are ignored. Constraints are Cargo's, as for
  APIs.
- **`dragon add`** writes the dependency into `dragon.toml`, changing only
  that line: `@1.8` becomes `"^1.8"`, `@~1.8` and `@=1.8.2` are kept as
  written, and no version means a caret on the newest. Then it installs.
- **Installing resolves the whole tree**: the game's dependencies, theirs,
  and so on, one version of each source, the newest every plugin that
  needs it accepts. A version already in `dragon.lock` stays as long as
  every constraint still allows it, so adding one plugin doesn't upgrade
  the rest; `dragon update` moves everything, or one plugin, to the newest
  allowed. Constraints no version satisfies are an error naming who wants
  what. Plugins nothing needs any more are removed.
- **Installed plugins are vendored** into `plugins/<name>/`, named by
  their manifest, without their git history, and committed with the game.
  They load after the built-ins and before the game's local plugins, each
  after the plugins providing the APIs it uses.
- **`dragon.lock`** records each installed plugin's source, exact tag,
  commit, a hash of its files, and what needs it (`dragon.toml`, or other
  plugins).
- **`dragon install`** installs exactly what `dragon.lock` pins, as after
  cloning a game, resolving nothing and leaving `dragon.toml` alone: each
  plugin at its locked tag, which must still name the locked commit and
  give the locked hash, so an author moving a tag can't change what a game
  installs. It removes anything in `plugins/` the lock doesn't list.
  The server won't start if `plugins/` doesn't match the lock, or the lock
  doesn't satisfy `dragon.toml`, with an error saying what to run. To
  change an installed plugin, move it to `game/plugins/`, where it's a
  local plugin.
- **`dragon add` shows the capabilities** each new plugin asks for, with
  what each lets it do, and asks before installing (`-y` doesn't ask).
  `dragon update` asks only when a new version wants capabilities the
  installed one didn't. `dragon remove` refuses a plugin that's only
  installed because another needs it.
- **`[dependencies]` and `[uses]` are different things.**
  `[dependencies]` says which plugins to install; `[uses]` says which
  APIs a plugin uses, whoever provides them. A plugin can use the
  `dragon:rooms` API and leave the game to choose which plugin provides
  it.
- Two installed plugins with the same name, or one named like a local
  plugin, is an error. (Aliases in the game config may come later.)
- A searchable index can come later.

## Tasks

Tasks are what a plugin offers on the command line: seeding a world,
rebuilding maps, importing areas. A plugin exports them as `tasks` from
`init.lua`. **Names are used as written**, like events and modes: a task
with no namespace is run by its name, and a plugin groups tasks under a
namespace the way rake does. Groups nest.

```lua
-- mapping/lua/tasks.lua
local world = require("dragon.world")

return {
  seed = function(args, out) ... end,             -- dragon seed
  {
    namespace = "mapping",
    tasks = {
      clear = function(args, out) ... end,         -- dragon mapping:clear
      {
        name = "rebuild",                          -- dragon mapping:rebuild
        desc = "Redraw every map.",
        depends = { "mapping:clear", "rooms:check" },
        execute = function(args, out)
          ...
          out("Drew", count, "maps.")
        end,
      },
      { namespace = { "areas", "river" }, tasks = { ... } }, -- mapping:areas:river:...
    },
  },
}
```

- **A task** is a function, a table keyed by its name with `desc`,
  `depends` and `execute`, or a table in the list part with its `name`
  inside. **A group** is `{ namespace = ..., tasks = { ... } }`, where
  `namespace` is a name or a list of names; its tasks' names start with
  it, joined with colons.
- **Invoked as `dragon <task>`** (`dragon seed`, `dragon mapping:rebuild`)
  and listed with `dragon tasks`, which shows tasks with no namespace
  first, then each namespace alphabetically. A task without a namespace
  can't share a name with one of dragon's own commands (`dragon list`
  lists plugins); `dragon tasks` points those out.
- **Namespaces are the plugin's choice**: a game can write its own `chat`
  tasks. `dragon` is reserved for built-ins. Two plugins with a task of
  the same name is a startup error that suggests a namespace.
- **`execute(args, out)`**: `args` is a list of the words after the task's
  name (`dragon mapping:rebuild riverside` gives `{ "riverside" }`), and
  `out(...)` prints a line, its arguments joined by spaces. dragon's own
  flags (`-dir`) come before the task's words. A task that raises an error
  fails, and `dragon` exits with it.
- **`depends`** runs prerequisites first, each once, in the order listed,
  named in full as written. Prerequisites get no `args`. A missing task
  or a circle is a startup error.
- **Offline**: a task runs on the game loop of its own copy of the game,
  against the game's database, with every module scripts normally get, but
  no players and no transports. `dragon:booted` is sent first, as when the
  server starts. A task has no deadline (Ctrl-C stops it), and what the
  tasks of one run change is saved together when they finish.
- **Not while the server runs.** `dragon serve` writes `data/server.lock`
  with its process id, and removes it when it stops. A task refuses to run
  while that process is alive, since the server wouldn't see the task's
  changes and could save over them; a second `dragon serve` refuses too. A
  lock left by a server that crashed is ignored.
- Exporting tasks needs the `tasks` capability (see Capabilities).
- To come: `live = true`, running inside the running game through the
  admin API (Milestone 5), the admin UI's Tasks page and the console, and
  the engine's own operations as tasks (`world:export`, `world:import`).

### Importers

Built-in importer plugins bring existing worlds in: `dragon:import-diku`
(Merc, ROM, Smaug `.are`) and `dragon:import-circle` (CircleMUD and tbaMUD
files). A mapping config translates each codebase's item types, flags and
values onto `dragon:items`:

```sh
dragon import-diku:import area/midgaard.are --map import/rom.toml
```

## Testing

`dragon test` runs the tests in `game/tests/` and in each local plugin's
`tests/`. A test file's name ends in `_test.lua`, and it returns a table
of tests; other files in `tests/` are helpers the tests `require`.

```lua
-- game/tests/chat_test.lua
return {
  ["say reaches the room"] = function(t)
    local alice, bob = t:connect(), t:connect()
    alice:login("Alice")
    bob:login("Bob")
    alice:expect("Bob has arrived.")

    bob:send("say hi")
    alice:expect('Bob says, "hi"')
  end,
}
```

- **Every test gets a fresh game**: the game's plugins, an empty world
  and a database of its own, with `dragon:booted` sent as when the server
  starts. Tests play it the way players do, so they test the game as
  telnet and the web see it.
- **`t:connect()`** opens a session at the login prompt. A session's
  `p:send(line)` types a line; `p:expect(text[, seconds])` waits (2
  seconds by default) for output containing `text`, with color codes
  removed, after whatever the last `expect` matched, and fails with
  everything received if it never comes; `p:expect_without(text,
  forbidden...)` also fails if a forbidden text arrives first;
  `p:login(name)` makes an account called `name` through the engine's
  login, returning where the game's character select takes over;
  `p:output()` lists every line so far; `p:disconnect()` closes it.
- **`t:eval(code)`** runs Lua in the game's own plugin, where it can
  `require` the game's modules and the engine's, to set up the world or
  look at it. It returns what the code returns, with objects as their
  ids. Tests themselves run in a Lua state of their own, so the game is
  reached only through sessions and `t:eval`.
- A failing assertion or Lua error fails the test with its file and line.
  `dragon test -run <regexp>` runs only the tests whose names match, and
  `dragon test` exits with an error when any test fails.
- `dragon new` writes `game/tests/tavern_test.lua`, which tests the game it
  makes.
- To come: conformance suites for APIs (`dragon test --conformance
  dragon:rooms@1`), and tests for installed plugins.

## Web client

A plugin adds to the web client with files in its `web/` directory, which
needs the `web_client` capability. The game's own `game/web/` works the
same way, with no capability needed.

```
mapping/web/
  main.mjs        loaded on every page: define elements, listen for events
  map.css         every top-level stylesheet is linked on every page
  map.mjs         other modules, imported from main.mjs or each other
  vendor/         third-party code, copied in
```

### JavaScript modules

Plain ES modules, no build step. The engine generates an import map from
the loaded plugins:

```html
<script type="importmap">
{ "imports": {
  "dragon":       "/assets/client.mjs",
  "mapping/":     "/plugins/mapping/9f8e7d6c5b4a/",
  "grid-rooms/":  "/plugins/grid-rooms/a1b2c3d4e5f6/",
  "dragon:rooms/": "/plugins/grid-rooms/a1b2c3d4e5f6/"
} }
</script>
```

- Each plugin's files import by its name (`import "mapping/map.mjs"`),
  and by each API it provides, so "provides" works in JavaScript too:
  `import "dragon:rooms/map.mjs"` reaches whichever plugin provides
  `dragon:rooms`.
- The hash in the path is of the files' contents, so browsers cache them
  for good and a change gets a new URL. A page loaded before a reload
  still gets the current files, uncached.
- Only a plugin's `web/` directory is served.
- No CDNs (`script-src 'self'`, plus the page's own import map by its
  hash); vendor dependencies into `web/vendor/`.
- Custom element names start with the plugin name (`<mapping-map>`).

### The client API

```js
import { client } from "dragon";   // also window.dragon

client.send("cast fireball goblin");                 // a command
client.push("mapping:pan", { x: 4, y: -2 });         // a client event
const area = await client.request("mapping:area", { id: "riverside" });
client.on("mapping:path_found", (data) => ...);      // pushed by session:push
client.onMessage("room", (element) => ...);          // a message in the feed
client.connection.on("connected", handler);          // or "disconnected"
client.hook("mapping-pin", { mounted(el) {}, removed(el) {} });
```

- Game actions are commands; push and request are for UI state and data.
- `on`, `onMessage` and `connection.on` return a function that stops
  listening. `onMessage` gets each feed message of a view, as the element
  added to the feed, after it's added.
- Custom elements' `connectedCallback` and `disconnectedCallback` are the
  lifecycle hooks; `client.hook(name, ...)` gives the same to plain
  elements with `data-dragon-hook="name"`.

### Client events

What a plugin's JavaScript sends with `client.push` and `client.request`
is handled by its `client` part, which needs the `client_events`
capability:

```lua
-- mapping/init.lua
return {
  client = {
    pan = function(session, data) ... end,
    area = function(session, data)
      return { rooms = rooms_in(data.id) }
    end,
  },
}
```

- The engine puts the plugin's namespace in front of each name, so `pan`
  in `mapping` is sent as `mapping:pan`.
- Handlers run on the game loop as `fn(session, data)`, with the script
  deadline. What one returns answers a `client.request`, with objects as
  their ids; a handler that fails is logged and answers with `null`.
- `session:push(name, data)` sends an event to the player's client, for
  `client.on`; name it with your plugin's name in front
  (`mapping:path_found`). Telnet drops these.
- Only players in the game can send client events, and each connection is
  limited to 20 a second, in bursts of up to 40. The session always comes
  from the server, never from what the client sends.

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
