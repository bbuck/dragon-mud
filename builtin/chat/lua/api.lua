-- The chat API: speech and emotes for anyone, players or not. Other
-- plugins and the game import it with require("@dragon:chat"), so they
-- speak through chat instead of sending its views themselves:
--
--   local chat = require("@dragon:chat")
--   chat.say(keeper, "What'll it be?")
--
-- Each action runs a hook before it, which other plugins use to change it
-- or stop it, and a notification after it, for reacting to it: an NPC
-- answers dragon:said, so its reply comes after the line it answers. The
-- events are declared in events.lua.

local game = require("dragon.game")
local events = require("dragon.events")

local chat = {}

-- around sends to everyone where actor is, or to the whole game if actor
-- is nowhere.
local function around(actor, kind, data, block, except)
  if actor.location then
    game.broadcast_to(actor.location, kind, data, block, except)
  else
    game.broadcast(kind, data, block, except)
  end
end

-- say has actor say message: to everyone around them, or only to target
-- if one is given. It returns true, or nil and the reason a
-- dragon:before_say handler gave for stopping it.
function chat.say(actor, message, target)
  local event, reason = events.run("dragon:before_say", { actor = actor, message = message, target = target })
  if not event then
    return nil, reason
  end

  local data = { actor = actor, message = event.message, target = target }
  actor:send("say", data, "actor")
  if target then
    target:send("say", data, "target")
  else
    around(actor, "say", data, "others", actor)
  end
  events.notify("dragon:said", data)

  return true
end

-- emote has actor act something out, like "waves.", for everyone around
-- them. It returns true, or nil and the reason a dragon:before_emote
-- handler gave for stopping it.
function chat.emote(actor, action)
  local event, reason = events.run("dragon:before_emote", { actor = actor, action = action })
  if not event then
    return nil, reason
  end

  local data = { actor = actor, action = event.action }
  around(actor, "emote", data)
  events.notify("dragon:emoted", data)

  return true
end

return chat
