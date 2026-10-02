-- Commands for your game. Each entry is a command players can type.
--
-- execute receives the player (actor.id, actor.name) and whatever they
-- typed after the command. Color codes like [Y]...[x] work everywhere.
--
-- To replace a built-in command such as look or say, set override = true.

return {
  look = {
    desc = "Look around.",
    override = true,
    execute = function(actor)
      game.send(actor.id, table.concat({
        "[Y]The Dragon's Rest[x]",
        "A low-beamed tavern, warm with the smell of woodsmoke and spiced cider.",
        "A fire crackles in a hearth carved to look like a sleeping dragon.",
      }, "\n"))
    end,
  },

  dance = {
    desc = "Dance a little jig.",
    execute = function(actor)
      game.send(actor.id, "You dance a little jig.")
      game.broadcast(actor.name .. " dances a little jig.", actor.id)
    end,
  },
}
