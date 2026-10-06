-- Commands for your game.
--
-- Each command has forms: patterns players type, each with the function
-- that runs. <name> captures text into args.name; <name:type> captures and
-- resolves it, such as <target:object:here> for something in the room.
-- When several forms match, the most specific one whose slots resolve wins.
--
-- actor is the player's character: actor:send(text) talks to them,
-- actor:get("name") is their name, and actor:set(...) stores anything you
-- like on them. Color codes like [Y]...[x] work everywhere.
--
-- actor:send(view, data, block) sends a view instead: a template in views/
-- filled in with data. See views/dance.txt.tmpl.
--
-- The engine's modules are loaded with require, like your own files here:
-- dragon.game, dragon.world, dragon.hooks, dragon.forms and
-- dragon.log. With local log = require("dragon.log"),
-- log.info("message", { player = actor }) writes to the server log, for
-- seeing what your code is doing; log.debug lines show when dragon.toml's
-- [[log]] has level = "debug".
--
-- Defining a command the engine already has (look, say, ...) adds your
-- forms to it. Set replace = true to use only yours.

local game = require("dragon.game")
local look = require("look") -- look.lua

return {
	look = {
		desc = "Look around.",
		replace = true,
		forms = {
			{ "look", look.room },
			{ "look <thing:object:here|held>", function(actor, args)
					actor:send("look_at", { thing = args.thing })
				end },
		},
	},

	describe = {
		desc = "Write how others see you when they look at you.",
		forms = {
			-- The editor mode (modes.lua) saves the text to actor's description.
			{ "describe", function(actor)
					game.session(actor):push_mode("editor", { target = actor, property = "description" })
				end },
		},
	},

	dance = {
		desc = "Dance a little jig, or dance with someone.",
		forms = {
			{ "dance", function(actor)
					local data = { actor = actor }
					actor:send("dance", data, "actor")
					-- Everyone else in the room sees the "others" block; skip
					-- the dancer.
					game.broadcast_to(actor.location, "dance", data, "others", actor)
				end },
			{ "dance with <partner:object:here,online>", function(actor, args)
					local data = { actor = actor, partner = args.partner }
					actor:send("dance_with", data, "actor")
					args.partner:send("dance_with", data, "partner")
					game.broadcast_to(actor.location, "dance_with", data, "others", { actor, args.partner })
				end },
		},
	},
}
