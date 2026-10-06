-- Announces players arriving and leaving.
--
-- Each key is a hook or notification the engine or a plugin runs, and each
-- value is the function that handles it. A game turns these off, or changes
-- the order handlers run in, with wiring in its init.lua.

local game = require("dragon.game")

local function name(o)
  return o:get("name") or "someone"
end

-- announce tells everyone where actor is, or the whole game if actor is
-- nowhere.
local function announce(actor, text, except)
  if actor.location then
    game.broadcast_to(actor.location, text, except)
  else
    game.broadcast(text, except)
  end
end

return {
  -- event.actor, a player, has entered the game. event.reconnected is true
  -- when they took over their character from another connection, so to
  -- everyone else they never left. This runs after the game's handler,
  -- which is where a game puts new characters somewhere.
  ["dragon:player_connected"] = {
    after = { "game" },
    handler = function(event)
      if not event.reconnected then
        announce(event.actor, name(event.actor) .. " has arrived.", event.actor)
      end
    end,
  },

  -- event.actor, a player, has left the game.
  ["dragon:player_disconnected"] = function(event)
    announce(event.actor, name(event.actor) .. " has left.")
  end,
}
