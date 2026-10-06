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
	"bbuck.dev/dragon-mud/plugin"
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

	// Plugins, if not nil, returns what the game's plugins serve to the
	// client: their web/ files, imported through the page's import map.
	// It's called for each page and file, so reloaded plugins show up.
	Plugins func() []plugin.Web
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
	mux.HandleFunc("GET /plugins/{plugin}/{hash}/{file...}", func(w http.ResponseWriter, r *http.Request) {
		servePluginFile(w, r, plugins(opts))
	})
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		head := pageHead(plugins(opts))
		w.Header().Set("Content-Security-Policy", csp(head.ImportMapHash))
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := page.Execute(w, map[string]any{"Name": opts.GameName, "Head": head}); err != nil {
			log.Error("rendering page", "error", err)
		}
	})

	return securityHeaders(mux), nil
}

func plugins(opts Options) []plugin.Web {
	if opts.Plugins == nil {
		return nil
	}

	return opts.Plugins()
}

// csp is the Content-Security-Policy for pages: scripts only from the
// server, plus the page's own import map, allowed by its hash. Inline
// styles are allowed for xterm 256 colors in feed messages.
func csp(importMapHash string) string {
	scripts := "'self'"
	if importMapHash != "" {
		scripts += " 'sha256-" + importMapHash + "'"
	}

	return "default-src 'self'; script-src " + scripts + "; style-src 'self' 'unsafe-inline'; connect-src 'self'"
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if w.Header().Get("Content-Security-Policy") == "" {
			w.Header().Set("Content-Security-Policy", csp(""))
		}
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
// request with the same ID, with HTML or Data. "event" frames are events
// for client code, named Name, with Data.
type serverFrame struct {
	T    string `json:"t"`
	HTML string `json:"html,omitempty"`
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
	Data any    `json:"data,omitempty"`

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
	limit := newBucket(requestRate, requestBurst)
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
			if !limit.allow(time.Now()) {
				log.Debug("dropped a client request over the rate limit", "request", frame.Name)
				continue
			}
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

	return wsjson.Write(ctx, c.ws, frameFor(m))
}

// frameFor is the frame that sends m.
func frameFor(m message.Message) serverFrame {
	switch {
	case m.Reply != "":
		return serverFrame{T: "reply", ID: m.Reply, HTML: m.HTML, Data: m.Data}
	case m.Event != "":
		return serverFrame{T: "event", Name: m.Event, Data: m.Data}
	default:
		return serverFrame{T: "html", HTML: render(m), Secret: m.Secret}
	}
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
		`<div hx-swap-oob="beforeend:#feed"><div class="msg msg-%s" data-kind="%s">%s</div></div>`,
		template.HTMLEscapeString(strings.ReplaceAll(m.Kind, "/", "-")), template.HTMLEscapeString(m.Kind), body,
	)
}
