-- Hook handlers for your game.
--
-- Hooks let you change or stop what other code is about to do; a handler
-- returns nothing to leave the event alone, the event to change it, or
-- false and a reason to cancel it. Notifications say something already
-- happened, so their handlers just react.
--
-- Handlers run in load order: built-in plugins, then installed plugins,
-- then this file, so your game has the last word. Run `dragon hooks` to see
-- every hook, and `dragon hooks dragon:before_say` to see one hook's fields
-- and the order its handlers run in. To turn another plugin's handler off or
-- reorder them, add wiring to events in init.lua:
--
--   wiring = { ["dragon:player_connected"] = { disable = { "dragon:presence" } } }

local world = require("dragon.world")
local look = require("look") -- look.lua

return {
	-- A notification sent once when the server starts, before anyone can
	-- type: the place to make sure the world has what the game needs. The
	-- first time the game runs, this makes the tavern everyone starts in.
	-- After that it's in the database, and changing it here changes nothing.
	["dragon:booted"] = function()
		if not world.keyed("tavern") then
			world.create({ key = "tavern", properties = {
				name = "The Dragon's Rest",
				description = "A low-beamed tavern, warm with the smell of woodsmoke and spiced cider. "
					.. "A fire crackles in a hearth carved to look like a sleeping dragon.",
			} })
		end
	end,

	-- A notification: event.actor, a player, has just entered the game, or
	-- taken their character over from another connection
	-- (event.reconnected). New characters are nowhere until something puts
	-- them somewhere.
	["dragon:player_connected"] = function(event)
		local player = event.actor
		if not player.location then
			player:move_to(world.keyed("tavern"))
		end
		if not event.reconnected then
			player:send("ambient", { text = "The barkeep looks up and nods at you." })
		end
		look.room(player)
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
	-- ["dragon:before_say"] = function(event)
	--   if event.message:find("dragon") then
	--     return false, "You think better of mentioning dragons in here."
	--   end
	-- end,

	-- A notification: something was said, and everyone heard it. Answer
	-- here, so the answer comes after the line it answers.
	--
	-- ["dragon:said"] = function(event)
	--   if event.message:lower():find("cider") then
	--     event.actor:send("ambient", { text = "The barkeep slides a mug of cider your way." })
	--   end
	-- end,
}
