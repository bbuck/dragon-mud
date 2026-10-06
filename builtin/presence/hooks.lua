-- Announces players arriving and leaving.
--
-- Each key is a hook or notification the engine or a plugin runs, and each
-- value is the function that handles it. A game turns these off, or changes
-- the order handlers run in, from game/wiring.lua.

local game = require("dragon.game")

local function name(o)
  return o:get("name") or "someone"
end

-- announce tells everyone where player is, or the whole game if player is
-- nowhere.
local function announce(player, text, except)
  if player.location then
    game.broadcast_to(player.location, text, except)
  else
    game.broadcast(text, except)
  end
end

return {
  -- event.player has entered the game. event.reconnected is true when they
  -- took over their character from another connection, so to everyone else
  -- they never left. This runs after the game's handler, which is where a
  -- game puts new characters somewhere.
  ["dragon:player_connected"] = {
    after = { "game" },
    handler = function(event)
      if not event.reconnected then
        announce(event.player, name(event.player) .. " has arrived.", event.player)
      end
    end,
  },

  -- event.player has left the game.
  ["dragon:player_disconnected"] = function(event)
    announce(event.player, name(event.player) .. " has left.")
  end,
}
