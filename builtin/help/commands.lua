-- Help for players: what commands there are and how to use them.

return {
  help = {
    desc = "List commands, or show how to use one: help say",
    forms = {
      -- messages/help_commands.txt.tmpl lays the list out with {{table}}.
      { "help", function(actor)
          local rows = {}
          for _, cmd in ipairs(game.commands()) do
            table.insert(rows, { "[c]" .. cmd.name .. "[x]", cmd.desc or "" })
          end
          actor:send("help_commands", { rows = rows })
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
