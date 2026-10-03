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
  inherit properties and script handlers from), a location (the object
  that contains it), properties and an optional script (§2)
- permissions
- scripting, including scripts on objects (§2)
- hooks, notifications and a replaceable command dispatcher
- messages and rendering
- storage, export and import

Rooms, exits, items, mobs, combat, maps and channels are plugins. Containment
is the one structure every MUD, MUSH and MOO shares: a room is an object other
objects are inside.

## 2. Entity scripts

Most of a game is behavior on the things in it: a shopkeeper who sells, a
guard who answers when greeted, a lever that opens a door, a quest item
that reacts when it's handed over. Plugins supply the rules; entity scripts
are what the rules apply to. Form sets, the `unmatched_input` hook and
`is_player` are built; entity scripts themselves come with Milestone 4,
but this is the shape the rest of the design has to leave room for.

There are three layers. The **engine** makes a game work. The **game** (the
game directory: config, plugins, views) uses the engine to turn input and
output into an experience. The **world** brings that game to life: the
objects in the database and the scripts on them, which make roaming mobs,
talking NPCs, random events and weather.

**Entity scripts are part of the world, not the game.** A script is stored
on its object, like its properties, and builders write it in-game over
telnet or in the admin editor, never as a file in the game directory.
Nobody pre-builds every enemy on disk. Scripts are saved with the object,
exported and imported with the world (§9), and compiled from their stored
source when they're edited and again after the game reloads.

**An entity script is a table of handlers**, keyed by event name:

```lua
return {
  player_entered = function(self, actor) ... end,
  given = function(self, giver, item) ... end,
  say = function(self, data, block) ... end,
}
```

**`o:handle(name, ...)` calls one.** If `o`'s script has no handler by that
name, or `o` has no script, the engine looks in its parent's script, then
that one's parent, the same way properties inherit; every copy of a
shopkeeper shares one script, and MOO-style verb inheritance comes free. If
nothing up the chain handles it, nothing happens. The engine gives the
names no meaning; plugins decide which events exist and when they happen.
`dragon:rooms` doesn't need to know about NPCs to tell everything in a room
that someone arrived:

```lua
for _, thing in ipairs(actor.location.contents) do
  thing:handle("player_entered", actor)
end
```

A combat plugin would call `attacked` and `died`, `dragon:items` would call
`given`. Event names are taken as written, like hook names, so a plugin's
own events should carry its prefix (`combat:died`).

Two kinds of event come from the engine's own primitives.

**Views.** `o:send(view, data[, block])` reaches everyone playing `o`
(§10) and also calls `o:handle(view, data, block)`. A room is an object
like any other, so delivering to everything in a room is the room's own
handler sending to its contents, written by `dragon:rooms`. When
`dragon:chat` sends a `say`, the guard's `say` handler gets `{ actor = bob,
message = "hail" }`, the data the view renders from, never the rendered
text, and the block (`target` when Bob said it to the guard).

**Input no command claimed.** Global commands go first. When nothing
matches, the engine runs the `unmatched_input` hook (`actor`, `line`, and
the near miss's `reason` if there was one) before telling the player. A
handler that deals with the line sets `event.handled = true` and returns
the event. A plugin decides which entities are offered the line: `dragon:rooms` would
call `input` on what's in the actor's room, a MOO kit on the objects the
line names, a MUSH kit on objects with `$`-commands. If nobody takes it, the
player sees the original reason, usage or "Huh?". So `buy 10 apples` with no
global `buy` reaches the shopkeeper, and a global `buy` would stop it from
ever getting there: shop verbs belong on shopkeepers, not in built-ins.

An entity parses the line with its own **form set**, built from the same
patterns, slot types and scoring as commands (§4):

```lua
local shop = forms.new {
  { "buy <count:number> <item>", function(actor, args, self) ... end },
  { "list", function(actor, args, self) ... end },
}

return {
  input = function(self, actor, line) return shop:parse(actor, line, self) end,
  say = function(self, data, block)
    if data.from_npc then return end
    if data.message:lower():find("hail") then
      self.location:send("say", { actor = self, message = "Well met.", from_npc = true })
    end
  end,
}
```

`shop:parse(actor, line, ...)` runs the form that matched as `fn(actor,
args, ...)` and returns true, or returns false and the near miss (`reason`
when a slot didn't resolve, `usage` listing patterns whose first word
matched), so the shopkeeper can say "I don't sell pears" for `buy 10
pears` and stay quiet for `dance`. A form does all its own work, like a
command; what it returns is ignored. `input` returns what `parse` did,
`o:handle` returns what the handler did, and the plugin that offered the
line sets `event.handled`. A form set is built when its file loads, so it
can use the slot types of its own plugin and those loaded before it.

- **Data is the contract.** The engine passes arguments through untouched
  and adds nothing to them. Who sent something is whatever the data says
  (`actor`, here); `actor:is_player()` is true when an account owns the
  object as a character.
- **Loops are a game problem.** Two NPCs answering each other forever is a
  bug the builder fixes with data (`from_npc` above), the same as any other
  infinite loop. The engine imposes no depth limit, so a game full of
  characters that talk to each other is possible. A loop that never yields
  is still interrupted by the script deadline (§10).
- **Missing handlers are the common case.** Most entities care about a
  few events, so an unhandled one is not an error: builders never write
  empty handlers, and a new plugin's events don't mean revisiting old
  scripts. To help find a misspelled handler (`player_enterd`), an event
  that some script in the chain could have handled but none did is logged
  at debug level, once per script and event, naming the object whose
  script was nearest ("mega wolf (#45) has no handler for
  `player_entered`"). The record of what's been logged is cleared when
  the script changes or the game reloads. A thousand wolves
  made with `world.create{ parent = mega_wolf }` log once between them.
- **One interface at every trust level.** A handler a builder writes in the
  game directory and one a player writes inside the game (§11) are called
  the same way. Trust changes where the code is stored, which modules it
  gets and its limits, never how it's called.

Open:

- What a world script can reach: which modules, and whether the game
  chooses what it exposes to the world.
- How a script that fails to compile is reported to the builder editing
  it, and what happens to the object until it's fixed.
- Whether handlers run immediately or after the current event, so the
  player sees their own "You say" before the guard answers.

## 3. The game loop

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
  game (§10).
- Slow work (database writes, outbound HTTP) is handed off; its result comes
  back as an event.

The old engine ran each event on its own goroutine, so back-to-back events
could interleave. The loop removes that whole class of ordering problem.

## 4. Extension points

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
  `2.sword` picks the second match; `#id` picks exactly that object, if
  it's within the modifiers' reach; ambiguity is an error), and any a
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

### Input modes

An **input mode** takes over a player's input: a yes/no question, a menu,
an editor, a combat stance, character creation. Each session has a **stack
of modes**, and input goes to the top one:

1. The mode's **forms**, matched exactly like commands (same patterns, slot
   types and maximal munch).
2. If none match, its **`input`** handler gets the raw, untrimmed line.
3. If it has neither and sets **`passthrough = true`**, the line goes to the
   mode below. Under every mode, once the player is in the game, are the
   commands.
4. Otherwise the player sees what the mode's forms accept, or "Huh?".

A plugin's `modes.lua` returns its modes:

```lua
return {
  confirm_destroy = {
    enter = function(session, state)
      session:prompt({ text = "Destroy it?", choices = { "yes", "no" } })
    end,
    forms = {
      { "yes", function(session, args, state) world.destroy(state.thing); session:pop_mode() end },
      { "no",  function(session) session:pop_mode() end },
    },
  },
  -- session:push_mode("editor", { target = object, property = "description", lines = {} })
  editor = {
    input = function(session, line, state)
      if line == "." then
        state.target:set(state.property, table.concat(state.lines, "\n"))
        return session:pop_mode()
      end
      table.insert(state.lines, line)
      return state
    end,
  },
}
```

- **Handlers:** `enter(session, state)` when the mode starts,
  `input(session, line, state)`, `resume(session, state, result)` when a
  mode above it ends, and `leave(session, state, reason)` when it ends
  (`reason` is `done`, `replaced`, `playing` or `disconnected`). Forms are
  called as `(session, args, state)`.
- **State is plain data.** A mode is a name plus a state table, never a
  closure, so a reload keeps every player's modes: the engine looks the name
  up again. State crosses the boundary as a copy; a handler returns the
  state to change it, like a hook returns its event. A function anywhere in
  state is an error. A mode a reload removed is dropped, and the player is
  told.
- **Results go down the stack.** `session:pop_mode(result)` passes `result`
  to the `resume` of the mode below. A mode that ends while the mode below
  it is still running a handler (a step that finishes in its own `enter`)
  resumes it once that handler returns, so `resume` sees the state it left.
- **Names are what the plugin wrote.** The engine never adds a namespace
  to anything defined in Lua: `modes = { edit_map = ... }` is pushed as
  `edit_map`. A name can carry a namespace, and plugins should use their
  own (`mapping:edit_map`) so they don't collide; the game's own modes
  don't need one. `dragon:` is reserved for built-ins, and `dragon:login`
  for the engine.
- **Defining a mode again adds to it.** Forms are additive, like commands,
  so a game can add a "back" form to a plugin's menu. Setting a handler or
  `passthrough` another plugin already set is a startup error unless the
  definition sets `replace = true`.
- **Prompts:** `session:prompt(text)`, or `session:prompt({ ... })` with
  `text`, or `view`, `data` and `block` to render a view (§5);
  `choices`, a list of answers; and `secret`, to hide what the player types.
  Text prompts list their choices, numbered in telnet and clickable on the
  web (`<dragon-choice>`). A view's template shows its choices itself, so a
  game can restyle a plugin's menu by overriding its template. Typing a
  choice's number answers with that choice. Choices need a mode to answer
  them.

#### Combining modes

A mode should never need to know which mode just ended above it. A `resume`
that checks where its result came from (`if state.pushed == "editor" and
state.field == "description" then ...`) is the sign of a missing pattern.
These cover what games need:

- **Give reusable modes what to do as data.** The editor above takes a
  `target` and a `property` and saves the text itself, so `describe`
  (a command) and `@describe` (in a builder's mode) both push it with
  different targets, and neither handles a result. Confirmations work the
  same way: push `confirm_destroy` with the `thing`.
- **Let a flow own its steps.** When several modes make up one task, one
  mode runs them in order and keeps its progress in its own state, the way
  `dragon:create_character` does. Each step ends with
  `session:pop_mode(result)`, and the flow's `resume` merges the result and
  starts the next step. The flow knows what it started because its own
  state says so, not because it inspects the result.
- **Replace instead of returning** when one mode leads to another and
  nothing comes back: `session:replace_mode("next", state)`.

**Login is the first mode.** It's the engine's own (`dragon:login`), at the
bottom of every new session's stack, and it stays in Go so passwords never
reach scripts. Once the account is known, the engine starts the game's
**`characters`** mode if it defines one, otherwise **`dragon:characters`**.
Either must end with `session:play(character)`, and startup fails if
neither exists. `play` ends every mode, takes the character over from any
other connection, sends `player_entered` and runs `look`.

The built-in `dragon:characters` plugin defines two modes:

- `dragon:characters` plays the account's only character, asks which when
  there are several, and starts `dragon:create_character` when there are
  none.
- `dragon:create_character` runs the `character_steps` hook (`account`, `steps`),
  whose handlers add the names of step modes to `steps`, ordered and wired
  like any hook. Each step gets `state.draft` and ends with
  `session:pop_mode(changes)`, merged into the draft. When the steps are
  done, the draft's fields become the new character's properties. The
  draft starts as `{ name = <account name> }`; with no steps, the
  character is made straight away.

Most games change creation by adding steps. A game with its own select
screen defines `characters`, which can still push
`dragon:create_character`. A game whose creation is entirely its own also
leaves `characters` out of `builtins`.

A plugin that changes another plugin's behavior does it through that
plugin's hooks, not by checking whether it's installed: `dragon:classes`
would run `available_classes` (`draft`, `classes`), and `dragon:races`
would filter the list. Optional dependencies (`depends = { weather = {
"^1.0", optional = true } }`, Milestone 3) are for *using* another plugin's
API.

Input no command matches can still reach the entities around the player,
which parse it with their own forms (§2). That's how a shopkeeper handles
`buy` and how a MOO kit finds verbs on the objects involved.

### Hooks (can veto or modify)

`can_move`, `modify_damage`, `before_say`. Synchronous and ordered. Each hook
can modify the payload the next one sees, or cancel with a reason. The caller
gets back the final payload and whether it was cancelled.

A plugin's `hooks.lua` returns its handlers, keyed by hook name. Each is a
function, or a table with the function and its ordering:

```lua
return {
  player_entered = function(event) ... end,
  before_say = {
    after = { "dragon:chat" },
    handler = function(event)
      if muted(event.actor) then return false, "You are muted." end
      event.message = tidy(event.message)
      return event
    end,
  },
}
```

- A hook handler returns nothing to leave the event as it is, the event to
  change it, or `false` and a reason to cancel. Payloads cross the
  scripting boundary as copies, so changing the event without returning it
  changes nothing.
- A handler that fails stops the hook with an error naming the handler.
- Code runs a hook with `hooks.run(name, event)`, which returns the event as
  the handlers left it, or `nil` and the reason one cancelled.
- A plugin has one handler per hook.

### Notifications (after the fact)

`player_entered_room`, `mob_died`. Can't modify or cancel. Order is
deterministic but plugins shouldn't depend on it. Handlers are declared in
`hooks.lua` like hook handlers; what they return is ignored. A failing
handler is logged and the rest still run. Code sends one with
`hooks.notify(name, event)`.

The engine sends `player_entered` (`player`, and `reconnected` when the
player took over their character from another connection) and
`player_left` (`player`). `dragon:presence` handles both to announce
arrivals and departures. `dragon:chat` runs `before_say` (`actor`,
`message`, and `target` when saying something to someone). The web client
runs `get_tooltip` and `get_default_action` (`viewer`, `entity`; §6).
Input no command matches runs `unmatched_input` (`actor`, `line`,
`reason`; §2).

### Ordering

1. **Plugin defaults.** Handlers run in load order: built-ins, then
   plugins in dependency order, then the game, so the game sees the payload
   last and has the final say. A handler's `before` and `after` move it
   relative to other plugins' handlers for the same hook; plugins that
   aren't installed or have no handler for it are ignored, so a plugin can
   order itself against optional ones. Cycles are startup errors that name
   every step of the cycle. Directory order never matters.
2. **Game wiring.** The game can reorder or disable any hook's handlers in
   one place, `game/wiring.lua`. Only the game's plugin may have one.
   Wiring is data, so it's validated at startup and the resolved order can
   be printed (`dragon hooks modify_damage`; `dragon hooks` lists every
   hook).

```lua
-- game/wiring.lua
return {
  hooks = {
    modify_damage = { order = { "game", "armor", "dragon:combat" } },
    player_entered = { disable = { "dragon:presence" } },
  },
}
```

`order` replaces the plugins' `before` and `after` for that hook (which also
settles a cycle) and must list every handler that isn't disabled, so a new
plugin's handler can't slip into a hand-made order unnoticed. Naming a
plugin with no handler for the hook is an error with a suggestion.
Redirecting a hook to a different handler is not built yet.

The same precedence applies everywhere: **the game, then plugins in
dependency order, then built-ins**.

## 5. Views and messages

The game never writes finished text. Scripts send **views**: a view is a
named template, one file per format, filled in with data. Rendering a view
makes a **message**, what actually goes to a session: its text and HTML
forms, and the `kind` it came from (the view's name, or one of the engine's
own kinds: `text`, `system`, `echo`, `prompt`).

- **Feed messages** become lines in the text feed.
- **State updates** (`vitals`, `room_contents`, `map`) are data: the web
  client draws bars and panels, telnet folds them into the prompt and GMCP.
  (Planned.)
- Views can have named **sections** other plugins add to (a minimap on the
  room description). See Sections below.
- **Every message has a text form**, so any client can always show it.
- Plain text still works: `o:send(text)` sends a line with no view.

### Views

A view is a template file in a plugin's `views/` directory, one per format:

```
views/
  say.txt.tmpl     required: telnet, and the web when there's no HTML
  say.html.tmpl    optional: the web
```

Most views only need the text template: the web shows it with color as
HTML and entities still clickable. Add HTML when the web should look
different, not just to get links.

Templates are Go templates (`text/template` for `.txt`, `html/template` for
`.html`, which escapes data). Data keeps the names scripts gave it, so
`{ actor = actor }` in Lua is `{{.actor}}` in a template, and an object's
`name` property is `{{.actor.name}}`.

```lua
actor:send("dance", { actor = actor })
game.broadcast("dance", { actor = actor }, nil, actor)
```

- **The engine renders the whole template**, and makes no assumptions about
  how it's built. A template may define named blocks
  (`{{define "actor"}}...{{end}}`); the engine renders one only when the
  sender names it as the third argument:
  `actor:send("hit", data, "actor")`. Which block a player sees (their own
  view of an action, someone else's) is the sending plugin's choice; the
  engine never picks one.
- **Precedence** is per file: the game, then plugins, then built-ins. A game
  can restyle a plugin's HTML by adding only `game/views/say.html.tmpl`.
- A view without a `.txt.tmpl` is a startup error, and so is a misnamed file
  in `views/`. Sending an unknown view or block is an error naming the
  view, the file and the blocks it does define.
- Output is trimmed of blank lines at either end, so blocks can sit on their
  own lines.
- Text templates can use color codes (`[c]...[x]`).
- Hot reload picks up template changes like Lua changes.

**Objects in data** become entities. An entity has `id`, `key` and every
property the object has or inherits (`{{.actor.name}}`). Objects held in an
entity's properties are entities with only `id`, `key` and `name`, so one
message can't pull in the whole world. `id` and `key` take precedence over
properties with those names.

**`{{entity .actor}}`** writes an entity's name: its `name` property, else
its key, else "something". In text that's all; in HTML it's a clickable
`<dragon-entity>` (§6). Write the element yourself to choose its text:
`<dragon-entity ref="{{.actor.id}}">the {{.actor.name}}</dragon-entity>`.

### Sections

A template marks a place other plugins can add to with
`{{section "exits"}}`. Plugins fill it by handling the hook
`section:<view>.<section>`, so sections are ordered with `before` and
`after`, rearranged in `game/wiring.lua`, and listed by `dragon hooks`,
like any hook:

```lua
-- mapping/hooks.lua
return {
  ["section:room.exits"] = function(event)
    table.insert(event.parts, { view = "minimap", data = { room = event.data.room } })
    return event
  end,
}
```

- The event has `data`, the view's data as the sender gave it (objects
  are still objects), and `parts`, a list each handler adds to. A part is
  text, or `{ view, data[, block] }`, another view rendered in the same
  format, so the plugin adding a part owns how it looks. Parts are joined with line breaks in text.
- A handler that cancels leaves the section empty. An empty section
  renders as nothing, so `{{with section "exits"}}Exits: {{.}}{{end}}`
  shows a heading only when there's something under it.
- The hook runs once per message, not once per format.
- Startup fails for a section hook whose view doesn't exist or whose
  template has no such section, and for an HTML template missing a section
  its text template has, since parts would never reach the web.
- A part can't have sections of its own.

### Text layout

Telnet wraps every line longer than `[telnet] wrap` (80 by default)
between words, so prose never needs laying out by hand. Color codes take
no room, wide characters take two columns, and continuation lines keep the
line's indentation. `wrap = 0` leaves wrapping to the client, as Mudlet
and screen-reader setups often prefer.

Templates get layout helpers for what wrapping can't do. In text they pad
with spaces to the wrap width (80 when the client wraps); rendered for the
web, they become HTML the stylesheet lays out, so nothing is padded in a
proportional font. HTML templates can use them too.

```
{{columns 3 .classes}}       up to 3 columns, top to bottom then across;
                             fewer when the items don't fit
{{table .rows}}              columns sized to their content; the last one
{{table .header .rows}}      wraps to fit
{{center .title}}
{{rule}}  {{rule "="}}       a line across the width
{{indent 4 .text}}           wrapped, every line indented
{{pad 10 .name}}             at least 10 wide; padleft aligns right
```

Items can be entities: they show their names, clickable on the web.
Per-session widths (telnet NAWS) are planned; templates would then render
once per width in use.

### Renderers

Messages are rendered on the game loop when they're sent, once per format,
so a template error reaches the script that sent it.

- **Telnet:** the text form, wrapped, with ANSI color (none, 16-color
  fallback, 256-color). Prompt and GMCP are planned.
- **Web:** the HTML form, or the text form with color and entities as HTML,
  pushed over a WebSocket into named regions of the page (§6).

### Input parity

Every web interaction is a command. Clicking an action bar slot sends
`cast fireball goblin`. There are no web-only actions, only web-only
shortcuts.

### Accessibility

The web feed is an `aria-live` region of real text. Telnet is first-class
because many blind players use it with screen readers.

## 6. The web client

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

- **Feed structure** comes from views: `room` renders as a heading
  with prose, `say` and `emote` as dialogue and action, `ambient` in italics,
  `echo` (the player's own command) small and muted. Each message is a
  `.msg-<kind>` element, so a stylesheet can style any kind. `dragon:chat`
  sends `say` and `emote`; `dragon new` puts `room` and `ambient` in the
  game's `views/` until `dragon:rooms` provides them; the engine sends
  `echo`, `system` and `prompt`.
- **Entities in text are clickable** (see Entities below).
- **Phones:** `#context` becomes a bottom sheet that slides up when a fixture
  opens; `#side` becomes a drawer.

### Entities

`<dragon-entity ref="id">name</dragon-entity>` marks an object in HTML.
The name is real text, so the feed reads correctly without JavaScript and
in screen readers. The core client makes each one a control:

- **Tooltip** on hover, keyboard focus or a long press on touch screens,
  never hover only. The client asks the server, which runs the
  `get_tooltip` hook with `viewer` and `entity` and renders the template
  `templates/entity_tooltip.html.tmpl` (or `.txt.tmpl`) with the event as
  its data. A handler
  sets `event.block` to render one block of it, adds anything else the
  template needs to the event, or cancels for no tooltip. With no block the
  whole template renders; with no template there's no tooltip. Tooltips are
  rendered when they open, so they're never stale.
- **Default action** on click, Enter or Space. The server runs the
  `get_default_action` hook with `viewer` and `entity`; a handler sets
  `event.command` (like `"attack #" .. event.entity.id`) and the server runs
  it as if the player typed it, echoing it in their feed. With no command,
  nothing happens. Clicking is only ever a shortcut for a command (input
  parity).

The engine never decides what a tooltip shows or what clicking does: games
and plugins write both hooks and the template. Any object can be asked
about by id; handlers that hide things cancel `get_tooltip`.

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

A plugin asks one player for a specific answer ("Wen is waiting for an
answer") by pushing an input mode and prompting from its `enter` (§4):
numbered choices, free text, or a yes/no. While the mode waits, the
player's input goes to it instead of the commands. Telnet prints the
choices as a numbered list and the player types the number; the web makes
them clickable. Moving prompts into a `#prompt` slot comes with the
default layout.

### Vitals

Vitals are defined by the game, not assumed to be HP/mana: a list of
`{ name, value, max?, text?, color }` (a "Warmth: low" bar is valid). Web
draws bars; telnet puts them in the prompt.

## 7. Transports

Each transport is a listener that creates sessions. The game loop doesn't
know which are running.

```toml
[telnet]
enabled = true
address = ":4000"
wrap = 80

[web]
address = ":8080"

[web.client]
enabled = true

[web.admin]
enabled = true
```

Startup fails if no play transport is enabled, and warns if admin is exposed
publicly without an admin account.

### Logs

Each `[[log]]` table is a place logs go, with its own level and format:

```toml
[[log]]
target = "stderr"          # stderr, stdout, or a file relative to the game
format = "pretty"          # plain (default), pretty (colored), json
level = "info"             # debug, info (default), warn, error

[[log]]
target = "logs/game.log"
format = "json"
level = "debug"
```

With no `[[log]]` tables, logs go to stderr as plain text at info.
`NO_COLOR` turns pretty's color off. A dragon greets the server on stderr
when it starts, remarks on reloads, speaks up after 15 quiet minutes with
nothing logged (so a silent server is visibly alive), and says goodbye when
it stops; `dragon = false` turns it off. Lines carry a `prefix` naming the part
of the engine that wrote them (`game`, `web`, `telnet`, `store`).

## 8. Fairness

Client parity only matters for PvP. The engine provides the mechanisms; games
choose whether to use them: server-side cooldowns and timing, GMCP so telnet
clients get the same state, and rate limits that apply equally to every
transport.

## 9. Storage, export and import

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

## 10. Scripting

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
  `dofile`, `loadfile` and the standard `require` are removed.
- **Each plugin has its own scope.** A plugin's files share globals of
  their own, falling back to the engine's modules and libraries, so one
  plugin's globals never collide with another's. `require("items")` loads
  `lua/items.lua` (or `lua/items/init.lua`) from the same plugin, once per
  load; `require("items.find")` loads `lua/items/find.lua`. It works inside
  functions as well as at the top of a file. Another plugin's modules are
  out of reach; its public API comes through `plugin.require` (Milestone
  3). Third-party Lua is copied into `lua/`: pure Lua 5.1 that sticks to
  the sandbox's libraries works, C modules don't.
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
  `o:is_a(other)` is true when `o` is `other` or inherits from it at any
  depth, so a lock that requires a key accepts every copy made from it.
  `o:send(text)` and `o:send(view, data[, block])` (§5) reach everyone
  playing `o` and do nothing otherwise. Views will also reach `o`'s own
  script (§2).
- **Sessions** are handles too: input modes get one, and
  `game.session(o)` finds the session playing `o`. Fields: `account`,
  `character` (either can be nil) and `mode`. Methods: `send`, `prompt`,
  `push_mode`, `pop_mode`, `replace_mode`, `play` and `close` (§4). An
  **account** handle has `name` and `characters`, and `add_character(o)`.
  The `world` module creates, finds and destroys objects, and the `forms`
  module builds form sets (§2). `o:is_player()` is true when an account
  owns `o` as a character. A command's actor
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

## 11. Trust tiers

- **Plugins** are installed by the game owner and trusted, within the
  capabilities they declare (see [plugins.md](plugins.md#capabilities)).
- **In-game code** written by players (MUSH softcode, MOO verbs) is untrusted:
  a restricted module set, per-player CPU and memory limits, stored in the
  object store, and output limited to text and color markup, never
  JavaScript. It's called through the same entity script interface as
  trusted code (§2).

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
| `message`          | Messages sent to sessions.                              |
| `view`             | Views: templates, layout helpers, sections.             |
| `session`          | Player sessions and their outgoing queues.              |
| `transport/telnet` | Telnet listener and renderer.                           |
| `transport/web`    | HTTP server, WebSocket game client, admin UI.           |
| `store`            | SQLite persistence.                                     |
| `scripting`        | Language-neutral engine interface, modules, values.     |
| `scripting/lua`    | The Lua implementation (gopher-lua).                    |
| `ansi`             | `[r]color[x]` codes to ANSI escapes and HTML.           |
| `random`           | Seedable random numbers and dice.                       |
| `termlog`          | Colored, aligned server logs for terminals (`slog`).    |
| `auth`             | Password hashing (Argon2id).                            |
