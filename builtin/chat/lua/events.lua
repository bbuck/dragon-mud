-- The events chat sends, and the fields of each.
-- Each action has a hook before it, to change or stop it, and a
-- notification after it, to react to it. Run `dragon events <name>` to see
-- one, with its handlers.
return {
  ["dragon:before_say"] = {
    desc = "Someone is about to say something. Change event.message, or cancel with a reason they'll see.",
    fields = {
      actor = "who's speaking",
      message = "what they'll say",
      target = { desc = "who they're speaking to, when they name someone", optional = true },
    },
  },

  ["dragon:said"] = {
    desc = "Someone said something, and everyone who heard it has seen it: the place to answer.",
    fields = {
      actor = "who spoke",
      message = "what they said",
      target = { desc = "who they spoke to, when they named someone", optional = true },
    },
  },

  ["dragon:before_emote"] = {
    desc = "Someone is about to act something out. Change event.action, or cancel with a reason they'll see.",
    fields = {
      actor = "who's acting",
      action = "what they'll do, after their name, like \"waves.\"",
    },
  },

  ["dragon:emoted"] = {
    desc = "Someone acted something out, and everyone who saw it has seen it: the place to react.",
    fields = {
      actor = "who acted",
      action = "what they did, after their name",
    },
  },
}
