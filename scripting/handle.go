package scripting

import (
	"fmt"
	"maps"
	"slices"
)

// Handle is a reference to something Go owns, such as a world object, that
// scripts hold and call methods on. Scripts can't see inside a handle; they
// only reach it through its Type's fields and methods.
//
// Handles with the same Type and Key are the same value to scripts: equal,
// and usable as the same table key.
type Handle struct {
	Type *Type

	// Key identifies what the handle refers to, such as an object id. It
	// must be comparable.
	Key any
}

// Type describes a kind of handle. Each language presents it in its own way;
// in Lua a handle is userdata, fields read as h.name and methods are called
// as h:name(...).
type Type struct {
	// Name is the type's script-facing name, such as "object".
	Name string

	// Fields are read-only values computed from the handle's key.
	Fields map[string]Field

	// Methods are functions called on a handle.
	Methods map[string]Method

	// String describes a handle for printing and error messages. Nil uses
	// the type name and key.
	String func(key any) string

	// Hint, if set, adds advice when a script reads a name the type doesn't
	// have, or assigns any name (assign is true then), such as pointing at
	// a method that does what the script meant.
	Hint func(name string, assign bool) string
}

// Names returns the type's field and method names, sorted, for errors.
func (t *Type) Names() (fields, methods []string) {
	fields = slices.Sorted(maps.Keys(t.Fields))
	methods = slices.Sorted(maps.Keys(t.Methods))

	return fields, methods
}

// Field computes a field's value for the handle with key.
type Field func(key any) (any, error)

// Method is called on the handle with key. args don't include the handle
// itself.
type Method func(key any, args Args) (any, error)

// Describe returns h's printable description.
func (h Handle) Describe() string {
	if h.Type.String != nil {
		return h.Type.String(h.Key)
	}

	return fmt.Sprintf("%s: %v", h.Type.Name, h.Key)
}
