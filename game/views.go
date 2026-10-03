package game

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"

	"bbuck.dev/dragon-mud/command"
	"bbuck.dev/dragon-mud/message"
	"bbuck.dev/dragon-mud/plugin"
	"bbuck.dev/dragon-mud/scripting"
	"bbuck.dev/dragon-mud/view"
	"bbuck.dev/dragon-mud/world"
)

var viewNameRx = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

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
func (g *Game) render(name string, data map[string]any, block string) (message.Message, error) {
	if g.scripts == nil {
		return message.Message{}, errors.New("views can't be sent while plugins are loading; send them from a command or a hook handler")
	}
	if !g.views.Has(name) {
		if !viewNameRx.MatchString(name) {
			return message.Message{}, fmt.Errorf("%q isn't a view. Views are template names, lowercase letters, digits and underscores like \"say\". To send plain text, leave out the data table: send(text).", name)
		}
		return message.Message{}, fmt.Errorf("there's no view %q. Add %s/%s.txt.tmpl to your plugin to define it.%s",
			name, plugin.ViewsDir, name, command.DidYouMean(name, g.views.Names()))
	}

	td, err := g.templateData(data, "")
	if err != nil {
		return message.Message{}, fmt.Errorf("argument #2: %w", err)
	}

	// Sections run their hooks with the data as the script gave it.
	g.rendering = append(g.rendering, &rendering{view: name, data: data, parts: make(map[string][]view.Part)})
	defer func() { g.rendering = g.rendering[:len(g.rendering)-1] }()

	m := message.Message{Kind: name}
	if m.Text, _, err = g.views.Render(name, view.FormatText, block, td); err != nil {
		return message.Message{}, err
	}
	if m.HTML, _, err = g.views.Render(name, view.FormatHTML, block, td); err != nil {
		return message.Message{}, err
	}

	return m, nil
}

// rendering is a view being rendered, and the parts its sections got.
type rendering struct {
	view  string
	data  map[string]any
	parts map[string][]view.Part
}

// sectionHook is the hook that fills a view's section.
func sectionHook(name, section string) string {
	return "section:" + name + "." + section
}

// sectionParts runs the hook for a section of the message being rendered.
// Handlers add to event.parts: text, or { kind = ..., data = ...[, block =
// ...] }. A handler that cancels leaves the section empty.
func (g *Game) sectionParts(name, section string) ([]view.Part, error) {
	if len(g.rendering) == 0 {
		return nil, errors.New("sections only render in views scripts send")
	}
	r := g.rendering[len(g.rendering)-1]
	if parts, ok := r.parts[section]; ok {
		return parts, nil
	}

	hook := sectionHook(name, section)
	if _, ok := g.hooks.Chain(hook); !ok {
		r.parts[section] = nil
		return nil, nil
	}

	// Called from a running script, whose deadline applies.
	result, err := g.hooks.Run(context.Background(), hook, map[string]any{"data": r.data, "parts": []any{}})
	if err != nil || result.Cancelled {
		return nil, err
	}

	raw, ok := result.Payload["parts"].([]any)
	if !ok && result.Payload["parts"] != nil {
		if m, isMap := result.Payload["parts"].(map[string]any); !isMap || len(m) > 0 {
			return nil, fmt.Errorf("%s: event.parts must be a list, not a %s", hook, scripting.TypeName(result.Payload["parts"]))
		}
	}

	var parts []view.Part
	for i, item := range raw {
		part, err := g.sectionPart(item)
		if err != nil {
			return nil, fmt.Errorf("%s: part %d: %w", hook, i+1, err)
		}
		parts = append(parts, part)
	}
	r.parts[section] = parts

	return parts, nil
}

// sectionPart reads one part a section hook handler added.
func (g *Game) sectionPart(item any) (view.Part, error) {
	switch v := item.(type) {
	case string:
		return view.Part{Text: v}, nil
	case map[string]any:
		name, ok := v["view"].(string)
		if !ok {
			return view.Part{}, errors.New(`a part is text, or a table like { view = "minimap", data = { ... } }`)
		}
		if !g.views.Has(name) {
			return view.Part{}, fmt.Errorf("there's no view %q.%s", name, command.DidYouMean(name, g.views.Names()))
		}
		block, _ := v["block"].(string)
		data, err := g.templateData(v["data"], "data")
		if err != nil {
			return view.Part{}, err
		}
		return view.Part{View: name, Block: block, Data: data}, nil
	default:
		return view.Part{}, fmt.Errorf(`a part is text, or a table like { view = "minimap", data = { ... } }, not a %s`, scripting.TypeName(item))
	}
}

// checkSectionHooks checks that every section hook names a view and a
// section its template has.
func (s *scripts) checkSectionHooks() error {
	for _, hook := range s.hooks.Names() {
		target, ok := strings.CutPrefix(hook, "section:")
		if !ok {
			continue
		}
		name, section, _ := strings.Cut(target, ".")

		chain, _ := s.hooks.Chain(hook)
		var files []string
		for _, h := range append(chain.Handlers, chain.Disabled...) {
			files = append(files, h.File())
		}
		where := strings.Join(files, ", ")

		if !s.views.Has(name) {
			return fmt.Errorf("%s: %s adds to the view %q, which no plugin defines.%s",
				where, hook, name, command.DidYouMean(name, s.views.Names()))
		}
		sections := s.views.Sections(name)
		if !slices.Contains(sections, section) {
			has := "It has no sections; add {{section \"" + section + "\"}} where the parts should go."
			if len(sections) > 0 {
				has = "Its sections: " + strings.Join(sections, ", ") + "."
			}
			return fmt.Errorf("%s: %s adds to the %q section of %q, but the %s template has no {{section %q}}.%s %s",
				where, hook, section, name, name, section, command.DidYouMean(section, sections), has)
		}
	}

	return nil
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
func (g *Game) entity(o *world.Object, full bool) view.Entity {
	e := view.Entity{}
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
