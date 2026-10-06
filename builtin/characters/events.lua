-- The events character select and creation send, and the fields of
-- each. Run `dragon events <name>` to see one, with its
-- handlers.
return {
  ["dragon:character_steps"] = {
    desc = "A new character is being created. Add the modes that ask for its details to event.steps, in order; each ends with the changes it made to the draft.",
    fields = {
      account = "the account creating it",
      steps = "the modes to run, in order; handlers add to it",
    },
  },

  ["dragon:character_created"] = {
    desc = "A new character was created and added to its account.",
    fields = {
      character = "the new character",
      account = "the account it belongs to",
    },
  },
}
