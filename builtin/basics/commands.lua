-- The commands every game starts with. A game can replace any of them by
-- defining a command with the same name and `override = true`.

local function pad(text, width)
  return text .. string.rep(" ", width - #text)
end

return {
  look = {
    desc = "Look around.",
    execute = function(actor)
      game.send(actor.id, "[W]The Void[x]\nNothing has been built here yet. Define a [c]look[x] command in game/commands.lua to describe your world.")
    end,
  },

  say = {
    desc = "Say something to everyone. Shortcut: 'hello",
    execute = function(actor, text)
      if text == "" then
        game.send(actor.id, "Say what?")
        return
      end

      game.send(actor.id, '[c]You say, "' .. text .. '"[x]')
      game.broadcast("[c]" .. actor.name .. ' says, "' .. text .. '"[x]', actor.id)
    end,
  },

  emote = {
    desc = "Act something out: emote waves.",
    execute = function(actor, text)
      if text == "" then
        game.send(actor.id, "Emote what?")
        return
      end

      game.broadcast("[m]" .. actor.name .. " " .. text .. "[x]")
    end,
  },

  who = {
    desc = "See who is online.",
    execute = function(actor)
      local players = game.players()
      local lines = { "[W]Online in " .. game.name .. ":[x]" }
      for _, p in ipairs(players) do
        table.insert(lines, "  " .. p.name)
      end
      table.insert(lines, #players .. (#players == 1 and " player." or " players."))
      game.send(actor.id, table.concat(lines, "\n"))
    end,
  },

  help = {
    desc = "List commands.",
    execute = function(actor)
      local commands = game.commands()
      local width = 0
      for _, cmd in ipairs(commands) do
        width = math.max(width, #cmd.name)
      end

      local lines = { "[W]Commands:[x]" }
      for _, cmd in ipairs(commands) do
        table.insert(lines, "  [c]" .. pad(cmd.name, width) .. "[x]  " .. (cmd.desc or ""))
      end
      game.send(actor.id, table.concat(lines, "\n"))
    end,
  },

  quit = {
    desc = "Leave the game.",
    execute = function(actor)
      game.disconnect(actor.id, "Farewell, " .. actor.name .. "!")
    end,
  },
}
