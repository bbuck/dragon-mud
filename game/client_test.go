package game

import (
	"fmt"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"bbuck.dev/dragon-mud/message"
	"bbuck.dev/dragon-mud/session"
)

// await waits for a message matching ok.
func (c *client) await(what string, ok func(message.Message) bool) message.Message {
	c.t.Helper()

	timeout := time.After(2 * time.Second)
	for {
		select {
		case m := <-c.conn.messages:
			if ok(m) {
				return m
			}
		case <-timeout:
			c.t.Fatalf("never received %s", what)
		}
	}
}

func TestClientEvents(t *testing.T) {
	g := startGame(t, fstest.MapFS{
		"init.lua": file(`return {
			client = {
				area = function(session, data)
					return { id = data.id, who = session.character:get("name"), here = session.character }
				end,
				pan = function(session, data)
					session:push("game:panned", { x = data.x * 2 })
				end,
				broken = function() error("oops") end,
			},
		}`),
	})
	alice := connect(t, g)
	alice.login("Alice")

	g.Request(alice.s, session.Request{ID: "1", Name: "game:area", Data: map[string]any{"id": "riverside"}})
	reply := alice.await("the reply", func(m message.Message) bool { return m.Reply == "1" })
	data, _ := reply.Data.(map[string]any)
	if data["id"] != "riverside" || data["who"] != "Alice" || len(data["here"].(string)) != 8 {
		t.Errorf("reply data = %#v", reply.Data)
	}

	g.Request(alice.s, session.Request{Name: "game:pan", Data: map[string]any{"x": 2}})
	pushed := alice.await("the pushed event", func(m message.Message) bool { return m.Event != "" })
	if x, _ := pushed.Data.(map[string]any)["x"]; pushed.Event != "game:panned" || fmt.Sprint(x) != "4" {
		t.Errorf("pushed %q %#v", pushed.Event, pushed.Data)
	}

	// Failures and unknown events still answer, with nothing.
	for i, name := range []string{"game:broken", "game:nothing"} {
		id := string(rune('2' + i))
		g.Request(alice.s, session.Request{ID: id, Name: name})
		reply := alice.await(name+"'s reply", func(m message.Message) bool { return m.Reply == id })
		if reply.Data != nil {
			t.Errorf("%s replied %#v", name, reply.Data)
		}
	}
}

func TestClientEventErrors(t *testing.T) {
	tests := []struct {
		name string
		init string
		want string
	}{
		{
			"namespaced",
			`return { client = { ["game:pan"] = function() end } }`,
			`client["game:pan"]: the engine puts game: in front of the plugin's client events itself, so name it "pan", and the client sends "game:pan".`,
		},
		{
			"not a function",
			`return { client = { pan = true } }`,
			`client.pan must be a function(session, data), not a boolean.`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := newGame(t, fstest.MapFS{"init.lua": file(tt.init)})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %v\nwant it to contain %q", err, tt.want)
			}
		})
	}
}

// A plugin's web files and client events need capabilities.
func TestClientCapabilities(t *testing.T) {
	_, err := newGameWithPlugins(t, nil, localPlugin("mapping", "", fstest.MapFS{"web/main.mjs": file(`export {}`)}))
	if err == nil || !strings.Contains(err.Error(), "web/: serving JavaScript and CSS to the web client needs the web_client capability") {
		t.Errorf("web files: %v", err)
	}

	_, err = newGameWithPlugins(t, nil, localPlugin("mapping", "", fstest.MapFS{"init.lua": file(`return { client = { pan = function() end } }`)}))
	if err == nil || !strings.Contains(err.Error(), "client: handling events from the web client needs the client_events capability") {
		t.Errorf("client events: %v", err)
	}

	g, err := newGameWithPlugins(t, nil, localPlugin("mapping", `capabilities = ["web_client"]
[provides]
"johns:maps" = "1.0"`, fstest.MapFS{
		"init.lua":     file(`return { api = "api" }`),
		"lua/api.lua":  file(`return {}`),
		"web/main.mjs": file(`export {}`),
		"web/map.css":  file(`x {}`),
	}))
	if err != nil {
		t.Fatal(err)
	}
	web := g.Web()
	if len(web) != 1 || web[0].Namespace != "mapping" || !web[0].Main || web[0].Styles[0] != "map.css" || web[0].APIs[0] != "johns:maps" || web[0].Hash == "" {
		t.Errorf("web = %+v", web)
	}
}

// {{asset}} writes the URL of a plugin's web file, by the name the import
// map gives the plugin's files.
func TestAssetHelper(t *testing.T) {
	mapping := localPlugin("mapping", `capabilities = ["web_client"]`, fstest.MapFS{
		"web/icons/door.png":   file("png"),
		"views/door.txt.tmpl":  file(`A door.`),
		"views/door.html.tmpl": file(`<img src="{{asset "mapping/icons/door.png"}}">`),
		"views/bad.txt.tmpl":   file(`{{asset "maping/icons/door.png"}}`),
	})
	g, err := newGameWithPlugins(t, fstest.MapFS{
		"lua/commands.lua": file(`return {
			door = { forms = { { "door", function(actor) actor:send("door", {}) end } } },
			bad = { forms = { { "bad", function(actor) actor:send("bad", {}) end } } },
		}`),
	}, mapping)
	if err != nil {
		t.Fatal(err)
	}
	runGame(t, g)
	alice := connect(t, g)
	alice.login("Alice")

	alice.send("door")
	m := alice.await("the door", func(m message.Message) bool { return m.Kind == "door" })
	want := `<img src="` + g.Web()[0].URL() + `icons/door.png">`
	if m.HTML != want {
		t.Errorf("HTML = %s, want %s", m.HTML, want)
	}

	alice.send("bad")
	alice.expect(`{{asset "maping/icons/door.png"}}: no plugin serves web files as maping/. Did you mean "mapping"?`)
}
