-- What a player sees when they look around. Any file in lua/ can be shared
-- with require: commands.lua and handlers.lua both load this one with
-- require("look").

local look = {}

function look.room(actor)
	if not actor.location then
		actor:send("You are nowhere at all.")
		return
	end

	actor:send("room", { room = actor.location })
end

return look
