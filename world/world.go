// Package world holds the game's objects in memory. Every object has an id,
// an optional parent it inherits properties from, an optional location (the
// object that contains it) and properties. Rooms, items and players are all
// objects; what they mean is up to plugins. See docs/design.md §1.
//
// A World belongs to the game loop and is not safe for concurrent use. It
// records which objects changed so the game can save them after each event.
package world

import (
	"crypto/rand"
	"errors"
	"fmt"
	"maps"
	"math"
	"regexp"
	"slices"
)

// ID identifies an object. IDs are stable strings, never database row ids,
// so they survive export and import.
type ID string

const (
	idAlphabet = "0123456789abcdefghjkmnpqrstvwxyz"
	idLength   = 8
)

var (
	keyRx      = regexp.MustCompile(`^[a-z][a-z0-9_.:-]*$`)
	propertyRx = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.:-]*$`)
)

// Ref is a property value that refers to another object, such as the room
// an exit leads to. A ref to a destroyed object stays as it was; whoever
// reads it decides what a missing object means.
type Ref struct {
	ID ID
}

// RefKey is the map key that marks a ref in stored JSON, as
// {"$object": "id"}. Property maps can't use it as a key.
const RefKey = "$object"

// ErrDestroyed is returned when changing an object that has been destroyed.
var ErrDestroyed = errors.New("object has been destroyed")

// World is every object in the game.
type World struct {
	objects map[ID]*Object
	keys    map[string]*Object

	changed   map[ID]struct{}
	destroyed map[ID]struct{}
}

// New returns an empty world.
func New() *World {
	return &World{
		objects:   make(map[ID]*Object),
		keys:      make(map[string]*Object),
		changed:   make(map[ID]struct{}),
		destroyed: make(map[ID]struct{}),
	}
}

// Create makes a new object with no parent, location or properties.
func (w *World) Create() *Object {
	o := &Object{w: w, id: w.newID(), props: make(map[string]any)}
	w.objects[o.id] = o
	w.touch(o)

	return o
}

// Get returns the object with id.
func (w *World) Get(id ID) (*Object, bool) {
	o, ok := w.objects[id]

	return o, ok
}

// Keyed returns the object a builder gave the key.
func (w *World) Keyed(key string) (*Object, bool) {
	o, ok := w.keys[key]

	return o, ok
}

// Len returns the number of objects.
func (w *World) Len() int {
	return len(w.objects)
}

// Destroy removes o. Its contents move to o's location and its children
// inherit from o's parent instead, so nothing is left pointing at it.
func (w *World) Destroy(o *Object) {
	if o.isDestroyed() {
		return
	}

	for _, item := range slices.Clone(o.contents) {
		item.moveTo(o.location)
	}
	for _, child := range w.objects {
		if child.parent == o {
			child.parent = o.parent
			w.touch(child)
		}
	}

	o.moveTo(nil)
	if o.key != "" {
		delete(w.keys, o.key)
	}
	delete(w.objects, o.id)
	delete(w.changed, o.id)
	w.destroyed[o.id] = struct{}{}
	o.w = nil
}

// Changes returns what changed since the last call and starts recording
// afresh.
func (w *World) Changes() Changes {
	var c Changes
	for _, id := range slices.Sorted(maps.Keys(w.changed)) {
		c.Saved = append(c.Saved, w.objects[id].Record())
	}
	c.Destroyed = slices.Sorted(maps.Keys(w.destroyed))

	clear(w.changed)
	clear(w.destroyed)

	return c
}

// Unsaved records changes again, such as after saving them failed, so the
// next call to Changes includes them.
func (w *World) Unsaved(c Changes) {
	for _, r := range c.Saved {
		if o, ok := w.objects[r.ID]; ok {
			w.touch(o)
		}
	}
	for _, id := range c.Destroyed {
		w.destroyed[id] = struct{}{}
	}
}

func (w *World) touch(o *Object) {
	w.changed[o.id] = struct{}{}
}

func (w *World) newID() ID {
	for {
		b := make([]byte, idLength)
		rand.Read(b)
		for i := range b {
			b[i] = idAlphabet[int(b[i])%len(idAlphabet)]
		}

		id := ID(b)
		if _, taken := w.objects[id]; !taken {
			if _, gone := w.destroyed[id]; !gone {
				return id
			}
		}
	}
}

// Object is a thing in the world.
type Object struct {
	w *World // nil once destroyed

	id       ID
	key      string
	parent   *Object
	location *Object
	contents []*Object
	props    map[string]any
}

// ID returns the object's id.
func (o *Object) ID() ID {
	return o.id
}

// Key returns the unique key a builder gave the object, if any.
func (o *Object) Key() string {
	return o.key
}

// SetKey gives the object a unique key, such as "tavern", so scripts and
// builders can find it without its id. An empty key removes it. Keys are
// identifiers, not display names; those are ordinary properties.
func (o *Object) SetKey(key string) error {
	if o.isDestroyed() {
		return ErrDestroyed
	}
	if key == o.key {
		return nil
	}
	if key != "" {
		if !keyRx.MatchString(key) {
			return fmt.Errorf("invalid key %q (use lowercase letters, digits, _ . : and -)", key)
		}
		if other, ok := o.w.keys[key]; ok {
			return fmt.Errorf("key %q is already used by %s", key, other.id)
		}
		o.w.keys[key] = o
	}
	if o.key != "" {
		delete(o.w.keys, o.key)
	}

	o.key = key
	o.w.touch(o)

	return nil
}

// Parent returns the object o inherits properties from, or nil.
func (o *Object) Parent() *Object {
	return o.parent
}

// SetParent makes o inherit properties from parent. A nil parent removes
// inheritance. An object can't inherit from itself or its descendants.
func (o *Object) SetParent(parent *Object) error {
	if o.isDestroyed() || (parent != nil && parent.isDestroyed()) {
		return ErrDestroyed
	}
	for p := parent; p != nil; p = p.parent {
		if p == o {
			return fmt.Errorf("%s can't inherit from %s: it would inherit from itself", o.id, parent.id)
		}
	}

	o.parent = parent
	o.w.touch(o)

	return nil
}

// Location returns the object that contains o, or nil.
func (o *Object) Location() *Object {
	return o.location
}

// Contents returns the objects inside o, in the order they arrived.
func (o *Object) Contents() []*Object {
	return slices.Clone(o.contents)
}

// MoveTo puts o inside location. A nil location takes o out of everything.
// An object can't be put inside itself or anything it contains.
func (o *Object) MoveTo(location *Object) error {
	if o.isDestroyed() || (location != nil && location.isDestroyed()) {
		return ErrDestroyed
	}
	for l := location; l != nil; l = l.location {
		if l == o {
			return fmt.Errorf("%s can't move into %s: it would contain itself", o.id, location.id)
		}
	}

	o.moveTo(location)

	return nil
}

func (o *Object) moveTo(location *Object) {
	if o.location == location {
		return
	}
	if o.location != nil {
		o.location.contents = slices.DeleteFunc(o.location.contents, func(c *Object) bool { return c == o })
	}

	o.location = location
	if location != nil {
		location.contents = append(location.contents, o)
	}
	o.w.touch(o)
}

// Get returns the property name, from o or the nearest ancestor that has
// it. Lists and maps are copies; change them with Set.
func (o *Object) Get(name string) (any, bool) {
	for p := o; p != nil; p = p.parent {
		if value, ok := p.props[name]; ok {
			return clone(value), true
		}
	}

	return nil, false
}

// GetOwn returns the property name only if o itself has it.
func (o *Object) GetOwn(name string) (any, bool) {
	value, ok := o.props[name]

	return clone(value), ok
}

// Properties returns the names of o's own properties, sorted.
func (o *Object) Properties() []string {
	return slices.Sorted(maps.Keys(o.props))
}

// Set gives o its own value for the property name, hiding any inherited
// value. Values are nil, bool, integers, floats, strings, refs to objects
// (Ref or *Object), lists ([]any or typed slices) and maps with string keys,
// nested to any depth.
func (o *Object) Set(name string, value any) error {
	if o.isDestroyed() {
		return ErrDestroyed
	}
	if !propertyRx.MatchString(name) {
		return fmt.Errorf("invalid property name %q", name)
	}

	normal, err := Normalize(value)
	if err != nil {
		return fmt.Errorf("property %q: %w", name, err)
	}

	o.props[name] = normal
	o.w.touch(o)

	return nil
}

// Delete removes o's own value for the property name. An inherited value,
// if any, shows through again.
func (o *Object) Delete(name string) error {
	if o.isDestroyed() {
		return ErrDestroyed
	}
	if _, ok := o.props[name]; !ok {
		return nil
	}

	delete(o.props, name)
	o.w.touch(o)

	return nil
}

// String returns the object's key and id, for logs and errors.
func (o *Object) String() string {
	if o.key != "" {
		return o.key + " (" + string(o.id) + ")"
	}

	return string(o.id)
}

func (o *Object) isDestroyed() bool {
	return o.w == nil
}

// Record returns the object as plain data for storage or export.
func (o *Object) Record() Record {
	r := Record{ID: o.id, Key: o.key, Properties: clone(o.props).(map[string]any)}
	if o.parent != nil {
		r.Parent = o.parent.id
	}
	if o.location != nil {
		r.Location = o.location.id
	}

	return r
}

// Record is an object as plain data. Parent and Location are empty when the
// object has none.
type Record struct {
	ID         ID
	Key        string
	Parent     ID
	Location   ID
	Properties map[string]any
}

// Changes is what changed in a world: objects to save in full and ids of
// destroyed objects.
type Changes struct {
	Saved     []Record
	Destroyed []ID
}

// Empty reports whether nothing changed.
func (c Changes) Empty() bool {
	return len(c.Saved) == 0 && len(c.Destroyed) == 0
}

// Load builds a world from records, such as those read from storage. It
// checks that every reference resolves and that there are no cycles.
func Load(records []Record) (*World, error) {
	w := New()

	for _, r := range records {
		if r.ID == "" {
			return nil, errors.New("object with an empty id")
		}
		if _, dup := w.objects[r.ID]; dup {
			return nil, fmt.Errorf("duplicate object %s", r.ID)
		}

		props := make(map[string]any, len(r.Properties))
		for name, value := range r.Properties {
			normal, err := Normalize(value)
			if err != nil {
				return nil, fmt.Errorf("object %s property %q: %w", r.ID, name, err)
			}
			props[name] = normal
		}

		w.objects[r.ID] = &Object{w: w, id: r.ID, props: props}
	}

	for _, r := range records {
		o := w.objects[r.ID]
		ref := func(id ID, what string) (*Object, error) {
			if id == "" {
				return nil, nil
			}
			target, ok := w.objects[id]
			if !ok {
				return nil, fmt.Errorf("object %s: %s %s doesn't exist", r.ID, what, id)
			}
			return target, nil
		}

		var err error
		if o.parent, err = ref(r.Parent, "parent"); err != nil {
			return nil, err
		}
		if o.location, err = ref(r.Location, "location"); err != nil {
			return nil, err
		}
		if o.location != nil {
			o.location.contents = append(o.location.contents, o)
		}

		if r.Key != "" {
			if err := o.SetKey(r.Key); err != nil {
				return nil, fmt.Errorf("object %s: %w", r.ID, err)
			}
		}
	}

	for _, o := range w.objects {
		if err := checkCycle(o, func(o *Object) *Object { return o.parent }, "inherits from"); err != nil {
			return nil, err
		}
		if err := checkCycle(o, func(o *Object) *Object { return o.location }, "contains"); err != nil {
			return nil, err
		}
	}

	clear(w.changed)

	return w, nil
}

// checkCycle reports an error if following next from o returns to o.
func checkCycle(o *Object, next func(*Object) *Object, verb string) error {
	seen := map[*Object]bool{o: true}
	for p := next(o); p != nil; p = next(p) {
		if seen[p] {
			return fmt.Errorf("object %s %s itself", o.id, verb)
		}
		seen[p] = true
	}

	return nil
}

// Normalize converts value to the property value types: nil, bool, int64,
// float64, string, Ref, []any and map[string]any.
func Normalize(value any) (any, error) {
	return normalize(value, 0)
}

const maxDepth = 64

func normalize(value any, depth int) (any, error) {
	if depth > maxDepth {
		return nil, errors.New("value is nested too deeply")
	}

	switch v := value.(type) {
	case nil, bool, int64, string:
		return v, nil
	case Ref:
		if v.ID == "" {
			return nil, errors.New("ref has no object id")
		}
		return v, nil
	case *Object:
		if v == nil {
			return nil, nil
		}
		return Ref{ID: v.id}, nil
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return nil, errors.New("can't store NaN or infinity")
		}
		return v, nil
	case int:
		return int64(v), nil
	case int8:
		return int64(v), nil
	case int16:
		return int64(v), nil
	case int32:
		return int64(v), nil
	case uint8:
		return int64(v), nil
	case uint16:
		return int64(v), nil
	case uint32:
		return int64(v), nil
	case float32:
		return normalize(float64(v), depth)
	case []any:
		list := make([]any, len(v))
		for i, item := range v {
			n, err := normalize(item, depth+1)
			if err != nil {
				return nil, err
			}
			list[i] = n
		}
		return list, nil
	case map[string]any:
		m := make(map[string]any, len(v))
		for key, item := range v {
			if key == RefKey {
				return nil, fmt.Errorf("map key %q is reserved for object references", RefKey)
			}
			n, err := normalize(item, depth+1)
			if err != nil {
				return nil, err
			}
			m[key] = n
		}
		return m, nil
	case []string:
		return normalizeSlice(v, depth)
	case []int:
		return normalizeSlice(v, depth)
	case []int64:
		return normalizeSlice(v, depth)
	case []float64:
		return normalizeSlice(v, depth)
	case []bool:
		return normalizeSlice(v, depth)
	case []map[string]any:
		return normalizeSlice(v, depth)
	default:
		return nil, fmt.Errorf("can't store a %T", value)
	}
}

func normalizeSlice[T any](s []T, depth int) (any, error) {
	list := make([]any, len(s))
	for i, item := range s {
		list[i] = item
	}

	return normalize(list, depth)
}

// clone deep-copies a normalized value so callers can't change stored lists
// and maps behind the world's back.
func clone(value any) any {
	switch v := value.(type) {
	case []any:
		list := make([]any, len(v))
		for i, item := range v {
			list[i] = clone(item)
		}
		return list
	case map[string]any:
		m := make(map[string]any, len(v))
		for key, item := range v {
			m[key] = clone(item)
		}
		return m
	default:
		return v
	}
}
