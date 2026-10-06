package plugin

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"bbuck.dev/dragon-mud/command"
	"bbuck.dev/dragon-mud/schema"
	"bbuck.dev/dragon-mud/scripting"
	"bbuck.dev/dragon-mud/world"
)

// typeNameRx matches type names: a name, optionally after its plugin's
// namespace, like items:item. As with modes and events, the engine never
// adds a namespace; plugins namespace their own types.
var typeNameRx = modeNameRx

// schemaKeys are the fields schema may have, typeKeys those of a type,
// extendKeys those of an extension, and schemaFieldKeys those of a field
// written as a table.
var (
	schemaKeys      = []string{"types", "extend"}
	typeKeys        = []string{"desc", "fields"}
	extendKeys      = []string{"fields"}
	schemaFieldKeys = []string{"type", "default"}
)

// Schema loads the types the plugin exports as schema.types and the fields
// it adds to other plugins' types as schema.extend, sorted by type name.
//
//	schema = {
//	  types = {
//	    ["items:item"] = {
//	      desc = "Something that can be carried.",
//	      fields = {
//	        description = { "what players see when they look at it", type = "text" },
//	        weight = { "how heavy it is, in pounds", type = "number", default = 1 },
//	        notes = "anything builders want to remember",
//	      },
//	    },
//	  },
//	  extend = {
//	    ["rooms:room"] = { fields = { coords = { "where it is on the map", type = "table" } } },
//	  },
//	}
func (p *Plugin) Schema(convert func(any) (any, error)) ([]schema.Type, []schema.Extension, error) {
	table, err := p.export("schema", `schema = { types = { ["items:item"] = { fields = { weight = "how heavy it is" } } } }`)
	if err != nil || table == nil {
		return nil, nil, err
	}
	if err := checkKeys("schema", table, schemaKeys, nil); err != nil {
		return nil, nil, err
	}

	typesTable, err := asTable("schema.types", table["types"], `types = { ["items:item"] = { fields = { weight = "how heavy it is" } } }`)
	if err != nil {
		return nil, nil, err
	}
	var types []schema.Type
	for _, name := range slices.Sorted(maps.Keys(typesTable)) {
		where := field("schema.types", name)
		switch {
		case !typeNameRx.MatchString(name):
			return nil, nil, fmt.Errorf("%s isn't a valid type name. Type names are lowercase letters, digits and underscores, starting with a letter, after your plugin's name, like %s:item.", where, p.Namespace())
		case strings.HasPrefix(name, BuiltinPrefix) && !strings.HasPrefix(p.ID, BuiltinPrefix):
			return nil, nil, fmt.Errorf("%s uses the %q namespace, which is reserved for the engine's built-in plugins. Use your plugin's name instead, like %q.",
				where, BuiltinPrefix, p.Namespace()+":"+strings.TrimPrefix(name, BuiltinPrefix))
		}

		entry, err := asTable(where, typesTable[name], `{ desc = "...", fields = { weight = "how heavy it is" } }`)
		if err != nil {
			return nil, nil, err
		}
		if err := checkKeys(where, entry, typeKeys, nil); err != nil {
			return nil, nil, err
		}

		t := schema.Type{Name: name, Plugin: p.ID}
		if t.Desc, err = optional[string](where, entry, "desc", "a string"); err != nil {
			return nil, nil, err
		}
		if t.Fields, err = p.schemaFields(where, entry["fields"], "", convert); err != nil {
			return nil, nil, err
		}
		types = append(types, t)
	}

	extendTable, err := asTable("schema.extend", table["extend"], `extend = { ["rooms:room"] = { fields = { coords = "where it is on the map" } } }`)
	if err != nil {
		return nil, nil, err
	}
	var extensions []schema.Extension
	for _, name := range slices.Sorted(maps.Keys(extendTable)) {
		where := field("schema.extend", name)
		if !typeNameRx.MatchString(name) {
			return nil, nil, fmt.Errorf("%s isn't a valid type name. Name the type to add fields to, like rooms:room.", where)
		}
		entry, err := asTable(where, extendTable[name], `{ fields = { coords = "where it is on the map" } }`)
		if err != nil {
			return nil, nil, err
		}
		if err := checkKeys(where, entry, extendKeys, nil); err != nil {
			return nil, nil, err
		}

		ext := schema.Extension{Type: name, Plugin: p.ID, Where: p.ID + ": " + where}
		if ext.Fields, err = p.schemaFields(where, entry["fields"], p.Namespace()+".", convert); err != nil {
			return nil, nil, err
		}
		extensions = append(extensions, ext)
	}

	return types, extensions, nil
}

// schemaFields reads fields: each a description, or a table with the
// description first and type and default. prefix goes in front of each
// field's name, for fields added to another plugin's type. convert turns a
// default from a script value into a property value.
func (p *Plugin) schemaFields(where string, raw any, prefix string, convert func(any) (any, error)) ([]schema.Field, error) {
	table, err := asTable(where+".fields", raw, `fields = { weight = { "how heavy it is", type = "number" } }`)
	if err != nil || table == nil {
		return nil, err
	}

	kinds := make([]string, len(schema.Kinds))
	for i, k := range schema.Kinds {
		kinds[i] = string(k)
	}

	var fields []schema.Field
	for _, name := range slices.Sorted(maps.Keys(table)) {
		at := fmt.Sprintf("%s: field %s", where, name)
		if !fieldNameRx.MatchString(name) {
			return nil, fmt.Errorf("%s isn't a valid field name. Field names are lowercase letters, digits and underscores, starting with a letter, like weight or short_desc.", at)
		}

		f := schema.Field{Name: prefix + name, Kind: schema.Any, Plugin: p.ID}
		switch v := table[name].(type) {
		case string:
			f.Desc = v
		case map[string]any:
			desc, ok := v["1"].(string)
			if !ok {
				return nil, fmt.Errorf("%s must start with its description, like { \"how heavy it is\", type = \"number\" }.", at)
			}
			rest := maps.Clone(v)
			delete(rest, "1")
			if err := checkKeys(at, rest, schemaFieldKeys, nil); err != nil {
				return nil, err
			}
			f.Desc = desc

			if kind, set := v["type"]; set {
				k, ok := kind.(string)
				if !ok || !slices.Contains(kinds, k) {
					return nil, fmt.Errorf("%s: type must be one of %s, not %v.%s", at, andList(kinds), kind, command.DidYouMean(fmt.Sprint(kind), kinds))
				}
				f.Kind = schema.Kind(k)
			}
			if def, set := v["default"]; set {
				value, err := convert(def)
				if err == nil {
					value, err = world.Normalize(value)
				}
				if err != nil {
					return nil, fmt.Errorf("%s: default: %v", at, err)
				}
				if !f.Allows(value) {
					return nil, fmt.Errorf("%s: its default is a %s, but the field is a %s.", at, scripting.TypeName(def), f.Kind)
				}
				f.Default, f.HasDefault = value, true
			}
		default:
			return nil, fmt.Errorf("%s must be a description, like %s = \"how heavy it is\", or a table like { \"how heavy it is\", type = \"number\" }, not a %s.",
				at, name, scripting.TypeName(table[name]))
		}
		if strings.TrimSpace(f.Desc) == "" {
			return nil, fmt.Errorf("%s needs a description saying what it holds, like %s = \"how heavy it is\".", at, name)
		}
		fields = append(fields, f)
	}

	return fields, nil
}
