-- This directory is your game's own plugin. It loads after every other
-- plugin, so it can add to or replace anything they provide.
--
-- This file returns everything the game provides. Each part comes from its
-- own file here, loaded with require, but the names of the files are up to
-- you: only what this table holds matters.
return {
	-- What players can type.
	commands = require("commands"),

	-- Input modes: prompts, menus and editors that take over a player's input.
	modes = require("modes"),

	events = {
		-- What the game does when hooks and notifications run.
		handlers = require("handlers"),
	},
}
