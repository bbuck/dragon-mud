// Package command turns what players type into command calls. Commands
// have forms, each a pattern such as "say <message> to <target:object:here>".
// The parser matches every form against the input, resolves typed slots,
// drops forms whose slots fail to resolve and runs the most specific form
// left (maximal munch). See docs/design.md §3.
package command

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode"
)

var slotNameRx = regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)

// Pattern is one compiled form pattern: a sequence of literal words and
// slots.
type Pattern struct {
	// Source is the pattern as written, for help and error messages.
	Source string

	Elements []Element
}

// Element is a literal word or a slot.
type Element struct {
	// Literal is the word to match, lowercased. Empty for a slot.
	Literal string

	// Slot is set for slots.
	Slot *Slot
}

// Slot captures part of the input.
type Slot struct {
	Name string

	// Type is the slot type's name. Empty means free text.
	Type string

	// Modifiers are the type's modifiers, such as "here" in
	// <thing:object:here>.
	Modifiers []string
}

// Typed reports whether the slot resolves through a slot type rather than
// taking free text.
func (s *Slot) Typed() bool {
	return s.Type != "" && s.Type != TypeText
}

// ParsePattern compiles a pattern. Optional groups in square brackets
// expand into one pattern with the group and one without, so
// "look [at] <thing:object:here>" gives two patterns.
//
// The syntax is literal words, slots written <name>, <name:type> or
// <name:type:modifier,modifier>, and [optional words or slots]. Groups
// don't nest. A leading punctuation character is its own literal, so
// "'<message>" matches "'hello" and "' hello" alike.
func ParsePattern(source string) ([]Pattern, error) {
	words := strings.Fields(source)
	if len(words) == 0 {
		return nil, errors.New("the pattern is empty. Write what players type, such as \"say <message>\".")
	}
	words = splitLeadingPunct(words)

	// Expand optional groups: each group doubles the variants.
	variants := [][]string{nil}
	for i := 0; i < len(words); i++ {
		word := words[i]
		if !strings.HasPrefix(word, "[") {
			for v := range variants {
				variants[v] = append(variants[v], word)
			}
			continue
		}

		var group []string
		word = strings.TrimPrefix(word, "[")
		for {
			closed := strings.HasSuffix(word, "]")
			word = strings.TrimSuffix(word, "]")
			if strings.ContainsAny(word, "[]") {
				return nil, errors.New("optional groups can't be nested. Write each optional part as its own [group].")
			}
			if word != "" {
				group = append(group, word)
			}
			if closed {
				break
			}
			i++
			if i >= len(words) {
				return nil, errors.New("an optional group opened with [ is never closed with ].")
			}
			word = words[i]
		}
		if len(group) == 0 {
			return nil, errors.New("there's an empty optional group []. Remove it or put words inside.")
		}

		with := make([][]string, len(variants))
		for v, variant := range variants {
			with[v] = append(slices.Clone(variant), group...)
		}
		variants = append(variants, with...)
	}

	var patterns []Pattern
	for _, variant := range variants {
		if len(variant) == 0 {
			continue
		}
		p, err := compile(source, variant)
		if err != nil {
			return nil, err
		}
		patterns = append(patterns, p)
	}
	if len(patterns) == 0 {
		return nil, errors.New("the pattern is only optional parts, so it would match empty input. Make at least one part required.")
	}

	return patterns, nil
}

func compile(source string, words []string) (Pattern, error) {
	p := Pattern{Source: source}
	names := make(map[string]bool)

	for _, word := range words {
		if !strings.HasPrefix(word, "<") {
			if strings.ContainsAny(word, "<>[]\"") {
				return Pattern{}, fmt.Errorf("%q has a stray < > [ ] or \". Slots are written <name>, <name:type> or <name:type:modifier>.", word)
			}
			p.Elements = append(p.Elements, Element{Literal: strings.ToLower(word)})
			continue
		}

		if !strings.HasSuffix(word, ">") {
			return Pattern{}, fmt.Errorf("slot %q isn't closed or has a space inside. Write slots as <name>, <name:type> or <name:type:modifier>, with no spaces.", word)
		}
		parts := strings.Split(strings.TrimSuffix(strings.TrimPrefix(word, "<"), ">"), ":")
		if len(parts) > 3 {
			return Pattern{}, fmt.Errorf("slot %q has too many parts. Write <name:type:modifier,modifier>; separate several modifiers with commas.", word)
		}

		slot := &Slot{Name: parts[0]}
		if !slotNameRx.MatchString(slot.Name) {
			return Pattern{}, fmt.Errorf("slot name %q isn't valid. Use lowercase letters, digits and _, starting with a letter; it becomes args.%s in your function.", slot.Name, strings.ToLower(slot.Name))
		}
		if names[slot.Name] {
			return Pattern{}, fmt.Errorf("the slot name %q is used twice. Each slot needs its own name, since it becomes args.%s.", slot.Name, slot.Name)
		}
		names[slot.Name] = true

		if len(parts) > 1 {
			slot.Type = parts[1]
		}
		if len(parts) > 2 {
			for _, m := range strings.Split(parts[2], ",") {
				if m == "" {
					return Pattern{}, fmt.Errorf("slot <%s> has an empty modifier. Remove the extra comma or colon.", slot.Name)
				}
				slot.Modifiers = append(slot.Modifiers, m)
			}
		}

		p.Elements = append(p.Elements, Element{Slot: slot})
	}

	return p, nil
}

// Literals returns how many literal words the pattern has.
func (p Pattern) Literals() int {
	n := 0
	for _, e := range p.Elements {
		if e.Slot == nil {
			n++
		}
	}

	return n
}

// key identifies the pattern's shape: two patterns with the same key match
// exactly the same inputs, so they can never be told apart.
func (p Pattern) key() string {
	var b strings.Builder
	for _, e := range p.Elements {
		b.WriteByte(' ')
		if e.Slot == nil {
			b.WriteString(e.Literal)
			continue
		}
		mods := slices.Clone(e.Slot.Modifiers)
		slices.Sort(mods)
		typ := e.Slot.Type
		if typ == "" {
			typ = TypeText
		}
		fmt.Fprintf(&b, "<%s:%s>", typ, strings.Join(mods, ","))
	}

	return b.String()
}

// splitLeadingPunct splits a leading punctuation character off the first
// word, as players type "'hello" for "say hello". Patterns and input use the
// same rule, so they always agree.
func splitLeadingPunct(words []string) []string {
	if len(words) == 0 {
		return words
	}

	first := []rune(words[0])
	if len(first) < 2 || !isCommandPunct(first[0]) {
		return words
	}

	return append([]string{string(first[0]), string(first[1:])}, words[1:]...)
}

func isCommandPunct(r rune) bool {
	switch r {
	case '"', '<', '[':
		return false
	}

	return unicode.IsPunct(r) || unicode.IsSymbol(r)
}
