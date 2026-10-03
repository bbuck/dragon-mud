-- What a player sees when they look around. Modules in lua/ are shared by
-- your game's files: commands.lua and hooks.lua both load this one with
-- require("look").

local look = {}

function look.room(actor)
  actor:send("room", {
    title = "The Dragon's Rest",
    description = "A low-beamed tavern, warm with the smell of woodsmoke and spiced cider. "
      .. "A fire crackles in a hearth carved to look like a sleeping dragon.",
  })
end

return look
