-- What a player sees when they look around. Modules in lua/ are shared by
-- your game's files: commands.lua and hooks.lua both load this one with
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
