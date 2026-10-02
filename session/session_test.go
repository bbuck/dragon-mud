package session

import (
	"sync"
	"testing"
	"time"

	"bbuck.dev/dragon-mud/message"
)

type fakeConn struct {
	mu      sync.Mutex
	written []string
	closed  chan struct{}
	block   chan struct{}
}

func newFakeConn() *fakeConn {
	return &fakeConn{closed: make(chan struct{})}
}

func (c *fakeConn) Write(m message.Message) error {
	if c.block != nil {
		<-c.block
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.written = append(c.written, m.Text)

	return nil
}

func (c *fakeConn) Close() error {
	close(c.closed)
	return nil
}

func (c *fakeConn) lines() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.written...)
}

func waitClosed(t *testing.T, c *fakeConn) {
	t.Helper()

	select {
	case <-c.closed:
	case <-time.After(time.Second):
		t.Fatal("connection was not closed")
	}
}

func TestCloseFlushesQueuedMessages(t *testing.T) {
	conn := newFakeConn()
	s := New(conn)

	s.Send(message.Text("one"))
	s.Send(message.Text("two"))
	s.Close()

	waitClosed(t, conn)

	got := conn.lines()
	if len(got) != 2 || got[0] != "one" || got[1] != "two" {
		t.Errorf("written = %v, want [one two]", got)
	}
}

func TestSlowClientIsDropped(t *testing.T) {
	conn := newFakeConn()
	conn.block = make(chan struct{})
	s := New(conn)

	for range queueSize + 2 {
		s.Send(message.Text("spam"))
	}

	select {
	case <-s.Done():
	default:
		t.Fatal("session should close when its queue is full")
	}

	close(conn.block)
	waitClosed(t, conn)
}

func TestIDsAreUnique(t *testing.T) {
	a := New(newFakeConn())
	b := New(newFakeConn())
	defer a.Close()
	defer b.Close()

	if a.ID() == b.ID() {
		t.Error("sessions share an id")
	}
}
