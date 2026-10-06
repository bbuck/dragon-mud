package game

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"bbuck.dev/dragon-mud/scripting"
	"bbuck.dev/dragon-mud/world"
)

// objectType is how scripts see world objects. A handle holds the object's
// id, not the object, so a handle to a destroyed object raises an error
// instead of reaching freed state.
//
//	o.id                       the object's id
//	o.key                      its unique builder key, or nil
//	o.parent                   the object it inherits from, or nil
//	o.location                 the object containing it, or nil
//	o.contents                 list of objects inside it
//	o.types                    its schema types, then its parents'
//
//	o:get(name)                property value, inherited from parents, or
//	                           its field's default
//	o:is_a(other)              true if o is other or inherits from it
//	o:is_player()              true if an account owns o as a character
//	o:get_own(name)            property value only if o has its own
//	o:set(name, value)         set a property; objects are stored as refs
//	o:delete(name)             remove o's own value
//	o:properties()             names of o's own properties
//	o:move_to(location)        move inside location (nil for nowhere)
//	o:set_parent(parent)       inherit from parent (nil for none)
//	o:set_key(key)             set the unique key (nil to remove)
//	o:add_type(name)           give o a schema type
//	o:remove_type(name)        take a type of o's own away
//	o:has_type(name)           true if o or a parent has the type
//	o:send(text)               send text to everyone playing o
//	o:send(kind, data[, block]) send a message kind to everyone playing o,
//	                           rendering only block if given
func (g *Game) objectType() *scripting.Type {
	return &scripting.Type{
		Name: "object",
		Fields: map[string]scripting.Field{
			"id": func(key any) (any, error) {
				o, err := g.object(key)
				if err != nil {
					return nil, err
				}
				return string(o.ID()), nil
			},
			"key": func(key any) (any, error) {
				o, err := g.object(key)
				if err != nil || o.Key() == "" {
					return nil, err
				}
				return o.Key(), nil
			},
			"parent": func(key any) (any, error) {
				o, err := g.object(key)
				if err != nil {
					return nil, err
				}
				return g.handle(o.Parent()), nil
			},
			"location": func(key any) (any, error) {
				o, err := g.object(key)
				if err != nil {
					return nil, err
				}
				return g.handle(o.Location()), nil
			},
			"contents": func(key any) (any, error) {
				o, err := g.object(key)
				if err != nil {
					return nil, err
				}
				contents := []any{}
				for _, item := range o.Contents() {
					contents = append(contents, g.handle(item))
				}
				return contents, nil
			},
			"types": func(key any) (any, error) {
				o, err := g.object(key)
				if err != nil {
					return nil, err
				}
				types := []any{}
				for _, t := range o.AllTypes() {
					types = append(types, t)
				}
				return types, nil
			},
		},
		Methods: map[string]scripting.Method{
			"get":         g.objectGet(true),
			"get_own":     g.objectGet(false),
			"is_a":        g.objectIsA,
			"is_player":   g.objectIsPlayer,
			"set":         g.mutating(g.objectSet),
			"delete":      g.mutating(g.objectDelete),
			"properties":  g.objectProperties,
			"move_to":     g.mutating(g.objectMoveTo),
			"set_parent":  g.mutating(g.objectSetParent),
			"set_key":     g.mutating(g.objectSetKey),
			"send":        g.mutating(g.objectSend),
			"add_type":    g.mutating(g.objectAddType),
			"remove_type": g.mutating(g.objectRemoveType),
			"has_type":    g.objectHasType,
		},
		String: func(key any) string {
			if o, ok := g.world.Get(key.(world.ID)); ok {
				return "object " + o.String()
			}
			return fmt.Sprintf("object %s (destroyed)", key)
		},
		Hint: func(name string, assign bool) string {
			switch {
			case name == "location" && assign:
				return "Move it with o:move_to(place)."
			case name == "parent" && assign:
				return "Set it with o:set_parent(parent)."
			case name == "key" && assign:
				return "Set it with o:set_key(key)."
			case name == "types" && assign:
				return "Change them with o:add_type(name) and o:remove_type(name)."
			case assign:
				return fmt.Sprintf("To store %s as a property, write o:set(%q, value).", name, name)
			default:
				return fmt.Sprintf("If %s is a property, read it with o:get(%q).", name, name)
			}
		},
	}
}

// worldModule is the "world" scripting module.
//
//	world.create([options])    a new object; options is a table with any of
//	                           parent, location, key, types and properties
//	world.get(id)              the object with id, or nil
//	world.keyed(key)           the object with the builder key, or nil
//	world.destroy(o)           destroy o; its contents move to its location
func (g *Game) worldModule() scripting.Module {
	return scripting.Module{
		Name: "dragon.world",
		Funcs: map[string]scripting.Func{
			"create": g.mutatingFunc(g.scriptCreate),
			"get": func(args scripting.Args) (any, error) {
				id, err := args.String(0)
				if err != nil {
					return nil, err
				}
				o, _ := g.world.Get(world.ID(id))
				return g.handle(o), nil
			},
			"keyed": func(args scripting.Args) (any, error) {
				key, err := args.String(0)
				if err != nil {
					return nil, err
				}
				o, _ := g.world.Keyed(key)
				return g.handle(o), nil
			},
			"destroy": g.mutatingFunc(func(args scripting.Args) (any, error) {
				o, err := g.objectArg(args, 0)
				if err != nil {
					return nil, err
				}
				for _, p := range g.players {
					if p.character == o {
						return nil, errors.New("can't destroy an object someone is playing")
					}
				}
				g.world.Destroy(o)
				return nil, nil
			}),
		},
	}
}

// createOptions are the options world.create accepts.
var createOptions = []string{"parent", "location", "key", "types", "properties"}

// scriptCreate creates an object, applying any options. If an option fails
// the object is destroyed again, so a failed create leaves nothing behind.
func (g *Game) scriptCreate(args scripting.Args) (any, error) {
	var opts map[string]any
	if args.Len() > 0 && args[0] != nil {
		var err error
		if opts, err = args.Map(0); err != nil {
			return nil, err
		}
	}
	for name := range opts {
		if !slices.Contains(createOptions, name) {
			return nil, fmt.Errorf("unknown option %q (expected %s)", name, strings.Join(createOptions, ", "))
		}
	}

	o := g.world.Create()
	if err := g.applyCreateOptions(o, opts); err != nil {
		g.world.Destroy(o)
		return nil, err
	}

	return g.handle(o), nil
}

func (g *Game) applyCreateOptions(o *world.Object, opts map[string]any) error {
	option := func(name string) (*world.Object, error) {
		h, ok := opts[name].(scripting.Handle)
		switch {
		case opts[name] == nil:
			return nil, nil
		case !ok || h.Type != g.objType:
			return nil, fmt.Errorf("%s: expected object, got %s", name, scripting.TypeName(opts[name]))
		}
		target, err := g.object(h.Key)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		return target, nil
	}

	parent, err := option("parent")
	if err != nil {
		return err
	}
	if err := o.SetParent(parent); err != nil {
		return err
	}

	location, err := option("location")
	if err != nil {
		return err
	}
	if err := o.MoveTo(location); err != nil {
		return err
	}

	if key, ok := opts["key"]; ok {
		s, ok := key.(string)
		if !ok {
			return fmt.Errorf("key: expected string, got %s", scripting.TypeName(key))
		}
		if err := o.SetKey(s); err != nil {
			return err
		}
	}

	if types, ok := opts["types"]; ok {
		list, isList := types.([]any)
		if !isList {
			if m, isMap := types.(map[string]any); !isMap || len(m) > 0 {
				return fmt.Errorf("types: expected a list of type names, like types = { \"items:item\" }, got %s", scripting.TypeName(types))
			}
		}
		for i, item := range list {
			name, ok := item.(string)
			if !ok {
				return fmt.Errorf("types: #%d: expected a type name, got %s", i+1, scripting.TypeName(item))
			}
			if err := g.schema.Declared(name); err != nil {
				return fmt.Errorf("types: %w", err)
			}
			if err := o.AddType(name); err != nil {
				return err
			}
		}
	}

	if props, ok := opts["properties"]; ok {
		m, ok := props.(map[string]any)
		if !ok {
			return fmt.Errorf("properties: expected map, got %s", scripting.TypeName(props))
		}
		for _, name := range slices.Sorted(maps.Keys(m)) {
			if err := structuralName(name); err != nil {
				return fmt.Errorf("properties: %w", err)
			}
			value, err := g.fromScript(m[name])
			if err != nil {
				return fmt.Errorf("properties: %q: %w", name, err)
			}
			if err := g.checkProperty(o, name, value); err != nil {
				return fmt.Errorf("properties: %w", err)
			}
			if err := o.Set(name, value); err != nil {
				return fmt.Errorf("properties: %w", err)
			}
		}
	}

	return nil
}

// mutating wraps a method that changes the world so it fails while slot
// resolvers run.
func (g *Game) mutating(m scripting.Method) scripting.Method {
	return func(key any, args scripting.Args) (any, error) {
		if err := g.writable(); err != nil {
			return nil, err
		}
		return m(key, args)
	}
}

// mutatingFunc is mutating for module functions.
func (g *Game) mutatingFunc(f scripting.Func) scripting.Func {
	return func(args scripting.Args) (any, error) {
		if err := g.writable(); err != nil {
			return nil, err
		}
		return f(args)
	}
}

// handle returns the script handle for o, or nil for no object.
func (g *Game) handle(o *world.Object) any {
	if o == nil {
		return nil
	}

	return scripting.Handle{Type: g.objType, Key: o.ID()}
}

// object returns the live object a handle's key refers to.
func (g *Game) object(key any) (*world.Object, error) {
	o, ok := g.world.Get(key.(world.ID))
	if !ok {
		return nil, world.ErrDestroyed
	}

	return o, nil
}

// objectArg returns argument i as a live object.
func (g *Game) objectArg(args scripting.Args, i int) (*world.Object, error) {
	h, err := args.Handle(i, g.objType)
	if err != nil {
		return nil, err
	}
	o, err := g.object(h.Key)
	if err != nil {
		return nil, fmt.Errorf("argument #%d: %w", i+1, err)
	}

	return o, nil
}

// optionalObjectArg is objectArg, allowing nil.
func (g *Game) optionalObjectArg(args scripting.Args, i int) (*world.Object, error) {
	if i >= args.Len() || args[i] == nil {
		return nil, nil
	}

	return g.objectArg(args, i)
}

// toScript converts a property value for scripts: refs become handles, or
// nil when their object is gone.
func (g *Game) toScript(value any) any {
	switch v := value.(type) {
	case world.Ref:
		o, _ := g.world.Get(v.ID)
		return g.handle(o)
	case []any:
		list := make([]any, len(v))
		for i, item := range v {
			list[i] = g.toScript(item)
		}
		return list
	case map[string]any:
		m := make(map[string]any, len(v))
		for key, item := range v {
			m[key] = g.toScript(item)
		}
		return m
	default:
		return v
	}
}

// fromScript converts a script value for storing in a property: object
// handles become refs.
func (g *Game) fromScript(value any) (any, error) {
	switch v := value.(type) {
	case scripting.Handle:
		if v.Type != g.objType {
			return nil, fmt.Errorf("can't store a %s", v.Type.Name)
		}
		return world.Ref{ID: v.Key.(world.ID)}, nil
	case scripting.Function:
		return nil, errors.New("can't store a function")
	case []any:
		list := make([]any, len(v))
		for i, item := range v {
			n, err := g.fromScript(item)
			if err != nil {
				return nil, err
			}
			list[i] = n
		}
		return list, nil
	case map[string]any:
		m := make(map[string]any, len(v))
		for key, item := range v {
			n, err := g.fromScript(item)
			if err != nil {
				return nil, err
			}
			m[key] = n
		}
		return m, nil
	default:
		return v, nil
	}
}

// objectGet reads a property: inherited from o's parents, falling back to
// its field's default, or only o's own when inherit is false.
func (g *Game) objectGet(inherit bool) scripting.Method {
	return func(key any, args scripting.Args) (any, error) {
		o, err := g.object(key)
		if err != nil {
			return nil, err
		}
		name, err := args.String(0)
		if err != nil {
			return nil, err
		}
		if err := structuralName(name); err != nil {
			return nil, err
		}
		f, checked, err := g.checkField(o, name)
		if err != nil {
			return nil, err
		}

		get := o.GetOwn
		if inherit {
			get = o.Get
		}
		value, ok := get(name)
		if !ok && inherit && checked && f.HasDefault {
			value = f.Default
		}

		return g.toScript(value), nil
	}
}

func (g *Game) objectSet(key any, args scripting.Args) (any, error) {
	o, err := g.object(key)
	if err != nil {
		return nil, err
	}
	name, err := args.String(0)
	if err != nil {
		return nil, err
	}
	if err := structuralName(name); err != nil {
		return nil, err
	}
	if args.Len() < 2 {
		return nil, errors.New("argument #2: expected a value (use delete to remove a property)")
	}
	value, err := g.fromScript(args[1])
	if err != nil {
		return nil, fmt.Errorf("argument #2: %w", err)
	}
	if err := g.checkProperty(o, name, value); err != nil {
		return nil, err
	}

	return nil, o.Set(name, value)
}

func (g *Game) objectDelete(key any, args scripting.Args) (any, error) {
	o, err := g.object(key)
	if err != nil {
		return nil, err
	}
	name, err := args.String(0)
	if err != nil {
		return nil, err
	}
	if err := structuralName(name); err != nil {
		return nil, err
	}
	if _, _, err := g.checkField(o, name); err != nil {
		return nil, err
	}

	return nil, o.Delete(name)
}

// structuralFields are the object fields that aren't properties, with
// how scripts read and change each.
var structuralFields = map[string]string{
	"id":       "Read it as o.id; the engine assigns it and it never changes.",
	"key":      "Read it as o.key, and set it with o:set_key(key) or key = ... in world.create.",
	"parent":   "Read it as o.parent, and set it with o:set_parent(parent) or parent = ... in world.create.",
	"location": "Read it as o.location, and set it with o:move_to(place) or location = ... in world.create.",
	"contents": "Read it as o.contents; it lists the objects whose location is o, so move things in with thing:move_to(o).",
	"types":    "Read it as o.types, and change it with o:add_type(name) and o:remove_type(name), or types = { ... } in world.create.",
}

// structuralName errors for a property named after an object field, which
// would hold data the engine never uses as that field.
func structuralName(name string) error {
	if how, ok := structuralFields[name]; ok {
		return fmt.Errorf("%q is one of an object's fields, not a property. %s", name, how)
	}

	return nil
}

func (g *Game) objectProperties(key any, _ scripting.Args) (any, error) {
	o, err := g.object(key)
	if err != nil {
		return nil, err
	}

	return o.Properties(), nil
}

func (g *Game) objectMoveTo(key any, args scripting.Args) (any, error) {
	o, err := g.object(key)
	if err != nil {
		return nil, err
	}
	location, err := g.optionalObjectArg(args, 0)
	if err != nil {
		return nil, err
	}

	return nil, o.MoveTo(location)
}

func (g *Game) objectIsA(key any, args scripting.Args) (any, error) {
	o, err := g.object(key)
	if err != nil {
		return nil, err
	}
	ancestor, err := g.objectArg(args, 0)
	if err != nil {
		return nil, err
	}

	return o.IsA(ancestor), nil
}

func (g *Game) objectIsPlayer(key any, _ scripting.Args) (any, error) {
	o, err := g.object(key)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), saveTimeout)
	defer cancel()

	return g.store.IsCharacter(ctx, o.ID())
}

func (g *Game) objectSetParent(key any, args scripting.Args) (any, error) {
	o, err := g.object(key)
	if err != nil {
		return nil, err
	}
	parent, err := g.optionalObjectArg(args, 0)
	if err != nil {
		return nil, err
	}
	types := o.Types()
	if parent != nil {
		for _, t := range parent.AllTypes() {
			if !slices.Contains(types, t) {
				types = append(types, t)
			}
		}
	}
	if err := g.checkOwnProperties(o, types, "change its parent"); err != nil {
		return nil, err
	}

	return nil, o.SetParent(parent)
}

func (g *Game) objectSetKey(key any, args scripting.Args) (any, error) {
	o, err := g.object(key)
	if err != nil {
		return nil, err
	}

	var k string
	if args.Len() > 0 && args[0] != nil {
		if k, err = args.String(0); err != nil {
			return nil, err
		}
	}

	return nil, o.SetKey(k)
}

func (g *Game) objectSend(key any, args scripting.Args) (any, error) {
	o, err := g.object(key)
	if err != nil {
		return nil, err
	}
	m, _, err := g.outgoing(args)
	if err != nil {
		return nil, err
	}

	for _, p := range g.players {
		if p.character == o {
			p.s.Send(m)
		}
	}

	return nil, nil
}

func (g *Game) objectAddType(key any, args scripting.Args) (any, error) {
	o, err := g.object(key)
	if err != nil {
		return nil, err
	}
	name, err := args.String(0)
	if err != nil {
		return nil, err
	}
	if err := g.schema.Declared(name); err != nil {
		return nil, err
	}
	types := o.AllTypes()
	if !slices.Contains(types, name) {
		types = append([]string{name}, types...)
	}
	if err := g.checkOwnProperties(o, types, "add the type "+name); err != nil {
		return nil, err
	}

	return nil, o.AddType(name)
}

func (g *Game) objectRemoveType(key any, args scripting.Args) (any, error) {
	o, err := g.object(key)
	if err != nil {
		return nil, err
	}
	name, err := args.String(0)
	if err != nil {
		return nil, err
	}
	if !slices.Contains(o.Types(), name) {
		if slices.Contains(o.AllTypes(), name) {
			return nil, fmt.Errorf("%s has the type %s from its parent, not of its own, so it can't remove it. Remove it from the parent, or change the parent.", o, name)
		}
		return nil, nil
	}

	var remaining []string
	for _, t := range o.AllTypes() {
		if t != name || o.Parent() != nil && slices.Contains(o.Parent().AllTypes(), t) {
			remaining = append(remaining, t)
		}
	}
	if err := g.checkOwnProperties(o, remaining, "remove the type "+name); err != nil {
		return nil, err
	}

	return nil, o.RemoveType(name)
}

func (g *Game) objectHasType(key any, args scripting.Args) (any, error) {
	o, err := g.object(key)
	if err != nil {
		return nil, err
	}
	name, err := args.String(0)
	if err != nil {
		return nil, err
	}

	return slices.Contains(o.AllTypes(), name), nil
}
