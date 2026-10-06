package web

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"bbuck.dev/dragon-mud/message"
	"bbuck.dev/dragon-mud/plugin"
)

func pluginServer(t *testing.T, plugins []plugin.Web) *httptest.Server {
	t.Helper()

	g := &echoGame{disconnected: make(chan struct{})}
	handler, err := NewHandler(Options{GameName: "Test", Plugins: func() []plugin.Web { return plugins }}, g, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	return server
}

var mapping = plugin.Web{
	Plugin:    "mapping",
	Namespace: "mapping",
	APIs:      []string{"johns:maps"},
	Hash:      "abc123",
	Styles:    []string{"map.css"},
	Main:      true,
	Files: fstest.MapFS{
		"main.mjs": {Data: []byte(`import { client } from "dragon";`)},
		"map.css":  {Data: []byte(`mapping-map { display: block; }`)},
	},
}

func get(t *testing.T, url string) (*http.Response, string) {
	t.Helper()

	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	return resp, string(body)
}

// The page imports each plugin's files by its name and its APIs, links
// its styles and loads its main module, and the CSP allows exactly the
// page's import map.
func TestPageImportsPlugins(t *testing.T) {
	server := pluginServer(t, []plugin.Web{mapping})
	resp, body := get(t, server.URL)

	m := regexp.MustCompile(`<script type="importmap">(.*?)</script>`).FindStringSubmatch(body)
	if m == nil {
		t.Fatalf("no import map in\n%s", body)
	}
	var importMap struct{ Imports map[string]string }
	if err := json.Unmarshal([]byte(m[1]), &importMap); err != nil {
		t.Fatalf("import map %s: %v", m[1], err)
	}
	want := map[string]string{
		"dragon":      "/assets/client.mjs",
		"mapping/":    "/plugins/mapping/abc123/",
		"johns:maps/": "/plugins/mapping/abc123/",
	}
	for k, v := range want {
		if importMap.Imports[k] != v {
			t.Errorf("imports[%q] = %q, want %q", k, importMap.Imports[k], v)
		}
	}

	sum := sha256.Sum256([]byte(m[1]))
	if hash := "'sha256-" + base64.StdEncoding.EncodeToString(sum[:]) + "'"; !strings.Contains(resp.Header.Get("Content-Security-Policy"), hash) {
		t.Errorf("CSP %q doesn't allow the import map (%s)", resp.Header.Get("Content-Security-Policy"), hash)
	}
	for _, want := range []string{
		`<link rel="stylesheet" href="/plugins/mapping/abc123/map.css">`,
		`<script type="module" src="/plugins/mapping/abc123/main.mjs"></script>`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("page doesn't contain %s:\n%s", want, body)
		}
	}
}

func TestPluginFiles(t *testing.T) {
	server := pluginServer(t, []plugin.Web{mapping})

	resp, body := get(t, server.URL+"/plugins/mapping/abc123/main.mjs")
	if resp.StatusCode != http.StatusOK || !strings.Contains(body, `from "dragon"`) {
		t.Fatalf("status %d, body %q", resp.StatusCode, body)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "javascript") {
		t.Errorf("content type %q", ct)
	}
	if cc := resp.Header.Get("Cache-Control"); !strings.Contains(cc, "immutable") {
		t.Errorf("cache control %q", cc)
	}

	resp, _ = get(t, server.URL+"/plugins/mapping/old999/main.mjs")
	if resp.StatusCode != http.StatusOK || resp.Header.Get("Cache-Control") != "no-cache" {
		t.Errorf("old hash: status %d, cache control %q", resp.StatusCode, resp.Header.Get("Cache-Control"))
	}

	for _, path := range []string{"/plugins/mapping/abc123/missing.mjs", "/plugins/nobody/abc123/main.mjs", "/plugins/mapping/abc123/../plugin.toml"} {
		if resp, _ := get(t, server.URL+path); resp.StatusCode != http.StatusNotFound {
			t.Errorf("%s: status %d, want 404", path, resp.StatusCode)
		}
	}
}

func TestEventFrames(t *testing.T) {
	frame := frameFor(message.ClientEvent("mapping:path_found", map[string]any{"steps": 3}))
	if frame.T != "event" || frame.Name != "mapping:path_found" || frame.Data.(map[string]any)["steps"] != 3 {
		t.Errorf("frame = %+v", frame)
	}

	frame = frameFor(message.Message{Reply: "4", Data: []any{"a"}})
	if frame.T != "reply" || frame.ID != "4" || frame.Data.([]any)[0] != "a" {
		t.Errorf("frame = %+v", frame)
	}
}

func TestRateLimit(t *testing.T) {
	b := newBucket(2, 3)
	now := time.Unix(0, 0)
	allowed := 0
	for range 10 {
		if b.allow(now) {
			allowed++
		}
	}
	if allowed != 3 {
		t.Errorf("allowed %d at once, want the burst of 3", allowed)
	}
	if !b.allow(now.Add(time.Second)) || !b.allow(now.Add(time.Second)) || b.allow(now.Add(time.Second)) {
		t.Error("after a second, want 2 more allowed")
	}
}
