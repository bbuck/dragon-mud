-- Input modes for your game.
--
-- A mode takes over a player's input until it ends: a question, a menu, an
-- editor. Start one with session:push_mode(name, state) and end it with
-- session:pop_mode(). In a command, game.session(actor) is the player's
-- session; in a mode, session.character is the player.
--
-- state is plain data the mode keeps while it runs. Make modes reusable by
-- giving them what to do as data, like the editor below: anything that
-- wants text typed in pushes it with a target and a property, and nothing
-- has to handle a result afterwards.
--
-- Name your game's modes however you like. Plugins namespace theirs, like
-- mapping:edit_map, so they don't collide.

return {
  -- session:push_mode("editor", { target = object, property = "description" })
  editor = {
    desc = "Write several lines of text.",

    enter = function(session, state)
      state.lines = {}
      session:prompt("Type your text a line at a time. A line with only [c].[x] saves it; [c]~q[x] cancels.")
      return state
    end,

    input = function(session, line, state)
      if line == "." then
        state.target:set(state.property, table.concat(state.lines, "\n"))
        session:send("Saved.")
        session:pop_mode()
      elseif line == "~q" then
        session:send("Cancelled.")
        session:pop_mode()
      else
        table.insert(state.lines, line)
        return state
      end
    end,
  },
}
