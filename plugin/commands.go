package plugin

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"

	"bbuck.dev/dragon-mud/command"
	"bbuck.dev/dragon-mud/scripting"
)

// commandKeys are the fields a command entry in commands.lua may have.
var commandKeys = []string{"desc", "forms", "execute", "replace"}

// slotKeys are the fields a slot type entry in slots.lua may have.
var slotKeys = []string{"desc", "modifiers", "resolve", "single", "replace"}

// Commands loads the commands the plugin's commands.lua returns. A plugin
// without commands.lua has no commands. Commands are returned sorted by
// name; each command's forms keep the order they're written in.
//
//	return {
//	  say = {
//	    desc = "Say something.",
//	    forms = {
//	      { "say <message>", function(actor, args) ... end },
//	      { "say <message> to <target:object:here>", function(actor, args) ... end, desc = "..." },
//	    },
//	  },
//	  dance = { execute = function(actor, args) ... end },  -- the form "dance [<text>]"
//	}
func (p *Plugin) Commands(ctx context.Context) ([]command.CommandDef, error) {
	file := p.ID + "/commands.lua"
	table, err := p.evalTable(ctx, "commands.lua", file)
	if err != nil || table == nil {
		return nil, err
	}

	var defs []command.CommandDef
	for _, name := range slices.Sorted(maps.Keys(table)) {
		def, err := p.commandDef(file, name, table[name])
		if err != nil {
			return nil, err
		}
		defs = append(defs, def)
	}

	return defs, nil
}

func (p *Plugin) commandDef(file, name string, raw any) (command.CommandDef, error) {
	where := fmt.Sprintf("%s: command %q", file, name)

	if raw == false {
		return command.CommandDef{}, fmt.Errorf("%s is false. To remove a command, write %s = { replace = true } instead.", where, name)
	}
	entry, ok := raw.(map[string]any)
	if !ok {
		return command.CommandDef{}, fmt.Errorf("%s must be a table such as { desc = \"...\", forms = { ... } }, not a %s.",
			where, scripting.TypeName(raw))
	}
	if err := checkKeys(where, entry, commandKeys, map[string]string{
		"override": "override was renamed: set replace = true to replace other plugins' forms of this command, or leave it out to add to them.",
	}); err != nil {
		return command.CommandDef{}, err
	}

	def := command.CommandDef{Name: name, Plugin: p.ID}
	var err error
	if def.Desc, err = optional[string](where, entry, "desc", "a string"); err != nil {
		return def, err
	}
	if def.Replace, err = optional[bool](where, entry, "replace", "true or false"); err != nil {
		return def, err
	}

	if execute, ok := entry["execute"]; ok {
		if _, hasForms := entry["forms"]; hasForms {
			return def, fmt.Errorf("%s has both execute and forms. Use execute for a simple command, or move it into forms as { %q, function(actor, args) ... end }.",
				where, name+" [<text>]")
		}
		fn, ok := execute.(scripting.Function)
		if !ok {
			return def, fmt.Errorf("%s: execute must be a function, not a %s.", where, scripting.TypeName(execute))
		}
		def.Forms = []command.FormDef{{Pattern: name + " [<text>]", Execute: fn}}
		return def, nil
	}

	rawForms, ok := entry["forms"]
	if !ok {
		if def.Replace {
			return def, nil // replacing with nothing removes the command
		}
		return def, fmt.Errorf("%s has no forms. Add forms = { { %q, function(actor, args) ... end } }, or execute = function(actor, args) ... end for a simple command.",
			where, name+" <text>")
	}
	forms, ok := rawForms.([]any)
	if !ok {
		return def, fmt.Errorf("%s: forms must be a list of forms, like { { %q, function(actor, args) ... end } }, not a %s.",
			where, name+" <text>", scripting.TypeName(rawForms))
	}

	for i, rawForm := range forms {
		form, err := ParseForm(fmt.Sprintf("%s form #%d", where, i+1), rawForm)
		if err != nil {
			return def, err
		}
		def.Forms = append(def.Forms, form)
	}

	return def, nil
}

// ParseForm reads { "pattern", function, desc = "..." }.
func ParseForm(where string, raw any) (command.FormDef, error) {
	shape := `{ "say <message>", function(actor, args) ... end }`

	var pattern, fn, desc any
	switch v := raw.(type) {
	case []any:
		if len(v) > 0 {
			pattern = v[0]
		}
		if len(v) > 1 {
			fn = v[1]
		}
		if len(v) > 2 {
			return command.FormDef{}, fmt.Errorf("%s has %d items; a form is %s, with an optional desc = \"...\".", where, len(v), shape)
		}
	case map[string]any:
		pattern, fn, desc = v["1"], v["2"], v["desc"]
		for key := range v {
			if key != "1" && key != "2" && key != "desc" {
				return command.FormDef{}, fmt.Errorf("%s has an unexpected field %q. A form is %s, with an optional desc = \"...\".", where, key, shape)
			}
		}
	default:
		return command.FormDef{}, fmt.Errorf("%s must be a table like %s, not a %s.", where, shape, scripting.TypeName(raw))
	}

	form := command.FormDef{}
	var ok bool
	if form.Pattern, ok = pattern.(string); !ok {
		return form, fmt.Errorf("%s must start with its pattern as a string, like %s.", where, shape)
	}
	if form.Execute, ok = fn.(scripting.Function); !ok {
		return form, fmt.Errorf("%s (%q) needs a function after its pattern, like %s.", where, form.Pattern, shape)
	}
	if desc != nil {
		if form.Desc, ok = desc.(string); !ok {
			return form, fmt.Errorf("%s (%q): desc must be a string.", where, form.Pattern)
		}
	}

	return form, nil
}

// SlotDef is a slot type as slots.lua declares it.
type SlotDef struct {
	Name      string
	Desc      string
	Modifiers []string
	Single    bool
	Replace   bool

	// Resolve is called as resolve(actor, text, modifiers, requirements)
	// and returns the value, or nil and a reason the player can read.
	// modifiers is the set the slot names, { open = true };
	// requirements is how they combine: { { "here", "held" }, { "online" } }
	// for here|held,online.
	Resolve scripting.Function
}

// Slots loads the slot types the plugin's slots.lua returns.
//
//	return {
//	  exit = {
//	    desc = "A way out of the room.",
//	    modifiers = { "open" },
//	    resolve = function(actor, text, modifiers) return exit_or_nil, "reason" end,
//	  },
//	}
func (p *Plugin) Slots(ctx context.Context) ([]SlotDef, error) {
	file := p.ID + "/slots.lua"
	table, err := p.evalTable(ctx, "slots.lua", file)
	if err != nil || table == nil {
		return nil, err
	}

	var defs []SlotDef
	for _, name := range slices.Sorted(maps.Keys(table)) {
		where := fmt.Sprintf("%s: slot type %q", file, name)
		entry, ok := table[name].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%s must be a table such as { resolve = function(actor, text, modifiers) ... end }, not a %s.",
				where, scripting.TypeName(table[name]))
		}
		if err := checkKeys(where, entry, slotKeys, nil); err != nil {
			return nil, err
		}

		def := SlotDef{Name: name}
		if def.Desc, err = optional[string](where, entry, "desc", "a string"); err != nil {
			return nil, err
		}
		if def.Single, err = optional[bool](where, entry, "single", "true or false"); err != nil {
			return nil, err
		}
		if def.Replace, err = optional[bool](where, entry, "replace", "true or false"); err != nil {
			return nil, err
		}

		if raw, ok := entry["modifiers"]; ok {
			list, ok := raw.([]any)
			if !ok {
				return nil, fmt.Errorf("%s: modifiers must be a list of names like { \"here\", \"held\" }, not a %s.", where, scripting.TypeName(raw))
			}
			for _, m := range list {
				s, ok := m.(string)
				if !ok || s == "" || strings.ContainsAny(s, ":,<> ") {
					return nil, fmt.Errorf("%s: modifier %v isn't valid. Modifiers are single words like \"here\".", where, m)
				}
				def.Modifiers = append(def.Modifiers, s)
			}
		}

		resolve, ok := entry["resolve"].(scripting.Function)
		if !ok {
			return nil, fmt.Errorf("%s needs resolve = function(actor, text, modifiers) ... end, returning the value, or nil and a reason the player will see.", where)
		}
		def.Resolve = resolve

		defs = append(defs, def)
	}

	return defs, nil
}

// evalTable evaluates file, which must return a table. A missing file
// returns nil, nil.
func (p *Plugin) evalTable(ctx context.Context, file, scriptName string) (map[string]any, error) {
	value, err := p.eval(ctx, file, scriptName)
	if err != nil || value == nil {
		return nil, err
	}

	switch v := value.(type) {
	case map[string]any:
		return v, nil
	case []any:
		if len(v) == 0 {
			return map[string]any{}, nil
		}
		return nil, fmt.Errorf("%s returns a list, but it must return a table keyed by name, like return { look = { ... } }.", scriptName)
	default:
		return nil, fmt.Errorf("%s must return a table keyed by name, like return { look = { ... } }, but it returns a %s.",
			scriptName, scripting.TypeName(value))
	}
}

// checkKeys rejects fields entry doesn't allow, with a hint for known
// mistakes.
func checkKeys(where string, entry map[string]any, allowed []string, hints map[string]string) error {
	for _, key := range slices.Sorted(maps.Keys(entry)) {
		if slices.Contains(allowed, key) {
			continue
		}
		if hint, ok := hints[key]; ok {
			return fmt.Errorf("%s: %s", where, hint)
		}
		return fmt.Errorf("%s has an unknown field %q.%s Allowed fields: %s.",
			where, key, command.DidYouMean(key, allowed), strings.Join(allowed, ", "))
	}

	return nil
}

// optional reads an optional field of type T.
func optional[T any](where string, entry map[string]any, key, want string) (T, error) {
	var zero T
	raw, ok := entry[key]
	if !ok || raw == nil {
		return zero, nil
	}
	v, ok := raw.(T)
	if !ok {
		return zero, fmt.Errorf("%s: %s must be %s, not a %s.", where, key, want, scripting.TypeName(raw))
	}

	return v, nil
}
