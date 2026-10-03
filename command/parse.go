package command

import (
	"context"
	"fmt"
	"slices"
	"strings"
)

// Match is the form that won and the values of its slots.
type Match struct {
	Form *Form

	// Args holds each slot's value by name: text for text slots, the
	// resolved value for typed ones.
	Args map[string]any
}

// NoMatch explains why nothing matched.
type NoMatch struct {
	// Reason is the most specific form's resolve failure, such as "You
	// don't see 'bob' here.", or empty when no form fit the input's shape.
	Reason string

	// Usage lists the patterns of commands whose first word matched, when
	// none of their forms fit.
	Usage []string
}

// Parse finds the form input means. Every form is matched against the
// input; typed slots are resolved; forms with a slot that fails to resolve
// are dropped; the most specific form left wins:
//
//  1. the most literal words matched,
//  2. the most typed slots resolved,
//  3. the fewest words typed abbreviated, so "go" picks go over g(oto),
//  4. the plugin loaded last (the game beats plugins beats built-ins),
//  5. the form registered first.
//
// When one form can split the input more than one way, the earliest split
// that resolves wins. Exactly one of the results is non-nil unless a
// resolver returned an error.
func (r *Registry) Parse(ctx context.Context, actor any, input string) (*Match, *NoMatch, error) {
	tokens := Tokenize(input)
	if len(tokens) == 0 {
		return nil, &NoMatch{}, nil
	}

	cache := make(map[resolveKey]resolved)
	single := func(s *Slot) bool { return r.slotType(s).Single }

	var (
		best        *Match
		bestScore   score
		failed      *resolved
		failedScore score
		usage       []string
	)

	for _, form := range r.forms() {
		splits := form.Pattern.match(tokens, single)
		if len(splits) == 0 {
			if first := form.Pattern.Elements[0]; first.Slot == nil && !tokens[0].Quoted &&
				first.matches(tokens[0].Text) && !slices.Contains(usage, form.Pattern.Source) {
				usage = append(usage, form.Pattern.Source)
			}
			continue
		}

		s := score{literals: form.Pattern.Literals(), precedence: form.precedence, order: form.order}
		for _, e := range form.Pattern.Elements {
			if e.Slot != nil && e.Slot.Typed() {
				s.typed++
			}
		}

		for _, split := range splits {
			s.shortened = form.Pattern.shortened(tokens, split)
			args, miss, err := r.resolve(ctx, actor, input, tokens, form.Pattern, split, cache)
			if err != nil {
				return nil, nil, err
			}
			if miss != nil {
				if failed == nil || s.beats(failedScore) {
					failed, failedScore = miss, s
				}
				continue
			}

			if best == nil || s.beats(bestScore) {
				best, bestScore = &Match{Form: form, Args: args}, s
			}
			break
		}
	}

	if best != nil {
		return best, nil, nil
	}
	if failed != nil {
		return nil, &NoMatch{Reason: failed.reason}, nil
	}

	return nil, &NoMatch{Usage: usage}, nil
}

type score struct {
	literals, typed, shortened, precedence, order int
}

func (s score) beats(o score) bool {
	switch {
	case s.literals != o.literals:
		return s.literals > o.literals
	case s.typed != o.typed:
		return s.typed > o.typed
	case s.shortened != o.shortened:
		return s.shortened < o.shortened
	case s.precedence != o.precedence:
		return s.precedence > o.precedence
	default:
		return s.order < o.order
	}
}

type resolveKey struct {
	typ, modifiers, text string
}

type resolved struct {
	value  any
	ok     bool
	reason string
}

// resolve resolves each slot of one split. It returns the args, or the
// first slot that failed.
func (r *Registry) resolve(ctx context.Context, actor any, input string, tokens []Token, p Pattern, split []span, cache map[resolveKey]resolved) (map[string]any, *resolved, error) {
	args := make(map[string]any, len(split))

	i := 0
	for _, e := range p.Elements {
		if e.Slot == nil {
			continue
		}
		text := spanText(input, tokens, split[i])
		i++

		t := r.slotType(e.Slot)
		key := resolveKey{typ: t.Name, modifiers: strings.Join(e.Slot.Modifiers, ","), text: text}
		res, ok := cache[key]
		if !ok {
			mods := make(map[string]bool, len(e.Slot.Modifiers))
			for _, m := range e.Slot.Modifiers {
				mods[m] = true
			}

			value, found, reason, err := t.Resolve(ctx, actor, text, mods)
			if err != nil {
				return nil, nil, fmt.Errorf("slot type %q (from %s): %w", t.Name, t.Plugin, err)
			}
			res = resolved{value: value, ok: found, reason: reason}
			cache[key] = res
		}

		if !res.ok {
			return nil, &res, nil
		}
		args[e.Slot.Name] = res.value
	}

	return args, nil, nil
}
