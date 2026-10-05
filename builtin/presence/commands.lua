-- Who's in the game, and leaving it.

local game = require("dragon.game")

local function name(o)
  return o:get("name") or "someone"
end

return {
  who = {
    desc = "See who is online.",
    forms = {
      { "who", function(actor)
          local players = game.players()
          local lines = { "[W]Online in " .. game.name .. ":[x]" }
          for _, p in ipairs(players) do
            table.insert(lines, "  " .. name(p))
          end
          table.insert(lines, #players .. (#players == 1 and " player." or " players."))
          actor:send(table.concat(lines, "\n"))
        end },
    },
  },

  quit = {
    desc = "Leave the game.",
    forms = {
      { "quit", function(actor)
          game.disconnect(actor, "Farewell, " .. name(actor) .. "!")
        end },
    },
  },
}
