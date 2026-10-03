package plugin

import (
	"context"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"

	"bbuck.dev/dragon-mud/command"
	"bbuck.dev/dragon-mud/scripting"
)

// modeNameRx matches mode names: a name, optionally after its plugin's
// namespace, like choose_class or mapping:edit_map. The engine never adds a
// namespace; plugins namespace their own modes so they don't collide.
var modeNameRx = regexp.MustCompile(`^([a-z][a-z0-9_-]*:)?[a-z][a-z0-9_]*$`)

// LoginMode is the engine's own login mode. No plugin defines or pushes it.
const LoginMode = BuiltinPrefix + "login"

// modeKeys are the fields a mode entry in modes.lua may have.
var modeKeys = []string{"desc", "forms", "input", "enter", "leave", "resume", "passthrough", "replace"}

// ModeHandlers are the functions a mode may define, in the order they're
// described in docs.
var ModeHandlers = []string{"enter", "input", "resume", "leave"}

// ModeDef is an input mode as modes.lua declares it.
type ModeDef struct {
	Name    string
	Desc    string
	Plugin  string
	File    string
	Replace bool

	// Passthrough is nil when the definition doesn't say.
	Passthrough *bool

	// Handlers are the functions it defines, keyed by ModeHandlers names.
	Handlers map[string]scripting.Function

	Forms []command.FormDef
}

// Modes loads the input modes the plugin's modes.lua returns, sorted by
// name.
//
//	return {
//	  confirm = {
//	    enter = function(session, state) session:prompt("Sure? (yes/no)") end,
//	    forms = {
//	      { "yes", function(session, args, state) ... end },
//	      { "no", function(session, args, state) session:pop_mode() end },
//	    },
//	  },
//	  editor = { input = function(session, line, state) ... end },
//	}
func (p *Plugin) Modes(ctx context.Context) ([]ModeDef, error) {
	file := p.ID + "/modes.lua"
	table, err := p.evalTable(ctx, "modes.lua", file)
	if err != nil || table == nil {
		return nil, err
	}

	var defs []ModeDef
	for _, name := range slices.Sorted(maps.Keys(table)) {
		def, err := p.modeDef(file, name, table[name])
		if err != nil {
			return nil, err
		}
		defs = append(defs, def)
	}

	return defs, nil
}

func (p *Plugin) modeDef(file, name string, raw any) (ModeDef, error) {
	where := fmt.Sprintf("%s: mode %q", file, name)
	def := ModeDef{Name: name, Plugin: p.ID, File: file, Handlers: map[string]scripting.Function{}}

	switch {
	case !modeNameRx.MatchString(name):
		return def, fmt.Errorf("%s isn't a valid mode name. Mode names are lowercase letters, digits and underscores, starting with a letter, like choose_class, optionally after a namespace, like mapping:edit_map.", where)
	case name == LoginMode:
		return def, fmt.Errorf("%s is the engine's own login, which plugins can't define. Name your mode something else.", where)
	case strings.HasPrefix(name, BuiltinPrefix) && !strings.HasPrefix(p.ID, BuiltinPrefix):
		return def, fmt.Errorf("%s uses the %q namespace, which is reserved for the engine's built-in plugins. Use your plugin's name instead, like %q.",
			where, BuiltinPrefix, p.Manifest.Name+":"+strings.TrimPrefix(name, BuiltinPrefix))
	}
	entry, ok := raw.(map[string]any)
	if !ok {
		return def, fmt.Errorf("%s must be a table such as { input = function(session, line, state) ... end }, not a %s.",
			where, scripting.TypeName(raw))
	}
	if err := checkKeys(where, entry, modeKeys, nil); err != nil {
		return def, err
	}

	var err error
	if def.Desc, err = optional[string](where, entry, "desc", "a string"); err != nil {
		return def, err
	}
	if def.Replace, err = optional[bool](where, entry, "replace", "true or false"); err != nil {
		return def, err
	}
	if v, ok := entry["passthrough"]; ok && v != nil {
		b, ok := v.(bool)
		if !ok {
			return def, fmt.Errorf("%s: passthrough must be true or false, not a %s.", where, scripting.TypeName(v))
		}
		def.Passthrough = &b
	}

	signatures := map[string]string{
		"enter":  "function(session, state) ... end",
		"input":  "function(session, line, state) ... end",
		"resume": "function(session, state, result) ... end",
		"leave":  "function(session, state, reason) ... end",
	}
	for _, h := range ModeHandlers {
		v, ok := entry[h]
		if !ok || v == nil {
			continue
		}
		fn, ok := v.(scripting.Function)
		if !ok {
			return def, fmt.Errorf("%s: %s must be %s, not a %s.", where, h, signatures[h], scripting.TypeName(v))
		}
		def.Handlers[h] = fn
	}

	if rawForms, ok := entry["forms"]; ok {
		forms, ok := rawForms.([]any)
		if !ok {
			return def, fmt.Errorf("%s: forms must be a list of forms, like { { \"yes\", function(session, args, state) ... end } }, not a %s.",
				where, scripting.TypeName(rawForms))
		}
		for i, rawForm := range forms {
			form, err := ParseForm(fmt.Sprintf("%s form #%d", where, i+1), rawForm)
			if err != nil {
				return def, err
			}
			def.Forms = append(def.Forms, form)
		}
	}

	return def, nil
}
