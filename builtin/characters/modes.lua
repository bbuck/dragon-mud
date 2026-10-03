-- Choosing and creating characters after login.
--
-- After a player logs in, the engine starts the game's characters mode if
-- it has one, otherwise dragon:characters. Either must end with
-- session:play(character). dragon:characters plays the account's only
-- character, asks which to play when there are several, and starts
-- dragon:create_character when there are none.
--
-- dragon:create_character makes a character from a draft. The draft starts as
-- { name = the account's name }, and each step can add to it. When every
-- step is done, each field of the draft becomes a property of the new
-- character, and dragon:create_character ends with
-- session:pop_mode(character).
-- With no steps, the character is made straight away.
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
--
-- To write your own character select, define a characters mode in
-- game/modes.lua; it can still push dragon:create_character. For a
-- different creation flow, add steps. For entirely your own, leave
-- "characters" out of builtins in dragon.toml.

local function name(o)
  return o:get("name") or "someone"
end

local function names(characters)
  local list = {}
  for i, c in ipairs(characters) do
    list[i] = name(c)
  end
  return list
end

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

local function choose(session, question)
  local characters = session.account.characters
  if #characters == 0 then
    session:push_mode("dragon:create_character")
  elseif #characters == 1 then
    session:play(characters[1])
  else
    session:prompt({ text = question, choices = names(characters) })
  end
end

return {
  ["dragon:characters"] = {
    desc = "Choose a character to play.",

    enter = function(session)
      choose(session, "Who will you play?")
    end,

    -- line is a character's name; a number picks from the list.
    input = function(session, line)
      for _, c in ipairs(session.account.characters) do
        if string.lower(name(c)) == string.lower(line) then
          session:play(c)
          return
        end
      end
      choose(session, "You have no character called '" .. line .. "'. Who will you play?")
    end,

    -- dragon:create_character ended with the new character, or nothing if it was
    -- cancelled. With no character to fall back on, there's nothing to
    -- play, and asking again would only be cancelled again.
    resume = function(session, state, character)
      if character then
        session:play(character)
      elseif #session.account.characters == 0 then
        session:close("You can't create a character right now.")
      else
        choose(session, "Who will you play?")
      end
    end,
  },

  ["dragon:create_character"] = {
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
