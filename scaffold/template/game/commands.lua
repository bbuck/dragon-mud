-- Commands for your game.
--
-- Each command has forms: patterns players type, each with the function
-- that runs. <name> captures text into args.name; <name:type> captures and
-- resolves it, such as <target:object:here> for something in the room.
-- When several forms match, the most specific one whose slots resolve wins.
--
-- actor is the player's character: actor:send(text) talks to them,
-- actor:get("name") is their name, and actor:set(...) stores anything you
-- like on them. Color codes like [Y]...[x] work everywhere.
--
-- Defining a command the engine already has (look, say, ...) adds your
-- forms to it. Set replace = true to use only yours.

return {
  look = {
    desc = "Look around.",
    replace = true,
    forms = {
      { "look", function(actor)
          actor:send(table.concat({
            "[Y]The Dragon's Rest[x]",
            "A low-beamed tavern, warm with the smell of woodsmoke and spiced cider.",
            "A fire crackles in a hearth carved to look like a sleeping dragon.",
          }, "\n"))
        end },
    },
  },

  dance = {
    desc = "Dance a little jig, or dance with someone.",
    forms = {
      { "dance", function(actor)
          actor:send("You dance a little jig.")
          game.broadcast(actor:get("name") .. " dances a little jig.", actor)
        end },
      { "dance with <partner:object:here,online>", function(actor, args)
          local partner = args.partner
          actor:send("You whirl " .. partner:get("name") .. " around the room.")
          partner:send(actor:get("name") .. " whirls you around the room.")
        end },
    },
  },
}
