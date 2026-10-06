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

var fieldNameRx = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// eventKeys are the fields a declaration may have, and
// fieldKeys the named fields of a field written as a table.
var (
	eventKeys = []string{"desc", "fields", "extra"}
	fieldKeys = []string{"optional"}
)

// Declarations loads what the plugin exports as events.declare, sorted by
// event name: the events the plugin sends, and the fields
// their events have.
//
//	declare = {
//	  ["mapping:map_drawn"] = {
//	    desc = "A map was drawn for a player.",
//	    fields = {
//	      actor = "who it was drawn for",
//	      area = { "the area drawn, if not the whole map", optional = true },
//	    },
//	  },
//	}
func (p *Plugin) Declarations() ([]event.Decl, error) {
	table, err := p.export("events.declare", `declare = { ["mapping:map_drawn"] = { fields = { actor = "who it was drawn for" } } }`)
	if err != nil || table == nil {
		return nil, err
	}

	var decls []event.Decl
	for _, name := range slices.Sorted(maps.Keys(table)) {
		where := field("events.declare", name)
		switch {
		case strings.HasPrefix(name, "section:"):
			return nil, fmt.Errorf("%s: the engine declares section hooks, so plugins don't. Remove it, and add to the section with a handler.", where)
		case !eventNameRx.MatchString(name):
			return nil, fmt.Errorf("%s isn't a valid event name. Event names are lowercase words joined by underscores, with your plugin's name in front, like mapping:map_drawn.", where)
		case strings.HasPrefix(name, BuiltinPrefix) && !strings.HasPrefix(p.ID, BuiltinPrefix):
			return nil, fmt.Errorf("%s uses the %q namespace, which is reserved for the engine's built-in plugins. Use your plugin's name instead, like %q.",
				where, BuiltinPrefix, p.Manifest.Name+":"+strings.TrimPrefix(name, BuiltinPrefix))
		}

		entry, ok := table[name].(map[string]any)
		if list, isList := table[name].([]any); isList && len(list) == 0 {
			entry, ok = map[string]any{}, true
		}
		if !ok {
			return nil, fmt.Errorf("%s must be a table like { desc = \"...\", fields = { actor = \"who did it\" } }, not a %s.",
				where, scripting.TypeName(table[name]))
		}
		if err := checkKeys(where, entry, eventKeys, nil); err != nil {
			return nil, err
		}

		d := event.Decl{Name: name, Plugin: p.ID}
		if d.Desc, err = optional[string](where, entry, "desc", "a string"); err != nil {
			return nil, err
		}
		if d.Extra, err = optional[string](where, entry, "extra", "a string saying what other fields are for"); err != nil {
			return nil, err
		}
		if d.Fields, err = eventFields(where, entry["fields"]); err != nil {
			return nil, err
		}
		decls = append(decls, d)
	}

	return decls, nil
}

// eventFields reads a declaration's fields: each a description, or a table
// with the description first and optional = true.
func eventFields(where string, raw any) ([]event.Field, error) {
	if raw == nil {
		return nil, nil
	}
	if list, ok := raw.([]any); ok && len(list) == 0 {
		return nil, nil
	}
	table, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s: fields must be a table keyed by field name, like fields = { actor = \"who did it\" }, not a %s.",
			where, scripting.TypeName(raw))
	}

	var fields []event.Field
	for _, name := range slices.Sorted(maps.Keys(table)) {
		at := fmt.Sprintf("%s: field %s", where, name)
		if !fieldNameRx.MatchString(name) {
			return nil, fmt.Errorf("%s isn't a valid field name. Field names are lowercase letters, digits and underscores, starting with a letter, like actor or target_room.", at)
		}

		f := event.Field{Name: name}
		switch v := table[name].(type) {
		case string:
			f.Desc = v
		case map[string]any:
			desc, ok := v["1"].(string)
			if !ok {
				return nil, fmt.Errorf("%s must start with its description, like { \"where from\", optional = true }.", at)
			}
			rest := maps.Clone(v)
			delete(rest, "1")
			if err := checkKeys(at, rest, fieldKeys, nil); err != nil {
				return nil, err
			}
			optional, isBool := v["optional"].(bool)
			if _, set := v["optional"]; set && !isBool {
				return nil, fmt.Errorf("%s: optional must be true or false, not a %s.", at, scripting.TypeName(v["optional"]))
			}
			f.Desc, f.Optional = desc, optional
		default:
			return nil, fmt.Errorf("%s must be a description, like %s = \"who did it\", or a table like { \"who did it\", optional = true }, not a %s.",
				at, name, scripting.TypeName(table[name]))
		}
		if strings.TrimSpace(f.Desc) == "" {
			return nil, fmt.Errorf("%s needs a description saying what it holds, like %s = \"who did it\".", at, name)
		}
		fields = append(fields, f)
	}

	return fields, nil
}
