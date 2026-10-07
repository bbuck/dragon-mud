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
	schemaFieldKeys = []string{"desc", "type", "of", "default"}
	shapeKeys       = []string{"type", "of"}
)

// Schema loads the types the plugin exports as schema.types and the fields
// it adds to other plugins' types as schema.extend, sorted by type name.
//
//	schema = {
//	  types = {
//	    ["items:item"] = {
//	      desc = "Something that can be carried.",
//	      fields = {
//	        description = { desc = "what players see when they look at it", type = "text" },
//	        weight = { desc = "how heavy it is, in pounds", type = "number", default = 1 },
//	        notes = "anything builders want to remember",
//	      },
//	    },
//	  },
//	  extend = {
//	    ["rooms:room"] = { fields = { coords = { desc = "where it is on the map", type = "table" } } },
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
// desc, type and default. prefix goes in front of each
// field's name, for fields added to another plugin's type. convert turns a
// default from a script value into a property value.
func (p *Plugin) schemaFields(where string, raw any, prefix string, convert func(any) (any, error)) ([]schema.Field, error) {
	table, err := asTable(where+".fields", raw, `fields = { weight = { desc = "how heavy it is", type = "number" } }`)
	if err != nil || table == nil {
		return nil, err
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
			if first, ok := v["1"].(string); ok {
				return nil, fmt.Errorf("%s names its description: write { desc = %q, ... }.", at, first)
			}
			if err := checkKeys(at, v, schemaFieldKeys, nil); err != nil {
				return nil, err
			}
			desc, ok := v["desc"].(string)
			if !ok {
				return nil, fmt.Errorf("%s needs desc, saying what it holds, like { desc = \"how heavy it is\", type = \"number\" }.", at)
			}
			f.Desc = desc

			if err := readShape(at, v, &f); err != nil {
				return nil, err
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
					return nil, fmt.Errorf("%s: its default doesn't fit the field, which holds %s.", at, f.Describe())
				}
				f.Default, f.HasDefault = value, true
			}
		default:
			return nil, fmt.Errorf("%s must be a description, like %s = \"how heavy it is\", or a table like { desc = \"how heavy it is\", type = \"number\" }, not a %s.",
				at, name, scripting.TypeName(table[name]))
		}
		if strings.TrimSpace(f.Desc) == "" {
			return nil, fmt.Errorf("%s needs a description saying what it holds, like %s = \"how heavy it is\".", at, name)
		}
		fields = append(fields, f)
	}

	return fields, nil
}

// readShape reads a field's type and, for a list or map, of: what its
// items or values are, a type name or a table like { type = "list", of =
// "string" }.
func readShape(at string, entry map[string]any, f *schema.Field) error {
	kinds := make([]string, len(schema.Kinds))
	for i, k := range schema.Kinds {
		kinds[i] = string(k)
	}

	if kind, set := entry["type"]; set {
		k, ok := kind.(string)
		switch {
		case k == "table":
			return fmt.Errorf("%s: type \"table\" is called map now, for values keyed by name. Write type = \"map\".", at)
		case !ok || !slices.Contains(kinds, k):
			return fmt.Errorf("%s: type must be one of %s, not %v.%s", at, andList(kinds), kind, command.DidYouMean(fmt.Sprint(kind), kinds))
		}
		f.Kind = schema.Kind(k)
	}

	of, set := entry["of"]
	if !set {
		return nil
	}
	if f.Kind != schema.List && f.Kind != schema.Map {
		return fmt.Errorf("%s: of says what a list's items or a map's values are, so it needs type = \"list\" or type = \"map\".", at)
	}

	item := &schema.Field{Kind: schema.Any}
	switch v := of.(type) {
	case string:
		if err := readShape(at+": of", map[string]any{"type": v}, item); err != nil {
			return err
		}
	case map[string]any:
		if err := checkKeys(at+": of", v, shapeKeys, nil); err != nil {
			return err
		}
		if _, typed := v["type"]; !typed {
			return fmt.Errorf("%s: of needs a type, like of = { type = \"list\", of = \"string\" }.", at)
		}
		if err := readShape(at+": of", v, item); err != nil {
			return err
		}
	default:
		return fmt.Errorf("%s: of must be a type, like of = \"string\", or a table like of = { type = \"list\", of = \"string\" }, not a %s.", at, scripting.TypeName(of))
	}
	f.Of = item

	return nil
}
