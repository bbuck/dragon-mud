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
--   return { hooks = { ["dragon:player_connected"] = { disable = { "dragon:presence" } } } }

return {
  -- A notification sent once when the server starts, before anyone can
  -- type. A good place to make sure the world has what the game needs:
  --
  -- ["dragon:booted"] = function()
  --   if not world.keyed("start") then
  --     world.create({ key = "start", properties = { name = "The Dragon's Rest" } })
  --   end
  -- end,

  -- A notification: event.player has just entered the game.
  ["dragon:player_connected"] = function(event)
    if not event.reconnected then
      event.player:send("ambient", { text = "The barkeep looks up and nods at you." })
    end
  end,

  -- What clicking something in the web client does: set event.command to
  -- a command, and it runs as if the player typed it. "#id" names exactly
  -- the thing clicked. event.viewer is the player; event.entity the thing.
  ["dragon:get_default_action"] = function(event)
    event.command = "look #" .. event.entity.id
    return event
  end,

  -- Tooltips render templates/entity_tooltip.html.tmpl. Set event.block to
  -- render one {{define}} block from it, add data the template can use,
  -- or return false for no tooltip.
  --
  -- ["dragon:get_tooltip"] = function(event)
  --   if event.entity:get("hidden") then return false end
  -- end,

  -- A hook: change what's said, or cancel it with a reason.
  --
  -- before_say = function(event)
  --   if event.message:find("dragon") then
  --     return false, "You think better of mentioning dragons in here."
  --   end
  -- end,
}
