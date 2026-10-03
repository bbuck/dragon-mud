package game

import (
	"errors"
	"fmt"
	"maps"
	"regexp"
	"slices"

	"bbuck.dev/dragon-mud/command"
	"bbuck.dev/dragon-mud/message"
	"bbuck.dev/dragon-mud/plugin"
	"bbuck.dev/dragon-mud/scripting"
	"bbuck.dev/dragon-mud/world"
)

var kindRx = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// outgoing reads the message a script asked to send from args: either
// (text) or (kind, data[, block]). It returns the message and how many
// arguments it used.
func (g *Game) outgoing(args scripting.Args) (message.Message, int, error) {
	first, err := args.String(0)
	if err != nil {
		return message.Message{}, 0, err
	}
	if !isTable(args, 1) {
		return message.Text(first), 1, nil
	}

	data, err := args.Map(1)
	if list, ok := args[1].([]any); ok && len(list) == 0 {
		data, err = map[string]any{}, nil
	}
	if err != nil {
		return message.Message{}, 0, err
	}

	var block string
	if args.Len() > 2 && args[2] != nil {
		if block, err = args.String(2); err != nil {
			return message.Message{}, 0, fmt.Errorf("%w (the block to render from the %s template, or nil for all of it)", err, first)
		}
	}

	m, err := g.render(first, data, block)

	return m, 3, err
}

// isTable reports whether argument i is a script table, as opposed to a
// handle, a string or nothing.
func isTable(args scripting.Args, i int) bool {
	if i >= args.Len() {
		return false
	}
	switch args[i].(type) {
	case map[string]any, []any:
		return true
	}

	return false
}

// render renders the message kind with data, or only its block when block
// isn't empty, in every format the kind has.
func (g *Game) render(kind string, data map[string]any, block string) (message.Message, error) {
	if g.scripts == nil {
		return message.Message{}, errors.New("messages can't be sent while plugins are loading; send them from a command or a hook handler")
	}
	if !g.messages.Has(kind) {
		if !kindRx.MatchString(kind) {
			return message.Message{}, fmt.Errorf("%q isn't a message kind. Kinds are template names, lowercase letters, digits and underscores like \"say\". To send plain text, leave out the data table: send(text).", kind)
		}
		return message.Message{}, fmt.Errorf("there's no message kind %q. Add %s/%s.txt.tmpl to your plugin to define it.%s",
			kind, plugin.MessagesDir, kind, command.DidYouMean(kind, g.messages.Names()))
	}

	td, err := g.templateData(data, "")
	if err != nil {
		return message.Message{}, fmt.Errorf("argument #2: %w", err)
	}

	m := message.Message{Kind: kind}
	if m.Text, _, err = g.messages.Render(kind, message.FormatText, block, td); err != nil {
		return message.Message{}, err
	}
	if m.HTML, _, err = g.messages.Render(kind, message.FormatHTML, block, td); err != nil {
		return message.Message{}, err
	}

	return m, nil
}

// templateData converts a script value for a template: objects become
// entities. path is where value is in the data, for errors.
func (g *Game) templateData(value any, path string) (any, error) {
	at := func(key string) string {
		if path == "" {
			return key
		}
		return path + "." + key
	}

	switch v := value.(type) {
	case scripting.Handle:
		if v.Type != g.objType {
			return nil, fmt.Errorf("%s: a %s can't be shown in a message", path, v.Type.Name)
		}
		o, err := g.object(v.Key)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		return g.entity(o, true), nil
	case scripting.Function:
		return nil, fmt.Errorf("%s: a function can't be shown in a message", path)
	case []any:
		list := make([]any, len(v))
		for i, item := range v {
			converted, err := g.templateData(item, fmt.Sprintf("%s[%d]", path, i+1))
			if err != nil {
				return nil, err
			}
			list[i] = converted
		}
		return list, nil
	case map[string]any:
		m := make(map[string]any, len(v))
		for key, item := range v {
			converted, err := g.templateData(item, at(key))
			if err != nil {
				return nil, err
			}
			m[key] = converted
		}
		return m, nil
	default:
		return v, nil
	}
}

// entity is how templates see o: its id, key and properties. Objects in
// its properties are entities with only id, key and name, so one message
// can't pull in the whole world. Without full, o gets only those too.
func (g *Game) entity(o *world.Object, full bool) message.Entity {
	e := message.Entity{}
	if full {
		for _, name := range allProperties(o) {
			value, _ := o.Get(name)
			e[name] = g.entityValue(value)
		}
	} else if name, ok := o.Get("name"); ok {
		e["name"] = name
	}

	e["id"] = string(o.ID())
	delete(e, "key")
	if o.Key() != "" {
		e["key"] = o.Key()
	}

	return e
}

// entityValue converts a property value for an entity: refs become
// entities with only id, key and name, or nil when their object is gone.
func (g *Game) entityValue(value any) any {
	switch v := value.(type) {
	case world.Ref:
		o, ok := g.world.Get(v.ID)
		if !ok {
			return nil
		}
		return g.entity(o, false)
	case []any:
		list := make([]any, len(v))
		for i, item := range v {
			list[i] = g.entityValue(item)
		}
		return list
	case map[string]any:
		m := make(map[string]any, len(v))
		for key, item := range v {
			m[key] = g.entityValue(item)
		}
		return m
	default:
		return v
	}
}

// allProperties returns the names of every property o has or inherits,
// sorted.
func allProperties(o *world.Object) []string {
	names := make(map[string]bool)
	for p := o; p != nil; p = p.Parent() {
		for _, name := range p.Properties() {
			names[name] = true
		}
	}

	return slices.Sorted(maps.Keys(names))
}
