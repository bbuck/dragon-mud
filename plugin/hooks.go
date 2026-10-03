package plugin

import (
	"context"
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
	sectionHookRx = regexp.MustCompile(`^section:[a-z][a-z0-9_]*\.[a-z][a-z0-9_]*$`)
)

// hookKeys are the fields a handler entry in hooks.lua may have.
var hookKeys = []string{"handler", "before", "after"}

// wiringKeys are the fields wiring.lua may have, and wiredHookKeys the
// fields of each hook under hooks.
var (
	wiringKeys    = []string{"hooks"}
	wiredHookKeys = []string{"order", "disable"}
)

// Hooks loads the handlers the plugin's hooks.lua returns, sorted by hook
// name. Each is a function, or a table with the function and ordering:
//
//	return {
//	  player_entered = function(event) ... end,
//	  before_say = {
//	    after = { "dragon:chat" },
//	    handler = function(event) ... end,
//	  },
//	}
func (p *Plugin) Hooks(ctx context.Context, engine scripting.Engine) ([]hook.Handler, error) {
	file := p.ID + "/hooks.lua"
	table, err := p.evalTable(ctx, engine, "hooks.lua", file)
	if err != nil || table == nil {
		return nil, err
	}

	var handlers []hook.Handler
	for _, name := range slices.Sorted(maps.Keys(table)) {
		where := fmt.Sprintf("%s: %s", file, name)
		if strings.HasPrefix(name, "section:") && !sectionHookRx.MatchString(name) || !strings.HasPrefix(name, "section:") && !hookNameRx.MatchString(name) {
			return nil, fmt.Errorf("%s isn't a valid hook name. Hook names are lowercase words joined by underscores, like player_entered, or section:<kind>.<section> to add to a message's section, like section:room.exits.", where)
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

// Wiring loads the game's wiring.lua: how it rearranges other plugins'
// hook handlers. A missing wiring.lua returns nil.
//
//	return {
//	  hooks = {
//	    before_say = { order = { "game", "dragon:chat" } },
//	    player_entered = { disable = { "dragon:presence" } },
//	  },
//	}
func (p *Plugin) Wiring(ctx context.Context, engine scripting.Engine) (map[string]hook.Wiring, error) {
	file := p.WiringFile()
	table, err := p.evalTable(ctx, engine, "wiring.lua", file)
	if err != nil || table == nil {
		return nil, err
	}
	if err := checkKeys(file, table, wiringKeys, nil); err != nil {
		return nil, err
	}

	raw, ok := table["hooks"]
	if !ok {
		return nil, nil
	}
	hooks, ok := raw.(map[string]any)
	if list, isList := raw.([]any); isList && len(list) == 0 {
		hooks, ok = map[string]any{}, true
	}
	if !ok {
		return nil, fmt.Errorf("%s: hooks must be a table keyed by hook name, like hooks = { before_say = { order = { ... } } }, not a %s.",
			file, scripting.TypeName(raw))
	}

	wiring := make(map[string]hook.Wiring, len(hooks))
	for _, name := range slices.Sorted(maps.Keys(hooks)) {
		where := fmt.Sprintf("%s: hooks.%s", file, name)
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

// WiringFile is where the plugin's wiring is, for messages.
func (p *Plugin) WiringFile() string {
	return p.ID + "/wiring.lua"
}

// HasWiring reports whether the plugin has a wiring.lua.
func (p *Plugin) HasWiring() bool {
	return p.exists("wiring.lua")
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
