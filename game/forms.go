package game

import (
	"context"
	"fmt"

	"bbuck.dev/dragon-mud/command"
	"bbuck.dev/dragon-mud/plugin"
	"bbuck.dev/dragon-mud/scripting"
)

// formsModule is the "forms" scripting module: sets of forms a script
// parses input with itself, such as an NPC's own commands. A set uses the
// same patterns, slot types and scoring as commands.
//
//	forms.new(forms)             a form set from a list of forms, each
//	                             { "pattern", function(actor, args, ...) ... end }
//
//	set:parse(actor, line, ...)  run the form line matches as
//	                             fn(actor, args, ...) and return true, or
//	                             return false and the near miss: a table
//	                             with reason (if a slot didn't resolve)
//	                             and usage (patterns whose first word
//	                             matched)
func (g *Game) formsModule(s *scripts) scripting.Module {
	return scripting.Module{
		Name: "dragon.forms",
		Funcs: map[string]scripting.Func{
			"new": func(args scripting.Args) (any, error) {
				return g.newFormSet(s, args)
			},
		},
	}
}

// formSetType is how scripts see a form set. Its key is the set's
// registry, which belongs to the engine that made it.
func (g *Game) formSetType() *scripting.Type {
	return &scripting.Type{
		Name: "form set",
		Methods: map[string]scripting.Method{
			"parse": g.mutating(g.formSetParse),
		},
		String: func(key any) string {
			return fmt.Sprintf("form set (%d forms)", len(key.(*command.Registry).All()[0].Forms))
		},
	}
}

func (g *Game) newFormSet(s *scripts, args scripting.Args) (any, error) {
	shape := `forms.new { { "buy <item>", function(actor, args) ... end } }`

	var first any
	if args.Len() > 0 {
		first = args[0]
	}

	// An empty table reads as a map, not a list.
	list, err := args.List(0)
	if m, isMap := first.(map[string]any); err != nil && isMap && len(m) == 0 {
		list, err = nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("takes a list of forms, like %s, not a %s.", shape, scripting.TypeName(first))
	}
	if len(list) == 0 {
		return nil, fmt.Errorf("was given no forms. Pass at least one, like %s.", shape)
	}

	// Errors from Add name the plugin; the script's file and line come
	// with the error anyway.
	owner := s.loading
	if owner == "" {
		owner = "a script"
	}

	def := command.CommandDef{Name: "forms", Plugin: owner}
	for i, raw := range list {
		form, err := plugin.ParseForm(fmt.Sprintf("form #%d", i+1), raw)
		if err != nil {
			return nil, err
		}
		def.Forms = append(def.Forms, form)
	}

	r := s.commands.Scoped()
	if err := r.Add(def); err != nil {
		return nil, err
	}

	return scripting.Handle{Type: g.formType, Key: r}, nil
}

func (g *Game) formSetParse(key any, args scripting.Args) (any, error) {
	actor, err := args.Handle(0, g.objType)
	if err != nil {
		return nil, err
	}
	line, err := args.String(1)
	if err != nil {
		return nil, err
	}

	// Called from a running script, whose deadline applies.
	ctx := context.Background()

	g.resolving = true
	match, miss, err := key.(*command.Registry).Parse(ctx, actor, line)
	g.resolving = false
	if err != nil {
		return nil, err
	}

	if miss != nil {
		usage := make([]any, len(miss.Usage))
		for i, u := range miss.Usage {
			usage[i] = u
		}
		near := map[string]any{"usage": usage}
		if miss.Reason != "" {
			near["reason"] = miss.Reason
		}
		return scripting.Results{false, near}, nil
	}

	call := append([]any{actor, match.Args}, args[2:]...)
	if _, err := match.Form.Execute.Call(ctx, call...); err != nil {
		return nil, fmt.Errorf("the form %q failed: %w", match.Form.Pattern.Source, err)
	}

	return true, nil
}
