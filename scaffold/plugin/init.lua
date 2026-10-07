-- The {{.Name}} plugin. This file returns everything it provides, built
-- from its modules in lua/: require("commands") loads lua/commands.lua.
return {
	-- What players can type.
	commands = require("commands"),

	events = {
		-- What the plugin does when events happen.
		handlers = require("handlers"),
	},
}
