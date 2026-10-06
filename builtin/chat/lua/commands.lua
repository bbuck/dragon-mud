-- Talking to other players.
--
-- Each command has forms: patterns players type, each with the function
-- that runs. <name> captures text; <name:type> captures and resolves it,
-- such as <target:object:here> for something in the room. When several
-- forms match, the most specific one whose slots resolve wins.
--
-- A game adds forms to these commands by defining a command with the same
-- name, or replaces one with replace = true. What players read comes from
-- views/say.txt.tmpl and views/emote.txt.tmpl; a game restyles them
-- with its own game/views/say.txt.tmpl, or say.html.tmpl for the web.
--
-- The commands are thin: what they do is in api.lua, which the game and
-- other plugins use too, through require("@dragon:chat").

local chat = require("api")

-- tell lets the actor know why chat stopped what they tried, if a handler
-- gave a reason.
local function tell(actor, ok, reason)
  if not ok and reason then
    actor:send(reason)
  end
end

local function say(actor, args)
  tell(actor, chat.say(actor, args.message))
end

local function say_to(actor, args)
  tell(actor, chat.say(actor, args.message, args.target))
end

local function emote(actor, args)
  tell(actor, chat.emote(actor, args.action))
end

return {
  say = {
    desc = "Say something. Shortcut: 'hello",
    forms = {
      { "say <message>", say },
      { "'<message>", say },
      { "say <message> to <target:object:here>", say_to, desc = "Say something to someone." },
    },
  },

  emote = {
    desc = "Act something out: emote waves. Shortcut: :waves",
    forms = {
      { "emote <action>", emote },
      { ":<action>", emote },
    },
  },
}
