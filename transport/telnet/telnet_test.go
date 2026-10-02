package telnet

import (
	"bytes"
	"io"
	"net"
	"testing"

	"bbuck.dev/dragon-mud/message"
)

func TestSecretInputHidesEcho(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	received := make(chan []byte, 1)
	go func() {
		b, _ := io.ReadAll(client)
		received <- b
	}()

	c := &conn{conn: server}
	c.Write(message.Secret("Password:"))
	c.Write(message.Secret("Again:")) // already hidden; no second WILL ECHO
	c.lineRead()
	c.lineRead() // already visible; no second WONT ECHO
	server.Close()

	got := <-received
	willEcho := []byte{iac, will, optEcho}
	wontEcho := []byte{iac, wont, optEcho}

	if n := bytes.Count(got, willEcho); n != 1 {
		t.Errorf("sent WILL ECHO %d times, want 1: %q", n, got)
	}
	if n := bytes.Count(got, wontEcho); n != 1 {
		t.Errorf("sent WONT ECHO %d times, want 1: %q", n, got)
	}
	if bytes.Index(got, willEcho) > bytes.Index(got, wontEcho) {
		t.Errorf("WONT ECHO came before WILL ECHO: %q", got)
	}
}
