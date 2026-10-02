-- The commands every game starts with.
--
-- Each command has forms: patterns players type, each with the function
-- that runs. <name> captures text; <name:type> captures and resolves it,
-- such as <target:object:here> for something in the room. When several
-- forms match, the most specific one whose slots resolve wins.
--
-- A game adds forms to these commands by defining a command with the same
-- name, or replaces one with replace = true.

local function pad(text, width)
  return text .. string.rep(" ", width - #text)
end

local function name(o)
  return o:get("name") or "something"
end

local function say(actor, args)
  actor:send('[c]You say, "' .. args.message .. '"[x]')
  game.broadcast("[c]" .. name(actor) .. ' says, "' .. args.message .. '"[x]', actor)
end

local function emote(actor, args)
  game.broadcast("[m]" .. name(actor) .. " " .. args.action .. "[x]")
end

return {
  look = {
    desc = "Look around.",
    forms = {
      { "look", function(actor)
          actor:send("[W]The Void[x]\nNothing has been built here yet. Define a [c]look[x] command in game/commands.lua to describe your world.")
        end },
    },
  },

  say = {
    desc = "Say something. Shortcut: 'hello",
    forms = {
      { "say <message>", say },
      { "'<message>", say },
      { "say <message> to <target:object:here,online>", function(actor, args)
          local target = args.target
          actor:send(('[c]You say to %s, "%s"[x]'):format(name(target), args.message))
          target:send(('[c]%s says to you, "%s"[x]'):format(name(actor), args.message))
        end, desc = "Say something to someone." },
    },
  },

  emote = {
    desc = "Act something out: emote waves. Shortcut: :waves",
    forms = {
      { "emote <action>", emote },
      { ":<action>", emote },
    },
  },

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

  help = {
    desc = "List commands, or show how to use one: help say",
    forms = {
      { "help", function(actor)
          local commands = game.commands()
          local width = 0
          for _, cmd in ipairs(commands) do
            width = math.max(width, #cmd.name)
          end

          local lines = { "[W]Commands:[x]" }
          for _, cmd in ipairs(commands) do
            table.insert(lines, "  [c]" .. pad(cmd.name, width) .. "[x]  " .. (cmd.desc or ""))
          end
          table.insert(lines, "Type [c]help <command>[x] to see how to use one.")
          actor:send(table.concat(lines, "\n"))
        end },

      { "help <topic:word>", function(actor, args)
          for _, cmd in ipairs(game.commands()) do
            if cmd.name == args.topic:lower() then
              local lines = { "[W]" .. cmd.name .. "[x]  " .. (cmd.desc or "") }
              for _, form in ipairs(cmd.forms) do
                local line = "  [c]" .. form.pattern .. "[x]"
                if form.desc and form.desc ~= "" then
                  line = line .. "  " .. form.desc
                end
                table.insert(lines, line)
              end
              actor:send(table.concat(lines, "\n"))
              return
            end
          end
          actor:send("There's no command called '" .. args.topic .. "'. Type [c]help[x] for a list.")
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
