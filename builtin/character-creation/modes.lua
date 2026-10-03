-- The create_character mode makes a new character for the logged-in
-- account, then ends with session:pop_mode(character).
--
-- A character is made from a draft. The draft starts as { name = the
-- account's name }, and each step can add to it. When every step is done,
-- each field of the draft becomes a property of the new character.
--
-- Plugins add steps by handling the character_steps hook: each handler
-- adds the name of a mode to event.steps.
--
--   character_steps = function(event)
--     table.insert(event.steps, "choose_class")
--     return event
--   end,
--
-- Order steps against other plugins' with before and after, and rearrange
-- or disable them from game/wiring.lua, like any hook. Run
-- `dragon hooks character_steps` to see the order.
--
-- A step mode gets the draft as state.draft and ends with
-- session:pop_mode(changes), where changes is a table merged into the
-- draft. Ending with nothing changes nothing.

local function advance(session, state)
  local step = state.steps[state.next]
  if step then
    state.next = state.next + 1
    session:push_mode(step, { draft = state.draft })
    return state
  end

  local character = world.create({ properties = state.draft })
  session.account:add_character(character)
  session:pop_mode(character)
end

return {
  create_character = {
    desc = "Create a new character.",

    enter = function(session, state)
      local event, reason = hooks.run("character_steps", { account = session.account, steps = {} })
      if not event then
        if reason then
          session:send(reason)
        end
        session:pop_mode()
        return
      end

      state.steps = event.steps or {}
      state.draft = { name = session.account.name }
      state.next = 1
      return advance(session, state)
    end,

    resume = function(session, state, changes)
      if type(changes) == "table" then
        for k, v in pairs(changes) do
          state.draft[k] = v
        end
      end
      return advance(session, state)
    end,
  },
}
