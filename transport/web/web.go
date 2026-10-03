// Package web serves the browser game client: an htmx page and a WebSocket
// that pushes rendered HTML fragments. See docs/design.md §5 and §6.
package web

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"bbuck.dev/dragon-mud/ansi"
	"bbuck.dev/dragon-mud/message"
	"bbuck.dev/dragon-mud/session"
)

const writeTimeout = 10 * time.Second

//go:embed static
var static embed.FS

var page = template.Must(template.ParseFS(static, "static/index.html"))

// Options configures the web server.
type Options struct {
	Address  string
	GameName string
}

// Serve runs the web server until ctx is cancelled.
func Serve(ctx context.Context, opts Options, g session.Handler, log *slog.Logger) error {
	handler, err := NewHandler(opts, g, log)
	if err != nil {
		return err
	}

	server := &http.Server{
		Addr:              opts.Address,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		BaseContext:       func(net.Listener) context.Context { return ctx },
	}

	listener, err := net.Listen("tcp", opts.Address)
	if err != nil {
		return err
	}

	log.Info("web listening", "url", "http://"+displayAddress(listener.Addr()))

	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(shutdown)
	}()

	if err := server.Serve(listener); !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}

// NewHandler returns the web client's HTTP handler: the page, its assets and
// the WebSocket.
func NewHandler(opts Options, g session.Handler, log *slog.Logger) (http.Handler, error) {
	assets, err := fs.Sub(static, "static")
	if err != nil {
		return nil, err
	}

	mux := http.NewServeMux()
	mux.Handle("GET /assets/", http.StripPrefix("/assets/", http.FileServerFS(assets)))
	// Some browsers ask for /favicon.ico regardless of the page's <link>.
	mux.HandleFunc("GET /favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFileFS(w, r, assets, "favicon.svg")
	})
	mux.HandleFunc("GET /ws", func(w http.ResponseWriter, r *http.Request) {
		serveSocket(w, r, g, log)
	})
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := page.Execute(w, map[string]string{"Name": opts.GameName}); err != nil {
			log.Error("rendering page", "error", err)
		}
	})

	return securityHeaders(mux), nil
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Inline styles are allowed for xterm 256 colors in feed messages.
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; connect-src 'self'")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		next.ServeHTTP(w, r)
	})
}

// displayAddress turns a listener address into something clickable.
func displayAddress(addr net.Addr) string {
	host, port, err := net.SplitHostPort(addr.String())
	if err != nil {
		return addr.String()
	}
	if host == "" || host == "::" || host == "0.0.0.0" {
		host = "localhost"
	}

	return net.JoinHostPort(host, port)
}

// clientFrame is a frame the browser sends: a command ("cmd", with Line)
// or a request ("req", with ID, Name and Data).
type clientFrame struct {
	T    string `json:"t"`
	Line string `json:"line"`

	ID   string         `json:"id"`
	Name string         `json:"name"`
	Data map[string]any `json:"data"`
}

// serverFrame is a frame sent to the browser. "html" frames are handed to
// htmx, which applies their out-of-band swaps. "reply" frames answer the
// request with the same ID.
type serverFrame struct {
	T    string `json:"t"`
	HTML string `json:"html"`
	ID   string `json:"id,omitempty"`

	// Secret asks the client to hide the player's next line.
	Secret bool `json:"secret,omitempty"`
}

func serveSocket(w http.ResponseWriter, r *http.Request, g session.Handler, log *slog.Logger) {
	ws, err := websocket.Accept(w, r, nil)
	if err != nil {
		log.Debug("websocket accept failed", "error", err)
		return
	}

	ctx := r.Context()
	s := session.New(&conn{ws: ws})
	g.Connect(s)
	defer g.Disconnect(s)
	defer s.Close()

	for {
		var frame clientFrame
		if err := wsjson.Read(ctx, ws, &frame); err != nil {
			return
		}

		switch frame.T {
		case "cmd":
			g.Input(s, frame.Line)
		case "req":
			g.Request(s, session.Request{ID: frame.ID, Name: frame.Name, Data: frame.Data})
		}
	}
}

// conn renders messages as HTML fragments for one browser.
type conn struct {
	ws *websocket.Conn
}

// Write sends one message. It doesn't use the request's context so that
// farewell messages still go out while the server is shutting down.
func (c *conn) Write(m message.Message) error {
	ctx, cancel := context.WithTimeout(context.Background(), writeTimeout)
	defer cancel()

	frame := serverFrame{T: "html", HTML: render(m), Secret: m.Secret}
	if m.Reply != "" {
		frame = serverFrame{T: "reply", ID: m.Reply, HTML: m.HTML}
	}

	return wsjson.Write(ctx, c.ws, frame)
}

func (c *conn) Close() error {
	return c.ws.Close(websocket.StatusNormalClosure, "")
}

// render turns a message into a fragment appended to the feed: its HTML
// form, or its text with color as HTML.
func render(m message.Message) string {
	body := m.HTML
	if body == "" {
		body = strings.ReplaceAll(ansi.HTML(m.Text), "\n", "<br>")
	}

	return fmt.Sprintf(
		`<div hx-swap-oob="beforeend:#feed"><div class="msg msg-%s">%s</div></div>`,
		template.HTMLEscapeString(m.Kind), body,
	)
}
