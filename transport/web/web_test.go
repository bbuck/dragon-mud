package web

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"bbuck.dev/dragon-mud/message"
	"bbuck.dev/dragon-mud/session"
)

// echoGame greets each session and echoes its input back.
type echoGame struct {
	disconnected chan struct{}
}

func (g *echoGame) Connect(s *session.Session) {
	s.Send(message.System("[Y]Welcome[x]"))
}

func (g *echoGame) Input(s *session.Session, line string) {
	s.Send(message.Text("you typed <" + line + ">"))
}

func (g *echoGame) Request(s *session.Session, r session.Request) {
	s.Send(message.Message{Reply: r.ID, HTML: "<p>" + r.Name + " " + r.Data["ref"].(string) + "</p>"})
}

func (g *echoGame) Disconnect(*session.Session) {
	close(g.disconnected)
}

func newServer(t *testing.T) (*httptest.Server, *echoGame) {
	t.Helper()

	g := &echoGame{disconnected: make(chan struct{})}
	handler, err := NewHandler(Options{GameName: "Test <Realm>"}, g, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	return server, g
}

func TestPage(t *testing.T) {
	server, _ := newServer(t)

	resp, err := http.Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if !strings.Contains(string(body), "<title>Test &lt;Realm&gt;</title>") {
		t.Errorf("page doesn't contain the escaped game name:\n%s", body)
	}
	if !strings.Contains(resp.Header.Get("Content-Security-Policy"), "script-src 'self'") {
		t.Errorf("missing CSP header: %v", resp.Header)
	}
}

func TestAssets(t *testing.T) {
	server, _ := newServer(t)

	for _, name := range []string{"client.mjs", "htmx.min.js"} {
		resp, err := http.Get(server.URL + "/assets/" + name)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK || !strings.Contains(resp.Header.Get("Content-Type"), "javascript") {
			t.Errorf("%s: status %d, content type %q", name, resp.StatusCode, resp.Header.Get("Content-Type"))
		}
	}
}

func TestSocket(t *testing.T) {
	server, g := newServer(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ws, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http")+"/ws", nil)
	if err != nil {
		t.Fatal(err)
	}

	var frame serverFrame
	if err := wsjson.Read(ctx, ws, &frame); err != nil {
		t.Fatal(err)
	}
	want := `<div hx-swap-oob="beforeend:#feed"><div class="msg msg-system"><span class="ansi-fg-3 ansi-bold">Welcome</span></div></div>`
	if frame.T != "html" || frame.HTML != want {
		t.Errorf("greeting frame = %+v\nwant html %s", frame, want)
	}

	if err := wsjson.Write(ctx, ws, clientFrame{T: "cmd", Line: "look"}); err != nil {
		t.Fatal(err)
	}
	if err := wsjson.Read(ctx, ws, &frame); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(frame.HTML, "you typed &lt;look&gt;") {
		t.Errorf("echo frame HTML = %s", frame.HTML)
	}

	ws.Close(websocket.StatusNormalClosure, "")
	select {
	case <-g.disconnected:
	case <-time.After(2 * time.Second):
		t.Error("game was not told about the disconnect")
	}
}

func TestRenderMultiline(t *testing.T) {
	got := render(message.Text("one\ntwo"))
	if !strings.Contains(got, "one<br>two") {
		t.Errorf("render = %s", got)
	}
}

func TestSocketRequests(t *testing.T) {
	server, _ := newServer(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ws, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http")+"/ws", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ws.Close(websocket.StatusNormalClosure, "")

	var frame serverFrame
	if err := wsjson.Read(ctx, ws, &frame); err != nil { // the greeting
		t.Fatal(err)
	}

	req := clientFrame{T: "req", ID: "7", Name: "entity_tooltip", Data: map[string]any{"ref": "abc"}}
	if err := wsjson.Write(ctx, ws, req); err != nil {
		t.Fatal(err)
	}
	if err := wsjson.Read(ctx, ws, &frame); err != nil {
		t.Fatal(err)
	}
	if frame.T != "reply" || frame.ID != "7" || frame.HTML != "<p>entity_tooltip abc</p>" {
		t.Errorf("reply frame = %+v", frame)
	}
}

func TestRenderHTML(t *testing.T) {
	got := render(message.Message{Kind: "say", Text: "[r]ignored[x]", HTML: `<q>hi</q>`})
	want := `<div hx-swap-oob="beforeend:#feed"><div class="msg msg-say"><q>hi</q></div></div>`
	if got != want {
		t.Errorf("render = %s\nwant %s", got, want)
	}
}
