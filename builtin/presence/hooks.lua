-- Announces players arriving and leaving.
--
-- Each key is a hook or notification the engine or a plugin runs, and each
-- value is the function that handles it. A game turns these off, or changes
-- the order handlers run in, from game/wiring.lua.

local function name(o)
  return o:get("name") or "someone"
end

return {
  -- event.player has entered the game. event.reconnected is true when they
  -- took over their character from another connection, so to everyone else
  -- they never left.
  player_entered = function(event)
    if not event.reconnected then
      game.broadcast(name(event.player) .. " has arrived.", event.player)
    end
  end,

  -- event.player has left the game.
  player_left = function(event)
    game.broadcast(name(event.player) .. " has left.")
  end,
}
