-- Help for players: what commands there are and how to use them.

local function pad(text, width)
  return text .. string.rep(" ", width - #text)
end

return {
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
}
