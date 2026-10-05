package command

import (
	"context"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"bbuck.dev/dragon-mud/scripting"
)

// Built-in slot types.
const (
	// TypeText is free text: one or more words, taken as typed. Slots with
	// no type are text.
	TypeText = "text"

	// TypeWord is exactly one word.
	TypeWord = "word"

	// TypeNumber is one number.
	TypeNumber = "number"
)

var nameRx = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)

// Resolver turns the text a slot matched into a value. It returns ok false
// and a reason the player can read ("You don't see 'bob' here.") when the
// text doesn't resolve; err is for resolvers that broke. Resolvers run
// speculatively on every input, so they must not change anything.
// modifiers are the slot's modifiers as written; a resolver honors them
// as requirements that must all hold, each satisfied by any of its
// alternatives.
type Resolver func(ctx context.Context, actor any, text string, modifiers Modifiers) (value any, ok bool, reason string, err error)

// SlotType is a kind of slot, such as "object" or a plugin's "exit".
type SlotType struct {
	Name   string
	Desc   string
	Plugin string

	// Modifiers are the modifiers patterns may use with this type.
	Modifiers []string

	// Single types match exactly one word.
	Single bool

	Resolve Resolver
}

// CommandDef is a command as a plugin declares it.
type CommandDef struct {
	Name   string
	Desc   string
	Plugin string

	// Replace drops every form registered for this name before it, so only
	// this definition's forms (and any added after it) remain. Replacing
	// with no forms removes the command.
	Replace bool

	Forms []FormDef
}

// FormDef is one form of a command.
type FormDef struct {
	Pattern string
	Desc    string
	Execute scripting.Function
}

// Command is a registered command: its name and every form for it.
type Command struct {
	Name   string
	Desc   string
	Plugin string // the plugin that declared it first, or replaced it

	Forms []*Form
}

// Form is one registered form.
type Form struct {
	Command *Command
	Pattern Pattern
	Desc    string
	Plugin  string
	Execute scripting.Function

	// precedence is the plugin's load position: later plugins win ties.
	precedence int

	// order is the registration order: earlier forms win ties.
	order int
}

// Registry holds commands and slot types. Plugins register in precedence
// order: built-ins first, the game last.
type Registry struct {
	commands map[string]*Command
	slots    map[string]*SlotType
	plugins  map[string]int
	order    int
}

// NewRegistry returns a registry with the built-in slot types.
func NewRegistry() *Registry {
	r := &Registry{
		commands: make(map[string]*Command),
		slots:    make(map[string]*SlotType),
		plugins:  make(map[string]int),
	}

	r.slots[TypeText] = &SlotType{
		Name: TypeText, Desc: "Any text.", Plugin: "engine",
		Resolve: func(_ context.Context, _ any, text string, _ Modifiers) (any, bool, string, error) {
			return text, true, "", nil
		},
	}
	r.slots[TypeWord] = &SlotType{
		Name: TypeWord, Desc: "One word.", Plugin: "engine", Single: true,
		Resolve: func(_ context.Context, _ any, text string, _ Modifiers) (any, bool, string, error) {
			return text, true, "", nil
		},
	}
	r.slots[TypeNumber] = &SlotType{
		Name: TypeNumber, Desc: "A number.", Plugin: "engine", Single: true,
		Resolve: func(_ context.Context, _ any, text string, _ Modifiers) (any, bool, string, error) {
			n, err := strconv.ParseFloat(text, 64)
			if err != nil {
				return nil, false, fmt.Sprintf("'%s' isn't a number.", text), nil
			}
			return n, true, "", nil
		},
	}

	return r
}

// Scoped returns an empty registry for a separate set of commands, such as
// an input mode's forms. It shares r's slot types and plugin precedence, so
// slot types registered on either are visible to both.
func (r *Registry) Scoped() *Registry {
	return &Registry{
		commands: make(map[string]*Command),
		slots:    r.slots,
		plugins:  r.plugins,
	}
}

func (r *Registry) precedence(plugin string) int {
	p, ok := r.plugins[plugin]
	if !ok {
		p = len(r.plugins)
		r.plugins[plugin] = p
	}

	return p
}

// AddSlot registers a slot type. A name another plugin already registered
// is an error unless replace is set.
func (r *Registry) AddSlot(t SlotType, replace bool) error {
	r.precedence(t.Plugin)

	if !nameRx.MatchString(t.Name) {
		return fmt.Errorf("%s: slot type name %q isn't valid. Use lowercase letters, digits, - and _, starting with a letter.",
			t.Plugin, t.Name)
	}
	if t.Resolve == nil {
		return fmt.Errorf("%s: slot type %q needs a resolve function: resolve = function(actor, text, modifiers) ... end",
			t.Plugin, t.Name)
	}
	if existing, ok := r.slots[t.Name]; ok && !replace {
		return fmt.Errorf("%s: slot type %q is already provided by %s. Rename yours, or set replace = true on it to use %s's version instead.",
			t.Plugin, t.Name, existing.Plugin, t.Plugin)
	}

	r.slots[t.Name] = &t

	return nil
}

// Slot returns the slot type named name.
func (r *Registry) Slot(name string) (*SlotType, bool) {
	t, ok := r.slots[name]

	return t, ok
}

// Add registers a command's forms. Forms add to any already registered for
// the name unless def.Replace is set.
func (r *Registry) Add(def CommandDef) error {
	prec := r.precedence(def.Plugin)
	name := strings.ToLower(def.Name)

	if !nameRx.MatchString(name) {
		return fmt.Errorf("%s: command name %q isn't valid. Use lowercase letters, digits, - and _, starting with a letter; what players type goes in the command's forms.",
			def.Plugin, def.Name)
	}

	cmd, exists := r.commands[name]
	if def.Replace || !exists {
		cmd = &Command{Name: name, Plugin: def.Plugin}
	}
	if def.Desc != "" {
		cmd.Desc = def.Desc
	}

	// Compile and check every form before changing anything.
	var forms []*Form
	seen := r.patternKeys(name, def.Replace)
	for _, fd := range def.Forms {
		where := fmt.Sprintf("%s: the %q form %q", def.Plugin, name, fd.Pattern)
		if fd.Execute == nil {
			return fmt.Errorf("%s has no function to run. Write it as { %q, function(actor, args) ... end }.",
				where, fd.Pattern)
		}

		patterns, err := ParsePattern(fd.Pattern)
		if err != nil {
			return fmt.Errorf("%s: %w", where, err)
		}

		for _, p := range patterns {
			if err := r.checkSlots(p); err != nil {
				return fmt.Errorf("%s: %w", where, err)
			}

			key := p.key()
			if other, ok := seen[key]; ok {
				return duplicateError(def.Plugin, name, fd.Pattern, other)
			}

			r.order++
			form := &Form{
				Command:    cmd,
				Pattern:    p,
				Desc:       fd.Desc,
				Plugin:     def.Plugin,
				Execute:    fd.Execute,
				precedence: prec,
				order:      r.order,
			}
			seen[key] = form
			forms = append(forms, form)
		}
	}

	if def.Replace && len(forms) == 0 {
		delete(r.commands, name)
		return nil
	}

	cmd.Forms = append(cmd.Forms, forms...)
	r.commands[name] = cmd

	return nil
}

// patternKeys returns the pattern key of every registered form, leaving out
// the command being replaced.
func (r *Registry) patternKeys(replacing string, replace bool) map[string]*Form {
	keys := make(map[string]*Form)
	for name, cmd := range r.commands {
		if replace && name == replacing {
			continue
		}
		for _, f := range cmd.Forms {
			keys[f.Pattern.key()] = f
		}
	}

	return keys
}

// checkSlots checks that a pattern's slot types exist and that its
// modifiers are ones the types declare.
func (r *Registry) checkSlots(p Pattern) error {
	for _, e := range p.Elements {
		if e.Slot == nil || e.Slot.Type == "" {
			continue
		}

		t, ok := r.slots[e.Slot.Type]
		if !ok {
			known := slices.Sorted(maps.Keys(r.slots))
			return fmt.Errorf("slot <%s> uses type %q, which no loaded plugin provides.%s Known types: %s.",
				e.Slot.Name, e.Slot.Type, DidYouMean(e.Slot.Type, known), strings.Join(known, ", "))
		}
		for _, m := range e.Slot.Modifiers.Names() {
			if slices.Contains(t.Modifiers, m) {
				continue
			}
			if len(t.Modifiers) == 0 {
				return fmt.Errorf("slot <%s> gives type %q the modifier %q, but %q takes no modifiers. Remove \":%s\".",
					e.Slot.Name, t.Name, m, t.Name, e.Slot.Modifiers)
			}
			return fmt.Errorf("slot <%s> gives type %q the modifier %q, which it doesn't have.%s %q's modifiers: %s.",
				e.Slot.Name, t.Name, m, DidYouMean(m, t.Modifiers), t.Name, strings.Join(t.Modifiers, ", "))
		}
	}

	return nil
}

// Lookup returns the command named name.
func (r *Registry) Lookup(name string) (*Command, bool) {
	cmd, ok := r.commands[strings.ToLower(name)]

	return cmd, ok
}

// All returns every command, sorted by name.
func (r *Registry) All() []*Command {
	return slices.SortedFunc(maps.Values(r.commands), func(a, b *Command) int {
		return strings.Compare(a.Name, b.Name)
	})
}

// forms returns every form in registration order.
func (r *Registry) forms() []*Form {
	var all []*Form
	for _, cmd := range r.commands {
		all = append(all, cmd.Forms...)
	}
	slices.SortFunc(all, func(a, b *Form) int { return a.order - b.order })

	return all
}

// slotType returns the type a slot resolves through.
func (r *Registry) slotType(s *Slot) *SlotType {
	if s.Type == "" {
		return r.slots[TypeText]
	}

	return r.slots[s.Type]
}

// duplicateError explains a form that matches exactly the same input as one
// already registered, and how to fix it.
func duplicateError(plugin, command, pattern string, other *Form) error {
	same := fmt.Sprintf("%s: the %q form %q matches exactly the same input as %q",
		plugin, command, pattern, other.Pattern.Source)

	switch {
	case other.Plugin == plugin:
		return fmt.Errorf("%s, also in %s's %q command. Remove one of them.",
			same, plugin, other.Command.Name)
	case other.Command.Name == command:
		return fmt.Errorf("%s, already defined by %s. Remove it from %s, or set replace = true on %q in %s to use only %s's %q forms.",
			same, other.Plugin, plugin, command, plugin, plugin, command)
	default:
		return fmt.Errorf("%s from %s's %q command. Players couldn't reach one of them; remove it from %s or change its pattern.",
			same, other.Plugin, other.Command.Name, plugin)
	}
}
