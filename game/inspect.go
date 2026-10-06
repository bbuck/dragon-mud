package game

import (
	"context"
	"log/slog"

	"bbuck.dev/dragon-mud/event"
	"bbuck.dev/dragon-mud/plugin"
	"bbuck.dev/dragon-mud/schema"
)

// Inspection is everything the game's plugins provide, as dragon plugin
// shows it.
type Inspection struct {
	// Plugins are every loaded plugin, in load order. Their scripts can't
	// run, but what they export can be read.
	Plugins []*plugin.Plugin

	Events *event.Registry
	Schema *schema.Registry
}

// Inspect loads the plugins in opts without starting a game. Only Name,
// NewEngine, Plugins and Log are used.
func Inspect(ctx context.Context, opts Options) (*Inspection, error) {
	if opts.Log == nil {
		opts.Log = slog.New(slog.DiscardHandler)
	}

	s, err := fromOptions(opts).load(ctx)
	if err != nil {
		return nil, err
	}
	s.engine.Close()

	return &Inspection{Plugins: s.plugins, Events: s.events, Schema: s.schema}, nil
}
