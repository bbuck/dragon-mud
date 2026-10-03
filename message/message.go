// Package message defines the structured messages the game sends to
// sessions, and the templates that render message kinds for each format.
// See docs/design.md §4.
package message

// Kinds the core sends.
const (
	// KindText is a line of text for the feed.
	KindText = "text"

	// KindSystem is a notice from the engine itself, such as the login
	// prompt or an unknown command.
	KindSystem = "system"

	// KindEcho is a command the player ran without typing it, such as by
	// clicking something.
	KindEcho = "echo"
)

// Message is one unit of output for a session.
type Message struct {
	// Kind says what the message is, such as "text" or "say".
	Kind string

	// Text is the feed text, which may contain color codes such as [r]...[x].
	Text string

	// HTML is the message rendered for the web, or empty to show Text.
	HTML string

	// Secret asks the transport not to echo the player's next line, such as
	// a password.
	Secret bool

	// Reply is the id of the client request this answers. Transports that
	// don't make requests drop replies.
	Reply string
}

// Text returns a text message.
func Text(text string) Message {
	return Message{Kind: KindText, Text: text}
}

// System returns a system message.
func System(text string) Message {
	return Message{Kind: KindSystem, Text: text}
}

// Secret returns a system message asking for secret input, such as a
// password.
func Secret(text string) Message {
	return Message{Kind: KindSystem, Text: text, Secret: true}
}

// Echo returns an echo of a command the player ran.
func Echo(command string) Message {
	return Message{Kind: KindEcho, Text: command}
}
