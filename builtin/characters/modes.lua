-- The characters mode, which the engine starts once a player has logged in.
-- It must end with session:play(character).
--
-- This one plays the account's only character, asks which to play when
-- there are several, and starts create_character (from
-- dragon:character-creation) when there are none.
--
-- To write your own character select, leave "characters" out of builtins in
-- dragon.toml and define a characters mode in game/modes.lua.

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

local function choose(session, question)
  local characters = session.account.characters
  if #characters == 0 then
    session:push_mode("create_character")
  elseif #characters == 1 then
    session:play(characters[1])
  else
    session:prompt({ text = question, choices = names(characters) })
  end
end

return {
  characters = {
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

    -- create_character ended with the new character, or nothing if it was
    -- cancelled.
    resume = function(session, state, character)
      if character then
        session:play(character)
      else
        choose(session, "Who will you play?")
      end
    end,
  },
}
