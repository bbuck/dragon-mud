// Package message defines the structured messages the game sends to
// sessions. The game never writes finished text; renderers turn messages into
// output for each transport. See docs/design.md §4.
package message

// Kinds the core sends.
const (
	// KindText is a line of text for the feed.
	KindText = "text"

	// KindSystem is a notice from the engine itself, such as the login
	// prompt or an unknown command.
	KindSystem = "system"
)

// Message is one unit of output for a session.
type Message struct {
	// Kind says what the message is, such as "text" or "say".
	Kind string

	// Text is the feed text, which may contain color codes such as [r]...[x].
	Text string

	// Secret asks the transport not to echo the player's next line, such as
	// a password.
	Secret bool
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
