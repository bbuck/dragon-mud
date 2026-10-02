# Design

The engine's core contracts. Code should follow this document; when the code
needs to diverge, update the document in the same change. Plugin packaging,
distribution and the plugin-facing APIs are in [plugins.md](plugins.md). The
goals behind all of it are in [vision.md](vision.md).

## 1. What the core assumes

Very little. The core provides:

- connections, sessions and accounts
- the game loop
- **objects**: an id, an optional unique key, an optional parent (to
  inherit properties from), a location (the object that contains it) and
  properties
- permissions
- scripting
- hooks, notifications and a replaceable command dispatcher
- messages and rendering
- storage, export and import

Rooms, exits, items, mobs, combat, maps and channels are plugins. Containment
is the one structure every MUD, MUSH and MOO shares: a room is an object other
objects are inside.

## 2. The game loop

One goroutine owns the game world. Nothing else reads or writes game state.

```
telnet conn ─┐
telnet conn ─┼─► events chan ─► game loop ─► messages ─► sessions ─► renderers
websocket  ──┤                     ▲
ticker ──────┘                     │
                          plugins (scripting)
```

- Connections turn input into events on a channel. They never touch game
  state.
- Timers, web requests that touch game state, and live tasks also arrive as
  events.
- The loop handles one event to completion, including every hook it triggers,
  before taking the next.
- There is one scripting engine, owned by the loop.
- Every script call gets a deadline, so a runaway plugin can't freeze the
  game (§9).
- Slow work (database writes, outbound HTTP) is handed off; its result comes
  back as an event.

The old engine ran each event on its own goroutine, so back-to-back events
could interleave. The loop removes that whole class of ordering problem.

## 3. Extension points

### Commands

What players type is matched against **forms**: patterns that belong to a
named command.

```lua
say = {
  desc = "Say something.",
  forms = {
    { "say <message>", function(actor, args) ... end },
    { "say <message> to <target:object:here,online>", function(actor, args) ... end },
    { "'<message>", function(actor, args) ... end },
  },
},
```

- **Patterns** are literal words and slots: `<name>` takes free text,
  `<name:type>` resolves it through a slot type, and
  `<name:type:modifier,modifier>` passes modifiers. `[optional parts]`
  expand into a form with and without them. A leading punctuation
  character is its own word, so `'<message>` matches `'hi`.
- **Quotes** escape the parser: `say "hi to bob"` is one value and never
  matches literal words.
- **Maximal munch.** Every form is matched against the input, every way it
  can split. Typed slots are resolved, and a form whose slot fails to
  resolve drops out. The most specific form left wins: the most literal
  words, then the most typed slots resolved, then the plugin loaded last
  (the game beats plugins beats built-ins), then the form written first
  (commands are taken alphabetically, forms in the order written). Within
  one form, the earliest split that resolves wins. `say hi to bob` reaches
  Bob only if Bob is here; otherwise it's said aloud.
- **No match** reports the most specific near miss's reason ("You don't see
  'bob' here."), else the usage of commands whose first word matched, else
  "Huh?".
- **Slot types** are registered like commands: the engine's `text`, `word`,
  `number` and `object` (modifiers `here`, `held`, `online`, `anywhere`;
  `2.sword` picks the second match; ambiguity is an error), and any a
  plugin's `slots.lua` returns, each with declared `modifiers` and
  `resolve(actor, text, modifiers)` returning the value, or nil and a
  reason. Patterns using unknown types or modifiers fail at startup, with a
  suggestion.
- **Resolvers only look.** They run speculatively for every candidate, so
  while they run the world is read-only and any change is an error.
- **Additive by default.** Defining a command another plugin has adds forms
  to it; `replace = true` drops every earlier form, and replacing with no
  forms removes the command. A form matching exactly the same input as
  another is a startup error that says which to remove.

**Input modes** (planned) let a plugin take over a player's input with its
own forms (an editor, a menu, a yes/no question); login becomes the first
mode. Objects in scope will contribute forms too (exits, verbs on held
things), which is how a MOO-style game finds verbs on the objects involved.

### Hooks (can veto or modify)

`can_move`, `modify_damage`, `before_say`. Synchronous and ordered. Each hook
can modify the payload the next one sees, or cancel with a reason. The caller
gets back the final payload and whether it was cancelled.

### Notifications (after the fact)

`player_entered_room`, `mob_died`. Can't modify or cancel. Order is
deterministic but plugins shouldn't depend on it.

### Ordering

1. **Plugin defaults.** Manifests declare dependencies and `before`/`after`
   for hooks. Plugins are sorted topologically; cycles and missing
   dependencies are startup errors. Directory order never matters.
2. **Game wiring.** The game can reorder, disable or redirect any hook in one
   place. Wiring is data, so it's validated at startup and the resolved order
   can be printed (`dragon hooks modify_damage`).

The same precedence applies everywhere: **the game, then plugins in
dependency order, then built-ins**.

## 4. Messages and rendering

The game never writes finished text. It sends **messages**, and each session
renders them for its transport.

- A message has a `kind`, structured `data`, and usually a text form with
  entity references (`{actor} hits {target}`).
- **Feed messages** become lines in the text feed.
- **State updates** (`vitals`, `room_contents`, `map`) are data: the web
  client draws bars and panels, telnet folds them into the prompt and GMCP.
- Messages can have named **sections** other plugins add to (a minimap on the
  room description).
- **Every message has a text form**, so any client can always show it.

### Renderers

Each message kind can have a template per transport:

```
messages/
  say.txt.tmpl     telnet
  say.html.tmpl    web
```

Missing templates fall back to the text form. The game directory can
override any plugin's template.

- **Telnet:** ANSI color (none, 16-color fallback, 256-color), prompt, GMCP.
- **Web:** HTML fragments pushed over a WebSocket into named regions of the
  page (§5).

### Input parity

Every web interaction is a command. Clicking an action bar slot sends
`cast fireball goblin`. There are no web-only actions, only web-only
shortcuts.

### Accessibility

The web feed is an `aria-live` region of real text. Telnet is first-class
because many blind players use it with screen readers.

## 5. The web client

Server-rendered HTML with htmx. No build step.

- **One WebSocket per player**, owned by the core client. Frames are
  multiplexed: HTML fragments go to `htmx.swap`, events go to JS listeners,
  replies resolve requests.
- **Slots** are named regions of the core layout (`#feed`, `#vitals`,
  `#room-panel`, `#sidebar`, `#action-bar`). Plugin templates target slots;
  slot ids are part of the public API.
- The core client ships the hard parts: input with history, window manager,
  tooltips, reconnects.
- Plugins add JavaScript as plain ES modules, imported by name through an
  import map the engine generates. Details and the client API are in
  [plugins.md](plugins.md#web-client).
- **Reconnects:** a disconnected session is kept for a grace period. On
  reconnect the server resends missed feed lines and current state.

The admin and builder UI uses the same approach.

### Layout

The page layout belongs to the game, not the client.

- `dragon new` copies the default `game/web/layout.html` and
  `game/web/layout.css` out of the binary. The game edits them freely.
- The layout defines the **slots** everything renders into, as plain ids htmx
  targets. `#feed` and `#input` are required; startup fails without them.
  Content for a slot the layout leaves out falls back gracefully (a fixture
  becomes text in the feed; a sidebar component doesn't render).
- Layouts can place plugin components directly
  (`<aside id="side"><mapping-map></mapping-map></aside>`), so moving a
  feature is moving a line of HTML.
- `{{.Head}}` is where the engine injects the import map, core client,
  security headers and plugin CSS.
- Themes are CSS custom properties (`--dragon-bg`, `--dragon-accent`, fonts).
  Plugin components use the same properties so they match any theme. Fonts
  are bundled, never loaded from a CDN.
- Layouts are shareable as plugins that provide the `layout` API.
  Precedence: the game's own layout, then a layout plugin, then the built-in
  default. `dragon layout:diff` shows changes to the default since the game
  copied it.

### The default layout

A book to read inside a richer game UI. Sketched from early mockups; not
final.

```
┌──────────────┬─────────────────────────────────┬──────────────┐
│ #context     │ #status: name · vitals · effects│ #side        │
│ (fixtures)   ├─────────────────────────────────┤ (persistent) │
│              │ #feed: one story, serif, book   │ map          │
│ shop, NPC    │ typography; room descriptions,  │ thread       │
│ dialogue,    │ dialogue and events together    │ party        │
│ or weather   ├─────────────────────────────────┤ letters      │
│ as fallback  │ #prompt: choices, actions       │              │
│              │ #input                          │              │
└──────────────┴─────────────────────────────────┴──────────────┘
```

- **Feed structure** comes from message kinds: `room` renders as a heading
  with prose, `say` and `emote` as dialogue and action, `ambient` in italics,
  `echo` (the player's own command) small and muted.
- **Entities in text are clickable**; details open as a card or in
  `#context`, never hover-only, so touch, keyboard and screen readers work.
- **Phones:** `#context` becomes a bottom sheet that slides up when a fixture
  opens; `#side` becomes a drawer.

### Fixtures

Panels the server opens in `#context` for one player, such as a shop's stock
while they browse it:

```lua
session:open_fixture("shop", { scope = "room", priority = 50, data = stock })
```

- **Scope** (`room`, `interaction`, `session`): the engine closes the fixture
  when the scope ends, such as leaving the room.
- **Priority and fallback:** higher priority wins; several open fixtures
  stack or become tabs; a low-priority fallback (weather) shows when nothing
  else is open.
- Players can dismiss fixtures. Every button in one is a command.
- **Telnet:** opening a fixture prints its contents as text (a shop prints
  its stock list); later changes only appear when they matter.

### Pending prompts

A plugin can ask one player for a specific answer ("Wen is waiting for an
answer"): numbered choices, free text, or a yes/no. While a prompt is pending,
the player's input goes to it instead of the command dispatcher. Web shows
the choices in `#prompt`; telnet prints them as a numbered list and the
player types the number. The login name prompt is the first, hardcoded case
of this.

### Vitals

Vitals are defined by the game, not assumed to be HP/mana: a list of
`{ name, value, max?, text?, color }` (a "Warmth: low" bar is valid). Web
draws bars; telnet puts them in the prompt.

## 6. Transports

Each transport is a listener that creates sessions. The game loop doesn't
know which are running.

```toml
[telnet]
enabled = true
address = ":4000"

[web]
address = ":8080"

[web.client]
enabled = true

[web.admin]
enabled = true
```

Startup fails if no play transport is enabled, and warns if admin is exposed
publicly without an admin account.

## 7. Fairness

Client parity only matters for PvP. The engine provides the mechanisms; games
choose whether to use them: server-side cooldowns and timing, GMCP so telnet
clients get the same state, and rate limits that apply equally to every
transport.

## 8. Storage, export and import

- SQLite through a pure-Go driver; no CGO. The database is
  `data/world.db` in the game directory.
- **The database is the source of truth.** The world is built live, mostly in
  the admin UI.
- **Every object is in memory**, owned by the game loop. After each event
  the loop saves what that event changed in one transaction, so a command's
  changes are saved together or not at all. A failed save is retried after
  the next event.
- **Accounts are separate from characters.** An account (name, password
  hash) owns characters, which are ordinary objects. How many characters an
  account may have, and how a player picks one, is the game's choice; until
  pending prompts exist, every account gets one character named after it.
  Passwords are hashed with Argon2id (OWASP's baseline: 19 MiB, two
  passes) off the game loop, a few at a time so a burst of logins can't
  exhaust memory. A hash made with older parameters is replaced at the next
  successful login. Logging in again takes over the
  character from the old connection.
- **Properties are rows** (object, name, JSON value). Whole numbers load as
  integers; a float with no fractional part comes back as an integer.
- **Export is explicit**: `dragon world:export` writes JSONL by default
  (`--format json` for pretty, hand-editable output).
  - One self-describing record per line: type, id, owning plugin, schema
    version, data.
  - Stable string ids, never database row ids, so references survive
    importing into a fresh database.
  - Deterministic order and sorted keys, so one changed room is one changed
    line.
- **Import streams** record by record, in a single transaction, resolving
  references at the end.
- Any type a plugin declares in its schema is exportable and importable for
  free.

## 9. Scripting

The scripting layer is **language-neutral**. The `scripting` package defines
the contract; each language implements it. Lua (`scripting/lua`, gopher-lua,
Lua 5.1) is the default. JavaScript (goja), a custom language, or anything
else can be added without changing the engine or the modules.

- **Modules, not reflection.** The engine exposes a designed API as modules of
  Go functions (`die.roll`, `room.get`). Modules are written once and work in
  every language. Go types are never exposed directly.
- **Boundary values** are nil, bool, number, string, list, map, script
  function and handle. `scripting.Args` gives uniform argument errors in
  every language.
- **Script functions held by Go** (hook handlers) are `scripting.Function`
  values tied to the engine that created them.
- **Every call takes a context.** A script that exceeds its deadline is
  interrupted. Any future language must support this.
- **Sandbox.** Lua gets base, table, string, math and coroutine only.
- **No game state in script globals.** State lives in objects and plugin data.
  This is what makes hot reload safe: the engine can throw away the script
  state, reload files and re-register, and nothing is lost.
- **Handles** are the one way Go-owned things cross the boundary. A
  `scripting.Type` declares a handle's read-only fields and its methods; each
  language presents it natively (Lua: userdata, `h.field`, `h:method()`).
  Handles with the same type and key are the same script value. Scripts
  can't assign fields or reach the metatable; unknown names are errors.
- **Objects** are handles holding an object's id, so a handle to a
  destroyed object raises an error. Fields: `id`, `key`, `parent`,
  `location`, `contents`. Properties are read and written with methods
  (`o:get(name)`, `o:set(name, value)`), never as fields, so property names
  can't collide with the API; ergonomic wrappers are a plugin's job.
  `o:send(text)` reaches everyone playing `o` and does nothing otherwise.
  The `world` module creates, finds and destroys objects. A command's actor
  is the player's character object.
- **Properties can hold objects.** They're stored as refs
  (`{"$object": "id"}` in JSON); a ref to a destroyed object reads as nil.
- **Keys, not names.** An object's unique builder identifier is its `key`
  (`world.keyed("tavern")`); display names are ordinary `name` properties.
- Dice objects built from notation (`random.dice("1d8+2")`) will be handles
  too.

Gopher-lua was chosen because Lua 5.1 is what MUD players already know
(Mudlet; WoW addons and Luau descend from 5.1), it's maintained, and it
supports interruption and coroutines.

## 10. Trust tiers

- **Plugins** are installed by the game owner and trusted, within the
  capabilities they declare (see [plugins.md](plugins.md#capabilities)).
- **In-game code** written by players (MUSH softcode, MOO verbs) is untrusted:
  a restricted module set, per-player CPU and memory limits, stored in the
  object store, and output limited to text and color markup, never
  JavaScript.

## Package layout

| Package            | Responsibility                                          |
| ------------------ | ------------------------------------------------------- |
| `cmds/dragon`      | The `dragon` binary.                                    |
| `config`           | Loading and validating `dragon.toml`.                   |
| `scaffold`         | Files written by `dragon new`.                          |
| `builtin`          | Built-in plugins, embedded in the binary.               |
| `game`             | The game loop, events and world ownership.              |
| `world`            | Objects in memory: ids, parents, locations, properties. |
| `command`          | Form patterns, slot types and the input parser.         |
| `hook`             | Hook chains, notifications, ordering.                   |
| `plugin`           | Plugin loading, manifests and dependency sorting.       |
| `message`          | The message type sent to sessions.                      |
| `session`          | Player sessions and their outgoing queues.              |
| `transport/telnet` | Telnet listener and renderer.                           |
| `transport/web`    | HTTP server, WebSocket game client, admin UI.           |
| `store`            | SQLite persistence.                                     |
| `scripting`        | Language-neutral engine interface, modules, values.     |
| `scripting/lua`    | The Lua implementation (gopher-lua).                    |
| `ansi`             | `[r]color[x]` codes to ANSI escapes and HTML.           |
| `random`           | Seedable random numbers and dice.                       |
| `auth`             | Password hashing (Argon2id).                            |
