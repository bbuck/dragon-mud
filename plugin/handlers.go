package plugin

import (
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"

	"bbuck.dev/dragon-mud/hook"
	"bbuck.dev/dragon-mud/scripting"
)

var (
	hookNameRx    = regexp.MustCompile(`^[a-z][a-z0-9_]*(:[a-z][a-z0-9_]*)?$`)
	sectionHookRx = regexp.MustCompile(`^section:[a-z][a-z0-9_]*(/[a-z][a-z0-9_]*)*\.[a-z][a-z0-9_]*$`)
)

// hookKeys are the fields a handler entry may have.
var hookKeys = []string{"handler", "before", "after"}

// wiredHookKeys are the fields wiring may set for each event.
var wiredHookKeys = []string{"order", "disable"}

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
func (p *Plugin) Handlers() ([]hook.Handler, error) {
	table, err := p.export("events.handlers", `handlers = { ["dragon:said"] = function(event) ... end }`)
	if err != nil || table == nil {
		return nil, err
	}

	var handlers []hook.Handler
	for _, name := range slices.Sorted(maps.Keys(table)) {
		where := field("events.handlers", name)
		if strings.HasPrefix(name, "section:") && !sectionHookRx.MatchString(name) || !strings.HasPrefix(name, "section:") && !hookNameRx.MatchString(name) {
			return nil, fmt.Errorf("%s isn't a valid hook name. Hook names are lowercase words joined by underscores, with an optional namespace, like can_move or mapping:map_changed, or section:<view>.<section> to add to a view's section, like section:room.exits or section:chat/say.badges.", where)
		}

		h := hook.Handler{Hook: name, Plugin: p.ID}
		switch v := table[name].(type) {
		case scripting.Function:
			h.Fn = v
		case map[string]any:
			if err := checkKeys(where, v, hookKeys, nil); err != nil {
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
//	}
func (p *Plugin) Wiring() (map[string]hook.Wiring, error) {
	hooks, err := p.export("events.wiring", `wiring = { ["dragon:player_connected"] = { disable = { "dragon:presence" } } }`)
	if err != nil || hooks == nil {
		return nil, err
	}

	wiring := make(map[string]hook.Wiring, len(hooks))
	for _, name := range slices.Sorted(maps.Keys(hooks)) {
		where := field("events.wiring", name)
		entry, ok := hooks[name].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%s must be a table like { order = { ... } } or { disable = { ... } }, not a %s.",
				where, scripting.TypeName(hooks[name]))
		}
		if err := checkKeys(where, entry, wiredHookKeys, nil); err != nil {
			return nil, err
		}

		var w hook.Wiring
		if w.Order, err = pluginList(where, entry, "order"); err != nil {
			return nil, err
		}
		if _, ok := entry["order"]; ok && w.Order == nil {
			w.Order = []string{}
		}
		if w.Disable, err = pluginList(where, entry, "disable"); err != nil {
			return nil, err
		}
		wiring[name] = w
	}

	return wiring, nil
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
