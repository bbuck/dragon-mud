-- Talking to other players.
return {
  commands = require("commands"),
  events = {
    declare = require("events"),
  },
  -- The chat API, which others import with require("@dragon:chat").
  api = "api",
}
