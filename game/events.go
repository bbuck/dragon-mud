package game

import "bbuck.dev/dragon-mud/hook"

// Notifications the engine sends about players.
const (
	notifyConnected    = "dragon:player_connected"
	notifyDisconnected = "dragon:player_disconnected"
)

// engineEvents declares the hooks and notifications the engine runs
// itself. Plugins declare theirs in events.declare.
var engineEvents = []hook.Decl{
	{
		Name: notifyBooted,
		Desc: "The server has started, before anyone can type: the place to make sure the world has what the game needs. Not sent on reload.",
	},
	{
		Name: notifyConnected,
		Desc: "A player has entered the game.",
		Fields: []hook.Field{
			{Name: "actor", Desc: "the character they play"},
			{Name: "reconnected", Desc: "true when they took over their character from another connection, so to everyone else they never left"},
		},
	},
	{
		Name: notifyDisconnected,
		Desc: "A player has left the game.",
		Fields: []hook.Field{
			{Name: "actor", Desc: "the character they played"},
		},
	},
	{
		Name: hookUnmatched,
		Desc: "Input no command matched, offered to plugins before the player is told why. A handler that deals with the line sets event.handled = true and returns the event.",
		Fields: []hook.Field{
			{Name: "actor", Desc: "who typed it"},
			{Name: "line", Desc: "what they typed"},
			{Name: "reason", Desc: "why the nearest command didn't match, when one nearly did", Optional: true},
			{Name: "handled", Desc: "set to true by a handler that dealt with the line", Optional: true},
		},
	},
	{
		Name: hookTooltip,
		Desc: "A player in the web client wants an entity's tooltip, rendered from templates/entity_tooltip. Cancel for no tooltip.",
		Fields: []hook.Field{
			{Name: "viewer", Desc: "the player looking"},
			{Name: "entity", Desc: "the thing they're looking at"},
			{Name: "block", Desc: "set to render one block of the tooltip template", Optional: true},
		},
		Extra: "data for the tooltip template",
	},
	{
		Name: hookAction,
		Desc: "A player in the web client clicked an entity. Set event.command to what that runs, as if they typed it.",
		Fields: []hook.Field{
			{Name: "viewer", Desc: "the player clicking"},
			{Name: "entity", Desc: "the thing they clicked"},
			{Name: "command", Desc: `set to the command to run, like "look #" .. event.entity.id`, Optional: true},
		},
	},
	{
		Name:   "section:",
		Prefix: true,
		Desc:   "Fills a section of a view: section:<view>.<section>. Handlers add to event.parts.",
		Fields: []hook.Field{
			{Name: "data", Desc: "the data the view was sent with"},
			{Name: "parts", Desc: `what handlers add: text, or a table like { view = "minimap", data = { ... } }`},
		},
	},
}
