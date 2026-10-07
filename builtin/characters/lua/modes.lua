-- Choosing and creating characters after login.
--
-- After a player logs in, the engine starts the game's characters mode if
-- it has one, otherwise dragon:characters. Either must end with
-- session:play(character). dragon:characters plays the account's only
-- character, asks which to play when there are several, and starts
-- dragon:create_character when there are none.
--
-- dragon:create_character makes a character from a draft: the options
-- world.create takes. It starts as { properties = { name = the account's
-- name, article = false } }, and each step can add to it. When every step is done, the
-- draft is passed to world.create, the dragon:character_created
-- notification is sent (character, account), and dragon:create_character
-- ends with session:pop_mode(character). With no steps, the character is
-- made straight away.
--
-- Plugins add steps by handling the dragon:character_steps hook: each
-- handler adds the name of a mode to event.steps.
--
--   ["dragon:character_steps"] = function(event)
--     table.insert(event.steps, "choose_class")
--     return event
--   end,
--
-- Order steps against other plugins' with before and after, and rearrange
-- or disable them in the game's wiring, like any hook. Run
-- `dragon events dragon:character_steps` to see the order.
--
-- A step mode gets the draft as state.draft and ends with
-- session:pop_mode(changes), where changes look like world.create's
-- options: { parent = elf }, { location = village }, or
-- { properties = { class = "ranger" } }. Properties are merged one by one;
-- anything else replaces what the draft had. Ending with nothing changes
-- nothing.
--
-- Setup that needs the character to exist, such as starting equipment,
-- belongs in a dragon:character_created handler.
--
-- To write your own character select, define a characters mode in
-- game/modes.lua; it can still push dragon:create_character. For a
-- different creation flow, add steps. For entirely your own, leave
-- "characters" out of builtins in dragon.toml.

local world = require("dragon.world")
local events = require("dragon.events")

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

  local character = world.create(state.draft)
  session.account:add_character(character)
  events.notify("dragon:character_created", { character = character, account = session.account })
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
      local event, reason = events.run("dragon:character_steps", { account = session.account, steps = {} })
      if not event then
        if reason then
          session:send(reason)
        end
        session:pop_mode()
        return
      end

      state.steps = event.steps or {}
      -- article = false: a character's name is a proper name, so
      -- {{the .x}} writes "Alice".
      state.draft = { properties = { name = session.account.name, article = false } }
      state.next = 1
      return advance(session, state)
    end,

    resume = function(session, state, changes)
      if type(changes) == "table" then
        for k, v in pairs(changes) do
          if k == "properties" and type(v) == "table" then
            for name, value in pairs(v) do
              state.draft.properties[name] = value
            end
          else
            state.draft[k] = v
          end
        end
      end
      return advance(session, state)
    end,
  },
}
