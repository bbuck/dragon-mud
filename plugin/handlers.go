package plugin

import (
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"

	"bbuck.dev/dragon-mud/event"
	"bbuck.dev/dragon-mud/scripting"
)

var (
	eventNameRx    = regexp.MustCompile(`^[a-z][a-z0-9_]*(:[a-z][a-z0-9_]*)?$`)
	sectionEventRx = regexp.MustCompile(`^section:[a-z][a-z0-9_]*(/[a-z][a-z0-9_]*)*\.[a-z][a-z0-9_]*$`)
)

// handlerKeys are the fields a handler entry may have.
var handlerKeys = []string{"handler", "before", "after"}

// wiringKeys are the fields wiring may set for each event.
var wiringKeys = []string{"order", "disable", "redirect"}

// Handlers loads the handlers the plugin exports as events.handlers,
// sorted by event name. Each is a function, or a table with the function
// and ordering:
//
//	handlers = {
//	  ["dragon:player_connected"] = function(event) ... end,
//	  ["dragon:before_say"] = {
//	    after = { "dragon:chat" },
//	    handler = function(event) ... end,
//	  },
//	}
func (p *Plugin) Handlers() ([]event.Handler, error) {
	table, err := p.export("events.handlers", `handlers = { ["dragon:said"] = function(event) ... end }`)
	if err != nil || table == nil {
		return nil, err
	}

	var handlers []event.Handler
	for _, name := range slices.Sorted(maps.Keys(table)) {
		where := field("events.handlers", name)
		if strings.HasPrefix(name, "section:") && !sectionEventRx.MatchString(name) || !strings.HasPrefix(name, "section:") && !eventNameRx.MatchString(name) {
			return nil, fmt.Errorf("%s isn't a valid event name. Event names are lowercase words joined by underscores, with an optional namespace, like can_move or mapping:map_changed, or section:<view>.<section> to add to a view's section, like section:room.exits or section:chat/say.badges.", where)
		}

		h := event.Handler{Event: name, Plugin: p.ID}
		switch v := table[name].(type) {
		case scripting.Function:
			h.Fn = v
		case map[string]any:
			if err := checkKeys(where, v, handlerKeys, nil); err != nil {
				return nil, err
			}
			fn, ok := v["handler"].(scripting.Function)
			if !ok {
				return nil, fmt.Errorf("%s needs handler = function(event) ... end alongside its before and after.", where)
			}
			h.Fn = fn
			if h.Before, err = pluginList(where, v, "before"); err != nil {
				return nil, err
			}
			if h.After, err = pluginList(where, v, "after"); err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("%s must be a function(event), or a table like { after = { \"dragon:chat\" }, handler = function(event) ... end }, not a %s.",
				where, scripting.TypeName(table[name]))
		}

		handlers = append(handlers, h)
	}

	return handlers, nil
}

// Wiring loads how the game rearranges other plugins' handlers, which it
// exports as events.wiring, keyed by event name. Only the game's own plugin
// has wiring; Load checks that.
//
//	wiring = {
//	  ["dragon:before_say"] = { order = { "game", "dragon:chat" } },
//	  ["dragon:player_connected"] = { disable = { "dragon:presence" } },
//	  ["dragon:player_disconnected"] = { redirect = { ["dragon:presence"] = "mygame:went_home" } },
//	}
func (p *Plugin) Wiring() (map[string]event.Wiring, error) {
	table, err := p.export("events.wiring", `wiring = { ["dragon:player_connected"] = { disable = { "dragon:presence" } } }`)
	if err != nil || table == nil {
		return nil, err
	}

	wiring := make(map[string]event.Wiring, len(table))
	for _, name := range slices.Sorted(maps.Keys(table)) {
		where := field("events.wiring", name)
		entry, ok := table[name].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%s must be a table like { order = { ... } }, { disable = { ... } } or { redirect = { ... } }, not a %s.",
				where, scripting.TypeName(table[name]))
		}
		if err := checkKeys(where, entry, wiringKeys, nil); err != nil {
			return nil, err
		}

		var w event.Wiring
		if w.Order, err = pluginList(where, entry, "order"); err != nil {
			return nil, err
		}
		if _, ok := entry["order"]; ok && w.Order == nil {
			w.Order = []string{}
		}
		if w.Disable, err = pluginList(where, entry, "disable"); err != nil {
			return nil, err
		}
		if w.Redirect, err = redirects(where, entry["redirect"]); err != nil {
			return nil, err
		}
		wiring[name] = w
	}

	return wiring, nil
}

// redirects reads wiring's optional redirect table: plugin ids keyed to the
// event each one's handler runs on instead.
func redirects(where string, raw any) (map[string]string, error) {
	if raw == nil {
		return nil, nil
	}
	if list, ok := raw.([]any); ok && len(list) == 0 {
		return nil, nil
	}
	table, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s: redirect must be a table of plugins and the events their handlers run on instead, like redirect = { [\"dragon:presence\"] = \"mygame:arrived\" }, not a %s.",
			where, scripting.TypeName(raw))
	}

	redirect := make(map[string]string, len(table))
	for _, id := range slices.Sorted(maps.Keys(table)) {
		to, ok := table[id].(string)
		if !ok {
			return nil, fmt.Errorf("%s: redirect[%q] must be the name of the event its handler runs on instead, like \"mygame:arrived\", not a %s.", where, id, scripting.TypeName(table[id]))
		}
		if !eventNameRx.MatchString(to) {
			return nil, fmt.Errorf("%s: redirect[%q] is %q, which isn't a valid event name. Event names are lowercase words joined by underscores, with an optional namespace, like mygame:arrived.", where, id, to)
		}
		redirect[id] = to
	}

	return redirect, nil
}

// pluginList reads an optional list of plugin ids.
func pluginList(where string, entry map[string]any, key string) ([]string, error) {
	raw, ok := entry[key]
	if !ok || raw == nil {
		return nil, nil
	}

	list, ok := raw.([]any)
	if !ok {
		if s, isString := raw.(string); isString {
			return nil, fmt.Errorf("%s: %s must be a list of plugins. Write %s = { %q }.", where, key, key, s)
		}
		if m, isMap := raw.(map[string]any); isMap && len(m) == 0 {
			return []string{}, nil
		}
		return nil, fmt.Errorf("%s: %s must be a list of plugins like { \"dragon:chat\" }, not a %s.", where, key, scripting.TypeName(raw))
	}

	ids := make([]string, len(list))
	for i, v := range list {
		s, ok := v.(string)
		if !ok || s == "" {
			return nil, fmt.Errorf("%s: %s #%d must be a plugin name like \"dragon:chat\", not %v.", where, key, i+1, v)
		}
		ids[i] = s
	}

	return ids, nil
}
