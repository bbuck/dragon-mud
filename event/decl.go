package event

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"bbuck.dev/dragon-mud/command"
)

// Field is one field of an event's payload.
type Field struct {
	Name string
	Desc string

	// Optional fields can be left out by whoever sends the event: they're
	// only sometimes there, or handlers set them.
	Optional bool
}

// Decl declares an event, a hook or a notification: what it's for and the
// fields its payload has. The plugin that sends an event declares it in
// events.declare; the engine declares its own.
type Decl struct {
	Name string

	// Plugin declares it; empty for the engine.
	Plugin string

	Desc   string
	Fields []Field

	// Extra, when set, says what any other fields are for, and lets an
	// event have fields beyond those declared.
	Extra string

	// Prefix makes the declaration cover every event whose name starts
	// with Name, such as section: for every view section.
	Prefix bool
}

// Where is who declares the event, for messages: the engine or a plugin.
func (d Decl) Where() string {
	if d.Plugin == "" {
		return "the engine"
	}

	return d.Plugin
}

// Field returns the field called name.
func (d Decl) Field(name string) (Field, bool) {
	for _, f := range d.Fields {
		if f.Name == name {
			return f, true
		}
	}

	return Field{}, false
}

// problem describes what's wrong with event for the event name, as the end
// of a sentence starting "the event", or returns "" when nothing is.
func (d Decl) problem(name string, event map[string]any) string {
	names := make([]string, len(d.Fields))
	for i, f := range d.Fields {
		names[i] = f.Name
	}

	if d.Extra == "" {
		for _, key := range slices.Sorted(maps.Keys(event)) {
			if _, ok := d.Field(key); ok {
				continue
			}
			fields := "It has no fields."
			if len(names) > 0 {
				fields = "Its fields are " + andList(names) + "."
			}
			return fmt.Sprintf("has a field %q, which %s doesn't have.%s %s Run dragon events %s to see what each is for.",
				key, name, command.DidYouMean(key, names), fields, name)
		}
	}

	for _, f := range d.Fields {
		if f.Optional {
			continue
		}
		if v, ok := event[f.Name]; !ok || v == nil {
			return fmt.Sprintf("is missing %s (%s), which %s needs.", f.Name, f.Desc, name)
		}
	}

	return ""
}

// undeclared describes sending an event that nothing declares.
func undeclared(name string, declared []string) error {
	return fmt.Errorf("no plugin declares the event %q.%s The plugin that sends an event declares it in its events.declare, like declare = { [%q] = { desc = \"...\", fields = { actor = \"who did it\" } } }.",
		name, command.DidYouMean(name, declared), name)
}

func andList(items []string) string {
	if len(items) == 1 {
		return items[0]
	}

	return strings.Join(items[:len(items)-1], ", ") + " and " + items[len(items)-1]
}
