// Package telnet serves classic MUD clients over telnet and renders messages
// as ANSI text. See docs/design.md §4 and §6.
package telnet

import (
	"bufio"
	"context"
	"errors"
	"log/slog"
	"net"
	"strings"
	"time"

	"bbuck.dev/dragon-mud/ansi"
	"bbuck.dev/dragon-mud/message"
	"bbuck.dev/dragon-mud/session"
)

const (
	writeTimeout  = 10 * time.Second
	maxLineLength = 4096
)

// Serve accepts telnet connections on address until ctx is cancelled.
func Serve(ctx context.Context, address string, g session.Handler, log *slog.Logger) error {
	var lc net.ListenConfig
	listener, err := lc.Listen(ctx, "tcp", address)
	if err != nil {
		return err
	}

	log.Info("telnet listening", "address", listener.Addr().String())

	go func() {
		<-ctx.Done()
		listener.Close()
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			if errors.Is(err, net.ErrClosed) {
				return err
			}
			log.Warn("telnet accept failed", "error", err)
			continue
		}

		go handle(conn, g, log)
	}
}

func handle(netConn net.Conn, g session.Handler, log *slog.Logger) {
	log.Debug("telnet connection", "remote", netConn.RemoteAddr().String())

	s := session.New(&conn{conn: netConn})
	g.Connect(s)
	defer g.Disconnect(s)
	defer s.Close()

	scanner := bufio.NewScanner(&iacFilter{r: netConn})
	scanner.Buffer(make([]byte, 0, 1024), maxLineLength)

	for scanner.Scan() {
		g.Input(s, strings.TrimRight(scanner.Text(), "\r"))
	}
}

// conn renders messages as ANSI text for one telnet client.
type conn struct {
	conn net.Conn
}

func (c *conn) Write(m message.Message) error {
	text := ansi.Colorize(m.Text + "[x]")
	text = strings.ReplaceAll(text, "\n", "\r\n") + "\r\n"

	if err := c.conn.SetWriteDeadline(time.Now().Add(writeTimeout)); err != nil {
		return err
	}

	_, err := c.conn.Write([]byte(text))

	return err
}

func (c *conn) Close() error {
	return c.conn.Close()
}
