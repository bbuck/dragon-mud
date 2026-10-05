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

local game = require("dragon.game")
local hooks = require("dragon.hooks")

-- dragon:before_say lets other plugins change what's said, or stop it. It
-- returns the message to say, or nil if a handler cancelled.
local function before_say(actor, message, target)
  local event, reason = hooks.run("dragon:before_say", { actor = actor, message = message, target = target })
  if not event then
    if reason then
      actor:send(reason)
    end
    return nil
  end

  return event.message
end

local function say(actor, args)
  local message = before_say(actor, args.message)
  if not message then
    return
  end

  local data = { actor = actor, message = message }
  actor:send("say", data, "actor")
  game.broadcast("say", data, "others", actor)
end

local function emote(actor, args)
  game.broadcast("emote", { actor = actor, action = args.action })
end

return {
  say = {
    desc = "Say something. Shortcut: 'hello",
    forms = {
      { "say <message>", say },
      { "'<message>", say },
      { "say <message> to <target:object:here>", function(actor, args)
          local target = args.target
          local message = before_say(actor, args.message, target)
          if not message then
            return
          end

          local data = { actor = actor, message = message, target = target }
          actor:send("say", data, "actor")
          target:send("say", data, "target")
        end, desc = "Say something to someone." },
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
