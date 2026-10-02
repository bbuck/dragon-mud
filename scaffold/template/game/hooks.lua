-- Hook handlers for your game.
--
-- Hooks let you change or stop what other code is about to do; a handler
-- returns nothing to leave the event alone, the event to change it, or
-- false and a reason to cancel it. Notifications say something already
-- happened, so their handlers just react.
--
-- Handlers run in load order: built-in plugins, then installed plugins,
-- then this file, so your game has the last word. Run `dragon hooks` to see
-- every handler, and `dragon hooks before_say` to see one hook's order. To
-- turn another plugin's handler off or reorder them, return a table from
-- game/wiring.lua:
--
--   return { hooks = { player_entered = { disable = { "dragon:presence" } } } }

return {
  -- A notification: event.player has just entered the game.
  player_entered = function(event)
    if not event.reconnected then
      event.player:send("The barkeep looks up and nods at you.")
    end
  end,

  -- A hook: change what's said, or cancel it with a reason.
  --
  -- before_say = function(event)
  --   if event.message:find("dragon") then
  --     return false, "You think better of mentioning dragons in here."
  --   end
  -- end,
}
