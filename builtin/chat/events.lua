-- The hooks and notifications chat runs, and the fields of their events.
-- Run `dragon hooks <name>` to see one, with its handlers.
return {
  ["dragon:before_say"] = {
    desc = "Someone is about to say something. Change event.message, or cancel with a reason they'll see.",
    fields = {
      actor = "who's speaking",
      message = "what they'll say",
      target = { "who they're speaking to, when they name someone", optional = true },
    },
  },
}
