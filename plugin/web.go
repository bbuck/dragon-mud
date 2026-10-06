package plugin

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"maps"
	"path"
	"regexp"
	"slices"
	"strings"

	"bbuck.dev/dragon-mud/scripting"
)

// WebDir holds what a plugin adds to the web client: ES modules, CSS and
// assets, served as they are.
const WebDir = "web"

// WebMain is the module in web/ the client loads for every player, where a
// plugin defines its custom elements and listens for its events.
const WebMain = "main.mjs"

// Web is what a plugin serves to the web client.
type Web struct {
	// Plugin is the plugin's id, and Namespace the name its files are
	// imported by, as mapping/ in import "mapping/map.mjs".
	Plugin    string
	Namespace string

	// APIs are the APIs the plugin provides, which import the same files:
	// a plugin providing dragon:rooms is also dragon:rooms/.
	APIs []string

	// Files is the plugin's web/ directory.
	Files fs.FS

	// Hash identifies the files' contents, so their URLs change when they
	// do and browsers can cache them forever.
	Hash string

	// Styles are the stylesheets at the top of web/, sorted, which every
	// page links.
	Styles []string

	// Main is true when web/ has main.mjs, which every page loads.
	Main bool
}

// Web returns what the plugin serves to the web client, or nil when it has
// no web/ directory. Serving it needs the web_client capability.
func (p *Plugin) Web() (*Web, error) {
	if !p.exists(WebDir) {
		return nil, nil
	}
	if err := p.need(WebDir+"/", CapWebClient, "serving JavaScript and CSS to the web client"); err != nil {
		return nil, err
	}

	files, err := fs.Sub(p.files, WebDir)
	if err != nil {
		return nil, err
	}

	w := &Web{Plugin: p.ID, Namespace: p.Namespace(), Files: files, APIs: slices.Sorted(maps.Keys(p.Manifest.Provides))}
	sum := sha256.New()
	err = fs.WalkDir(files, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := fs.ReadFile(files, name)
		if err != nil {
			return err
		}
		fmt.Fprintf(sum, "%s\x00%d\x00", name, len(data))
		sum.Write(data)

		if !strings.Contains(name, "/") {
			switch {
			case path.Ext(name) == ".css":
				w.Styles = append(w.Styles, name)
			case name == WebMain:
				w.Main = true
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	w.Hash = hex.EncodeToString(sum.Sum(nil))[:12]
	slices.Sort(w.Styles)

	return w, nil
}

// clientNameRx matches a client event's name as a plugin writes it,
// without the namespace the engine adds.
var clientNameRx = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// ClientDef is a handler for an event the web client sends, through
// client.push or client.request in the plugin's JavaScript.
type ClientDef struct {
	// Name is the event's full name, with the plugin's namespace, such as
	// mapping:pan.
	Name   string
	Plugin string

	// Path is where the plugin exports it, such as client.pan.
	Path string

	// Fn is called as fn(session, data). What it returns answers a
	// client.request.
	Fn scripting.Function
}

// Client loads the handlers the plugin exports as client, sorted by name:
// what its JavaScript sends with client.push and client.request. Handling
// them needs the client_events capability.
//
//	client = {
//	  pan = function(session, data) ... end,
//	  area = function(session, data) return { rooms = ... } end,
//	}
func (p *Plugin) Client() ([]ClientDef, error) {
	table, err := p.export("client", `client = { pan = function(session, data) ... end }`)
	if err != nil || table == nil {
		return nil, err
	}
	if err := p.need("client", CapClientEvents, "handling events from the web client"); err != nil {
		return nil, err
	}

	var defs []ClientDef
	for _, name := range slices.Sorted(maps.Keys(table)) {
		where := field("client", name)
		if strings.Contains(name, ":") {
			return nil, fmt.Errorf("%s: the engine puts %s: in front of the plugin's client events itself, so name it %q, and the client sends %q.",
				where, p.Namespace(), name[strings.LastIndex(name, ":")+1:], p.Namespace()+":"+name[strings.LastIndex(name, ":")+1:])
		}
		if !clientNameRx.MatchString(name) {
			return nil, fmt.Errorf("%s isn't a valid client event name. Names are lowercase letters, digits and underscores, starting with a letter, like pan or load_area.", where)
		}
		fn, ok := table[name].(scripting.Function)
		if !ok {
			return nil, fmt.Errorf("%s must be a function(session, data), not a %s.", where, scripting.TypeName(table[name]))
		}
		defs = append(defs, ClientDef{Name: p.Namespace() + ":" + name, Plugin: p.ID, Path: where, Fn: fn})
	}

	return defs, nil
}
