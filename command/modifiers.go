package command

import (
	"fmt"
	"slices"
	"strings"
)

// Modifiers are a slot's modifiers as written in its pattern: requirements
// separated by commas, all of which must hold, each one or more
// alternatives separated by |. <thing:object:here|held,online> is
// Modifiers{{"here", "held"}, {"online"}}: here or held, and online.
type Modifiers [][]string

// String writes m as a pattern does, such as "here|held,online".
func (m Modifiers) String() string {
	reqs := make([]string, len(m))
	for i, alts := range m {
		reqs[i] = strings.Join(alts, "|")
	}

	return strings.Join(reqs, ",")
}

// Names returns every modifier m names, in order, each once.
func (m Modifiers) Names() []string {
	var names []string
	for _, alts := range m {
		for _, name := range alts {
			if !slices.Contains(names, name) {
				names = append(names, name)
			}
		}
	}

	return names
}

// Has reports whether m names the modifier name anywhere.
func (m Modifiers) Has(name string) bool {
	return slices.Contains(m.Names(), name)
}

// Only reports whether m is exactly the one modifier name.
func (m Modifiers) Only(name string) bool {
	return len(m) == 1 && len(m[0]) == 1 && m[0][0] == name
}

// key is m in a canonical order, so modifiers that mean the same thing
// compare equal: "held|here" and "here|held" do.
func (m Modifiers) key() string {
	reqs := make([]string, len(m))
	for i, alts := range m {
		sorted := slices.Clone(alts)
		slices.Sort(sorted)
		reqs[i] = strings.Join(sorted, "|")
	}
	slices.Sort(reqs)

	return strings.Join(reqs, ",")
}

// parseModifiers reads the modifiers part of a slot, such as
// "here|held,online".
func parseModifiers(slot, source string) (Modifiers, error) {
	var m Modifiers
	for _, req := range strings.Split(source, ",") {
		var alts []string
		for _, name := range strings.Split(req, "|") {
			if name == "" {
				return nil, fmt.Errorf("slot <%s> has an empty modifier in %q. Remove the extra comma, | or colon.", slot, source)
			}
			alts = append(alts, name)
		}
		m = append(m, alts)
	}

	return m, nil
}
