// Package session represents a connected player. Each transport provides a
// Conn that renders messages for its clients; a Session queues outgoing
// messages so the game loop never blocks on a slow connection.
// See docs/design.md §4 and §6.
package session

import (
	"sync"
	"sync/atomic"

	"bbuck.dev/dragon-mud/message"
)

// queueSize is how many messages may wait for a slow connection before the
// session is dropped.
const queueSize = 256

// ID identifies a session for its lifetime.
type ID uint64

var lastID atomic.Uint64

// Handler receives what happens on a transport's connections. The game
// implements it; transports call it from their own goroutines.
type Handler interface {
	Connect(s *Session)
	Input(s *Session, line string)
	Request(s *Session, r Request)
	Disconnect(s *Session)
}

// Request is something a client asks the game for other than running a
// command, such as an entity's tooltip. The game answers with a message
// whose Reply is the request's ID.
type Request struct {
	ID   string
	Name string
	Data map[string]any
}

// Conn is a transport's connection to one client.
type Conn interface {
	// Write renders and sends one message.
	Write(m message.Message) error

	// Close closes the connection. The transport's read loop should then
	// end and report the disconnect.
	Close() error
}

// Session is a connected client. Send and Close are safe to call from any
// goroutine.
type Session struct {
	id    ID
	conn  Conn
	out   chan message.Message
	done  chan struct{}
	close sync.Once
}

// New returns a session for conn and starts delivering its messages.
func New(conn Conn) *Session {
	s := &Session{
		id:   ID(lastID.Add(1)),
		conn: conn,
		out:  make(chan message.Message, queueSize),
		done: make(chan struct{}),
	}

	go s.deliver()

	return s
}

// ID returns the session's id.
func (s *Session) ID() ID {
	return s.id
}

// Send queues m for delivery. If the client has fallen too far behind, the
// session is closed instead of blocking the caller.
func (s *Session) Send(m message.Message) {
	select {
	case <-s.done:
	case s.out <- m:
	default:
		s.Close()
	}
}

// Close stops the session. Messages already queued are still delivered
// before the connection closes.
func (s *Session) Close() {
	s.close.Do(func() {
		close(s.done)
	})
}

// Done is closed when the session is closed.
func (s *Session) Done() <-chan struct{} {
	return s.done
}

func (s *Session) deliver() {
	defer s.conn.Close()

	for {
		select {
		case m := <-s.out:
			if err := s.conn.Write(m); err != nil {
				s.Close()
				return
			}
		case <-s.done:
			for {
				select {
				case m := <-s.out:
					if err := s.conn.Write(m); err != nil {
						return
					}
				default:
					return
				}
			}
		}
	}
}
