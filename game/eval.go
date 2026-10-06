package game

import (
	"context"
	"errors"
	"fmt"

	"bbuck.dev/dragon-mud/scripting"
	"bbuck.dev/dragon-mud/world"
)

// evalEvent asks the loop to run source in the game's own plugin.
type evalEvent struct {
	ctx    context.Context
	name   string
	source string
	done   chan evalResult
}

type evalResult struct {
	value any
	err   error
}

// Eval runs source on the game loop, which must be running, in the game's
// own plugin, so it can require the game's modules and the engine's, and
// returns what it returns as plain data: objects become their ids, and
// other handles their descriptions. name identifies source in errors.
// dragon test uses it to set up the world and look at it.
func (g *Game) Eval(ctx context.Context, name, source string) (any, error) {
	e := evalEvent{ctx: ctx, name: name, source: source, done: make(chan evalResult, 1)}
	g.post(e)

	select {
	case r := <-e.done:
		return r.value, r.err
	case <-g.stopped:
		return nil, errors.New("the game stopped")
	}
}

func (g *Game) eval(e evalEvent) {
	if g.scripts.game == nil {
		e.done <- evalResult{err: errors.New("the game has no game/ directory to run code in")}
		return
	}

	ctx, cancel := context.WithTimeout(e.ctx, scriptTimeout)
	defer cancel()

	value, err := g.scripts.game.Eval(ctx, e.name, e.source)
	if err == nil {
		value, err = g.plain(value)
	}
	e.done <- evalResult{value, err}
}

// plain converts a script value to data that's safe to hand outside the
// loop: objects become their ids, other handles their descriptions.
func (g *Game) plain(value any) (any, error) {
	switch v := value.(type) {
	case scripting.Handle:
		if v.Type == g.objType {
			return string(v.Key.(world.ID)), nil
		}
		return v.Describe(), nil
	case scripting.Function:
		return nil, errors.New("can't return a function")
	case []any:
		list := make([]any, len(v))
		for i, item := range v {
			n, err := g.plain(item)
			if err != nil {
				return nil, fmt.Errorf("#%d: %w", i+1, err)
			}
			list[i] = n
		}
		return list, nil
	case map[string]any:
		m := make(map[string]any, len(v))
		for key, item := range v {
			n, err := g.plain(item)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", key, err)
			}
			m[key] = n
		}
		return m, nil
	}

	return value, nil
}
