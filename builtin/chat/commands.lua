-- Talking to other players.
--
-- Each command has forms: patterns players type, each with the function
-- that runs. <name> captures text; <name:type> captures and resolves it,
-- such as <target:object:here> for something in the room. When several
-- forms match, the most specific one whose slots resolve wins.
--
-- A game adds forms to these commands by defining a command with the same
-- name, or replaces one with replace = true. What players read comes from
-- views/say.txt.tmpl and views/emote.txt.tmpl; a game restyles them
-- with its own game/views/say.txt.tmpl, or say.html.tmpl for the web.

local game = require("dragon.game")
local hooks = require("dragon.hooks")

-- Each action runs a hook before it, which other plugins use to change it
-- or stop it, and a notification after it, for reacting to it: an NPC
-- answers dragon:said, so its reply comes after the line it answers. The
-- hooks are declared in events.lua, which init.lua exports as events.declare.

-- before runs the hook, telling the actor why if a handler cancelled. It
-- returns the event as the handlers left it, or nil if one cancelled.
local function before(hook, event)
  local result, reason = hooks.run(hook, event)
  if not result and reason then
    event.actor:send(reason)
  end

  return result
end

-- around sends to everyone where actor is, or to the whole game if actor
-- is nowhere.
local function around(actor, kind, data, block, except)
  if actor.location then
    game.broadcast_to(actor.location, kind, data, block, except)
  else
    game.broadcast(kind, data, block, except)
  end
end

local function say(actor, args)
  local event = before("dragon:before_say", { actor = actor, message = args.message })
  if not event then
    return
  end

  local data = { actor = actor, message = event.message }
  actor:send("say", data, "actor")
  around(actor, "say", data, "others", actor)
  hooks.notify("dragon:said", data)
end

local function say_to(actor, args)
  local event = before("dragon:before_say", { actor = actor, message = args.message, target = args.target })
  if not event then
    return
  end

  local data = { actor = actor, message = event.message, target = args.target }
  actor:send("say", data, "actor")
  args.target:send("say", data, "target")
  hooks.notify("dragon:said", data)
end

local function emote(actor, args)
  local event = before("dragon:before_emote", { actor = actor, action = args.action })
  if not event then
    return
  end

  local data = { actor = actor, action = event.action }
  around(actor, "emote", data)
  hooks.notify("dragon:emoted", data)
end

return {
  say = {
    desc = "Say something. Shortcut: 'hello",
    forms = {
      { "say <message>", say },
      { "'<message>", say },
      { "say <message> to <target:object:here>", say_to, desc = "Say something to someone." },
    },
  },

  emote = {
    desc = "Act something out: emote waves. Shortcut: :waves",
    forms = {
      { "emote <action>", emote },
      { ":<action>", emote },
    },
  },
}
