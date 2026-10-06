package game

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"bbuck.dev/dragon-mud/command"
	"bbuck.dev/dragon-mud/schema"
	"bbuck.dev/dragon-mud/world"
)

// loadSchema builds s's schema from the types and extensions its plugins
// declared. Extensions of a type nothing declares are logged, like
// handlers for an undeclared event: a misspelling, or a type from a
// plugin the game doesn't load.
func (g *Game) loadSchema(s *scripts) error {
	registry, unmatched, err := schema.New(s.types, s.extensions)
	if err != nil {
		return err
	}
	s.schema = registry
	for _, ext := range unmatched {
		g.log.Warn(ext.Where+" adds fields to "+ext.Type+", but no plugin declares that type, so they're never used; it may be misspelled, or from a plugin the game doesn't load."+command.DidYouMean(ext.Type, registry.Names()),
			"type", ext.Type)
	}

	// Objects whose types a plugin no longer declares take any property
	// until the type comes back or a builder removes it.
	missing := make(map[string]int)
	g.world.Each(func(o *world.Object) {
		for _, t := range o.Types() {
			if _, ok := registry.Type(t); !ok {
				missing[t]++
			}
		}
	})
	for _, t := range slices.Sorted(maps.Keys(missing)) {
		g.log.Warn(fmt.Sprintf("%d objects have the type %s, which no plugin declares, so their properties aren't checked. Load the plugin that declares it, or remove the type from them.", missing[t], t),
			"type", t)
	}

	return nil
}

// checkProperty returns an error unless o may hold value in the property
// name, given its types.
func (g *Game) checkProperty(o *world.Object, name string, value any) error {
	types := o.AllTypes()
	if !g.schema.Checked(types) {
		return nil
	}
	normal, err := world.Normalize(value)
	if err != nil {
		return err
	}

	return g.schema.Check(types, name, normal)
}

// checkField returns the field name of o, and whether o's properties are
// checked at all; an error if o is checked and has no such field.
func (g *Game) checkField(o *world.Object, name string) (schema.Field, bool, error) {
	types := o.AllTypes()
	if !g.schema.Checked(types) {
		return schema.Field{}, false, nil
	}
	f, err := g.schema.Field(types, name)

	return f, true, err
}

// checkOwnProperties returns an error unless every property o has itself
// fits types, as o would have them after a change to its types or parent.
// what says what the change is, like "add the type items:item".
func (g *Game) checkOwnProperties(o *world.Object, types []string, what string) error {
	if !g.schema.Checked(types) {
		return nil
	}

	var problems []string
	for _, name := range o.Properties() {
		value, _ := o.GetOwn(name)
		if err := g.schema.Check(types, name, value); err != nil {
			problems = append(problems, err.Error())
		}
	}
	if len(problems) > 0 {
		return fmt.Errorf("can't %s: %s Change or delete those properties first.", what, strings.Join(problems, " "))
	}

	return nil
}
