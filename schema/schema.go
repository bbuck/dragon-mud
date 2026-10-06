// Package schema holds the types plugins declare for objects: what an
// object of a type is for, and the properties (fields) it has. An object
// can have several types, and gets its parents' too, so a bag can be both
// an item and a container. An object with types can only have the
// properties its types declare, which catches typos like descrition; an
// object with none takes any property, as before.
//
// Plugins add fields to other plugins' types with extensions. An added
// field is named for the plugin that adds it, mapping.coords for mapping's
// coords, so two plugins' fields never collide. See docs/plugins.md.
package schema

import (
	"fmt"
	"maps"
	"math"
	"slices"
	"strings"

	"bbuck.dev/dragon-mud/command"
	"bbuck.dev/dragon-mud/world"
)

// Kind is what a field's values may be.
type Kind string

// The kinds of field. Every kind also allows nil, for no value.
const (
	Any     Kind = "any"     // any value
	String  Kind = "string"  // a line of text, like a name
	Text    Kind = "text"    // text that may run to several lines, like a description
	Number  Kind = "number"  // any number
	Integer Kind = "integer" // a whole number
	Boolean Kind = "boolean" // true or false
	Object  Kind = "object"  // another object
	List    Kind = "list"    // a list of values
	Table   Kind = "table"   // a table keyed by name
)

// Kinds lists every kind, in the order docs give them.
var Kinds = []Kind{Any, String, Text, Number, Integer, Boolean, Object, List, Table}

// Field is a property objects of a type have.
type Field struct {
	// Name is the property's name: as declared for a type's own fields,
	// and after the adding plugin's namespace for added ones, like
	// mapping.coords.
	Name string
	Desc string
	Kind Kind

	// Default is what reading the property gives when no object in the
	// chain has it. HasDefault tells a nil default from none.
	Default    any
	HasDefault bool

	// Plugin declares the field: the type's plugin, an extending plugin,
	// or empty for the engine's own fields.
	Plugin string
}

// Allows reports whether v is a value the field may hold. Values are as
// world.Normalize leaves them.
func (f Field) Allows(v any) bool {
	if v == nil {
		return true
	}

	switch f.Kind {
	case String, Text:
		_, ok := v.(string)
		return ok
	case Number:
		switch v.(type) {
		case int64, float64:
			return true
		}
		return false
	case Integer:
		switch n := v.(type) {
		case int64:
			return true
		case float64:
			return n == math.Trunc(n)
		}
		return false
	case Boolean:
		_, ok := v.(bool)
		return ok
	case Object:
		_, ok := v.(world.Ref)
		return ok
	case List:
		_, ok := v.([]any)
		return ok
	case Table:
		switch t := v.(type) {
		case map[string]any:
			return true
		case []any:
			// An empty table is an empty list to the engine.
			return len(t) == 0
		}
		return false
	default:
		return true
	}
}

// Type is a kind of object a plugin declares, such as items:item.
type Type struct {
	Name   string
	Plugin string
	Desc   string

	// Fields are the type's own, sorted by name.
	Fields []Field

	// Added are fields other plugins add to it, sorted by name.
	Added []Field
}

// Extension adds fields to another plugin's type.
type Extension struct {
	// Type is the type it adds to.
	Type   string
	Plugin string

	// Where is where the plugin declares it, for messages.
	Where string

	// Fields are the added fields, already named with the adding
	// plugin's namespace.
	Fields []Field
}

// EngineFields are fields every type has, because the engine reads them:
// templates name an object with name, and use proper and article to write
// "the" and "a" (docs/design.md §5).
var EngineFields = []Field{
	{Name: "article", Kind: String, Desc: `the article that replaces "a" or "an" before its name, like "some", or "" for none`},
	{Name: "name", Kind: String, Desc: "what it's called, without an article"},
	{Name: "proper", Kind: Boolean, Desc: "true when its name never takes an article, as for people"},
}

// Registry is every declared type.
type Registry struct {
	types map[string]*Type
}

// New builds a registry from the plugins' types and extensions. Two
// plugins declaring one type, or a field declared twice, is an error. It
// returns the extensions whose type no plugin declares, which add nothing:
// a misspelled type, or one from a plugin the game doesn't load.
func New(types []Type, extensions []Extension) (*Registry, []Extension, error) {
	r := &Registry{types: make(map[string]*Type, len(types))}
	for _, t := range types {
		if other, ok := r.types[t.Name]; ok {
			return nil, nil, fmt.Errorf("%s and %s both declare the type %q. A type has one declaration, from the plugin that owns it; rename one of them, with its plugin's name in front.",
				other.Plugin, t.Plugin, t.Name)
		}
		for _, f := range t.Fields {
			if _, ok := engineField(f.Name); ok {
				return nil, nil, fmt.Errorf("%s: schema.types[%q] declares %s, which every type has already, since the engine reads it. Remove it.", t.Plugin, t.Name, f.Name)
			}
		}
		t.Fields = slices.Clone(t.Fields)
		r.types[t.Name] = &t
	}

	var unmatched []Extension
	for _, ext := range extensions {
		t, ok := r.types[ext.Type]
		if !ok {
			unmatched = append(unmatched, ext)
			continue
		}
		if t.Plugin == ext.Plugin {
			return nil, nil, fmt.Errorf("%s adds fields to %s, which the same plugin declares. Add them to its fields instead.", ext.Where, ext.Type)
		}
		for _, f := range ext.Fields {
			if _, ok := t.field(f.Name); ok {
				return nil, nil, fmt.Errorf("%s adds %s to %s, which has it already. Remove one of them.", ext.Where, f.Name, ext.Type)
			}
			t.Added = append(t.Added, f)
		}
		slices.SortFunc(t.Added, func(a, b Field) int { return strings.Compare(a.Name, b.Name) })
	}

	return r, unmatched, nil
}

// Type returns the type called name.
func (r *Registry) Type(name string) (*Type, bool) {
	t, ok := r.types[name]

	return t, ok
}

// Names returns every declared type, sorted.
func (r *Registry) Names() []string {
	return slices.Sorted(maps.Keys(r.types))
}

// Declared returns an error unless name is a declared type, suggesting
// the nearest when it isn't.
func (r *Registry) Declared(name string) error {
	if _, ok := r.types[name]; ok {
		return nil
	}

	names := r.Names()
	if len(names) == 0 {
		return fmt.Errorf("there's no type %q, and no plugin the game loads declares any. A plugin declares types in its schema, like schema = { types = { [%q] = { fields = { ... } } } }.", name, name)
	}
	return fmt.Errorf("there's no type %q.%s Types: %s.", name, command.DidYouMean(name, names), strings.Join(names, ", "))
}

// Checked reports whether an object with types has its properties checked:
// when it has at least one type, and every one is declared. An object with
// a type no plugin declares any more, such as one from a plugin the game
// stopped loading, takes any property, so it isn't stuck.
func (r *Registry) Checked(types []string) bool {
	if len(types) == 0 {
		return false
	}
	for _, name := range types {
		if _, ok := r.types[name]; !ok {
			return false
		}
	}

	return true
}

// Fields returns every field an object with types has: the engine's, then
// each type's own and added fields, each name once, sorted.
func (r *Registry) Fields(types []string) []Field {
	fields := slices.Clone(EngineFields)
	seen := make(map[string]bool)
	for _, f := range fields {
		seen[f.Name] = true
	}
	for _, name := range types {
		t, ok := r.types[name]
		if !ok {
			continue
		}
		for _, f := range append(slices.Clone(t.Fields), t.Added...) {
			if !seen[f.Name] {
				seen[f.Name] = true
				fields = append(fields, f)
			}
		}
	}
	slices.SortFunc(fields, func(a, b Field) int { return strings.Compare(a.Name, b.Name) })

	return fields
}

// Field returns the field name of an object with types. Several types can
// declare the same field; the first in types wins. It's an error, with a
// suggestion, for a field none of them has.
func (r *Registry) Field(types []string, name string) (Field, error) {
	if f, ok := engineField(name); ok {
		return f, nil
	}
	for _, typeName := range types {
		if t, ok := r.types[typeName]; ok {
			if f, ok := t.field(name); ok {
				return f, nil
			}
		}
	}

	var names []string
	suggest := ""
	for _, f := range r.Fields(types) {
		names = append(names, f.Name)
		if _, local, ok := strings.Cut(f.Name, "."); ok && local == name && suggest == "" {
			suggest = fmt.Sprintf(" Did you mean %q?", f.Name)
		}
	}
	if suggest == "" {
		suggest = command.DidYouMean(name, names)
	}
	return Field{}, fmt.Errorf("%q isn't a field of %s.%s Fields: %s. A plugin adds a field to another plugin's type with schema.extend.",
		name, typeList(types), suggest, strings.Join(names, ", "))
}

// Check returns an error unless an object with types may hold value, as
// world.Normalize leaves it, in the property name: every type declaring
// the field must allow it.
func (r *Registry) Check(types []string, name string, value any) error {
	if _, err := r.Field(types, name); err != nil {
		return err
	}

	if f, ok := engineField(name); ok {
		if !f.Allows(value) {
			return fmt.Errorf("%s is %s field, so it can't hold %s.", name, article(f.Kind), describe(value))
		}
		return nil
	}
	for _, typeName := range types {
		t, ok := r.types[typeName]
		if !ok {
			continue
		}
		if f, ok := t.field(name); ok && !f.Allows(value) {
			return fmt.Errorf("%s's %s is %s field, so it can't hold %s.", typeName, name, article(f.Kind), describe(value))
		}
	}

	return nil
}

func (t *Type) field(name string) (Field, bool) {
	for _, f := range t.Fields {
		if f.Name == name {
			return f, true
		}
	}
	for _, f := range t.Added {
		if f.Name == name {
			return f, true
		}
	}

	return Field{}, false
}

func engineField(name string) (Field, bool) {
	for _, f := range EngineFields {
		if f.Name == name {
			return f, true
		}
	}

	return Field{}, false
}

func typeList(types []string) string {
	if len(types) == 1 {
		return types[0]
	}

	return strings.Join(types[:len(types)-1], ", ") + " or " + types[len(types)-1]
}

func article(k Kind) string {
	switch k {
	case Any, Integer, Object:
		return "an " + string(k)
	}

	return "a " + string(k)
}

// describe names a value for messages: its kind, and short values
// themselves.
func describe(v any) string {
	switch v := v.(type) {
	case string:
		if len(v) <= 20 {
			return fmt.Sprintf("the string %q", v)
		}
		return "a string"
	case int64:
		return fmt.Sprintf("the number %d", v)
	case float64:
		return fmt.Sprintf("the number %g", v)
	case bool:
		return fmt.Sprintf("%t", v)
	case world.Ref:
		return "an object"
	case []any:
		return "a list"
	case map[string]any:
		return "a table"
	}

	return fmt.Sprintf("a %T", v)
}
