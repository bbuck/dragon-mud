package main

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"
)

// activity is a log handler that notes when something was last logged, so
// the dragon can speak up when the logs go quiet. Records no target wants,
// such as debug lines when every target logs info and above, don't count.
type activity struct {
	slog.Handler

	// last is when something was last logged, in Unix nanoseconds. It's
	// shared by every handler derived from this one.
	last *atomic.Int64
}

func newActivity(h slog.Handler) activity {
	a := activity{Handler: h, last: new(atomic.Int64)}
	a.touch()

	return a
}

// Handle notes the record, then passes it on.
func (a activity) Handle(ctx context.Context, r slog.Record) error {
	a.touch()

	return a.Handler.Handle(ctx, r)
}

func (a activity) WithAttrs(attrs []slog.Attr) slog.Handler {
	return activity{Handler: a.Handler.WithAttrs(attrs), last: a.last}
}

func (a activity) WithGroup(name string) slog.Handler {
	return activity{Handler: a.Handler.WithGroup(name), last: a.last}
}

// touch counts now as activity.
func (a activity) touch() {
	a.last.Store(time.Now().UnixNano())
}

// quiet returns how long it's been since anything was logged.
func (a activity) quiet() time.Duration {
	return time.Since(time.Unix(0, a.last.Load()))
}
