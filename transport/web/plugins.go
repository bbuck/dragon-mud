package web

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"html/template"
	"io/fs"
	"net/http"
	"net/url"
	"path"
	"sync"
	"time"

	"bbuck.dev/dragon-mud/plugin"
)

// clientModule is the core client's URL, which the import map calls
// "dragon": import { client } from "dragon".
const clientModule = "/assets/client.mjs"

// head is what the page's <head> gets from plugins: the import map, and
// each plugin's stylesheets and main module.
type head struct {
	// ImportMap is JSON, which json.Marshal has made safe to put in a
	// <script> as it is: it escapes <, > and &. html/template doesn't
	// treat an import map as JavaScript, so it's marked as HTML to keep
	// it from being escaped as text.
	ImportMap     template.HTML
	ImportMapHash string
	Styles        []string
	Modules       []string
}

// pluginURL is where a plugin's web files are served: under its id and the
// hash of their contents, so a URL never changes what it serves and
// browsers can cache it for good.
func pluginURL(w plugin.Web) string {
	return "/plugins/" + url.PathEscape(w.Plugin) + "/" + w.Hash + "/"
}

// pageHead builds the page's head for the plugins' web files. The import
// map names each plugin's files for its namespace and for each API it
// provides, so code imports an API without knowing which plugin
// provides it: import "dragon:rooms/map.mjs".
func pageHead(plugins []plugin.Web) head {
	imports := map[string]string{"dragon": clientModule}
	var h head
	for _, w := range plugins {
		base := pluginURL(w)
		imports[w.Namespace+"/"] = base
		for _, api := range w.APIs {
			imports[api+"/"] = base
		}
		for _, style := range w.Styles {
			h.Styles = append(h.Styles, base+style)
		}
		if w.Main {
			h.Modules = append(h.Modules, base+plugin.WebMain)
		}
	}

	data, err := json.Marshal(map[string]any{"imports": imports})
	if err != nil {
		// A map of strings always marshals.
		panic(err)
	}
	sum := sha256.Sum256(data)
	h.ImportMap = template.HTML(data)
	h.ImportMapHash = base64.StdEncoding.EncodeToString(sum[:])

	return h
}

// servePluginFile serves a file from a plugin's web/ directory. A URL with
// the current hash is cached for good; one with an old hash, from a page
// loaded before the plugin changed, gets the current file, uncached.
func servePluginFile(w http.ResponseWriter, r *http.Request, plugins []plugin.Web) {
	id, hash, file := r.PathValue("plugin"), r.PathValue("hash"), r.PathValue("file")
	for _, p := range plugins {
		if p.Plugin != id {
			continue
		}
		if !fs.ValidPath(file) {
			break
		}
		if hash == p.Hash {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}
		if path.Ext(file) == ".mjs" {
			w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		}
		http.ServeFileFS(w, r, p.Files, file)
		return
	}

	http.NotFound(w, r)
}

// Client requests and pushes are limited per connection, so a client
// can't flood the game loop with plugin handlers: requestRate a second on
// average, in bursts of up to requestBurst.
const (
	requestRate  = 20
	requestBurst = 40
)

// bucket is a token bucket rate limit.
type bucket struct {
	mu     sync.Mutex
	rate   float64
	burst  float64
	tokens float64
	last   time.Time
}

func newBucket(rate, burst float64) *bucket {
	return &bucket{rate: rate, burst: burst, tokens: burst}
}

// allow takes a token at now, reporting whether there was one.
func (b *bucket) allow(now time.Time) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.last.IsZero() {
		b.tokens = min(b.burst, b.tokens+now.Sub(b.last).Seconds()*b.rate)
	}
	b.last = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--

	return true
}
