package game

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"bbuck.dev/dragon-mud/message"
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
//
//	o:get(name)                property value, inherited from parents
//	o:is_a(other)              true if o is other or inherits from it
//	o:get_own(name)            property value only if o has its own
//	o:set(name, value)         set a property; objects are stored as refs
//	o:delete(name)             remove o's own value
//	o:properties()             names of o's own properties
//	o:move_to(location)        move inside location (nil for nowhere)
//	o:set_parent(parent)       inherit from parent (nil for none)
//	o:set_key(key)             set the unique key (nil to remove)
//	o:send(text)               send text to everyone playing o
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
		},
		Methods: map[string]scripting.Method{
			"get":        g.objectGet((*world.Object).Get),
			"get_own":    g.objectGet((*world.Object).GetOwn),
			"is_a":       g.objectIsA,
			"set":        g.mutating(g.objectSet),
			"delete":     g.mutating(g.objectDelete),
			"properties": g.objectProperties,
			"move_to":    g.mutating(g.objectMoveTo),
			"set_parent": g.mutating(g.objectSetParent),
			"set_key":    g.mutating(g.objectSetKey),
			"send":       g.mutating(g.objectSend),
		},
		String: func(key any) string {
			if o, ok := g.world.Get(key.(world.ID)); ok {
				return "object " + o.String()
			}
			return fmt.Sprintf("object %s (destroyed)", key)
		},
	}
}

// worldModule is the "world" scripting module.
//
//	world.create([options])    a new object; options is a table with any of
//	                           parent, location, key and properties
//	world.get(id)              the object with id, or nil
//	world.keyed(key)           the object with the builder key, or nil
//	world.destroy(o)           destroy o; its contents move to its location
func (g *Game) worldModule() scripting.Module {
	return scripting.Module{
		Name: "world",
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
var createOptions = []string{"parent", "location", "key", "properties"}

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

	if props, ok := opts["properties"]; ok {
		m, ok := props.(map[string]any)
		if !ok {
			return fmt.Errorf("properties: expected map, got %s", scripting.TypeName(props))
		}
		for _, name := range slices.Sorted(maps.Keys(m)) {
			value, err := g.fromScript(m[name])
			if err != nil {
				return fmt.Errorf("properties: %q: %w", name, err)
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

func (g *Game) objectGet(get func(*world.Object, string) (any, bool)) scripting.Method {
	return func(key any, args scripting.Args) (any, error) {
		o, err := g.object(key)
		if err != nil {
			return nil, err
		}
		name, err := args.String(0)
		if err != nil {
			return nil, err
		}

		value, _ := get(o, name)

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
	if args.Len() < 2 {
		return nil, errors.New("argument #2: expected a value (use delete to remove a property)")
	}
	value, err := g.fromScript(args[1])
	if err != nil {
		return nil, fmt.Errorf("argument #2: %w", err)
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

	return nil, o.Delete(name)
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

func (g *Game) objectSetParent(key any, args scripting.Args) (any, error) {
	o, err := g.object(key)
	if err != nil {
		return nil, err
	}
	parent, err := g.optionalObjectArg(args, 0)
	if err != nil {
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
	text, err := args.String(0)
	if err != nil {
		return nil, err
	}

	for _, p := range g.players {
		if p.character == o {
			p.s.Send(message.Text(text))
		}
	}

	return nil, nil
}
