package termlog

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"
)

var (
	on  = true
	off = false
)

// logger returns a logger writing into a buffer, with record times
// removed so lines are predictable.
func logger(color bool, level slog.Level) (*slog.Logger, *bytes.Buffer) {
	var b bytes.Buffer
	h := New(&b, &Options{Level: level, Color: &color})

	return slog.New(noTime{h}), &b
}

type noTime struct{ slog.Handler }

func (n noTime) Handle(ctx context.Context, r slog.Record) error {
	r.Time = time.Time{}
	return n.Handler.Handle(ctx, r)
}

func (n noTime) WithAttrs(attrs []slog.Attr) slog.Handler {
	return noTime{n.Handler.WithAttrs(attrs)}
}

func (n noTime) WithGroup(name string) slog.Handler {
	return noTime{n.Handler.WithGroup(name)}
}

// line is a plain log line: start padded so fields line up, then fields.
func line(start, fields string) string {
	const level = len("ERROR ")
	if pad := level + messageWidth - len(start); pad > 0 {
		start += strings.Repeat(" ", pad)
	}

	return start + "  " + fields + "\n"
}

func TestPlain(t *testing.T) {
	log, b := logger(off, slog.LevelDebug)
	game := log.With(PrefixKey, "game", "plugin", "dragon:chat")

	game.Info("loaded plugin", "commands", 2)
	game.Warn("slow", "took", 3*time.Second)
	log.Error("saving the world failed; will retry", "error", errors.New(`disk "full"`))
	log.Debug("no fields")
	log.WithGroup("req").Info("request", "name", "entity_tooltip", "data", "")

	want := line(` INFO game: loaded plugin`, `plugin=dragon:chat commands=2`) +
		line(` WARN game: slow`, `plugin=dragon:chat took=3s`) +
		line(`ERROR saving the world failed; will retry`, `error="disk \"full\""`) +
		"DEBUG no fields\n" +
		line(` INFO request`, `req.name=entity_tooltip req.data=""`)
	if got := b.String(); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

func TestColor(t *testing.T) {
	log, b := logger(on, slog.LevelInfo)

	log.With("plugin", "game").Error("failed", PrefixKey, "web", "n", 1)

	want := red + "ERROR" + reset + " " + cyan + "web:" + reset + " failed" + strings.Repeat(" ", messageWidth-len("web: failed")) +
		"  " + red + "plugin" + reset + "=game " + red + "n" + reset + "=1\n"
	if got := b.String(); got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}

func TestLevelAndTime(t *testing.T) {
	var b bytes.Buffer
	log := slog.New(New(&b, &Options{Color: &off}))

	log.Debug("hidden")
	log.Info("shown")

	got := b.String()
	if strings.Contains(got, "hidden") {
		t.Errorf("debug logged at the default level: %q", got)
	}
	if !strings.HasPrefix(got, "[") || !strings.HasSuffix(got, "]  INFO shown\n") {
		t.Errorf("got %q, want a [time] then the line", got)
	}
}

func TestNotATerminalHasNoColor(t *testing.T) {
	var b bytes.Buffer
	slog.New(New(&b, nil)).Error("x")

	if strings.Contains(b.String(), "\033") {
		t.Errorf("colored output to a buffer: %q", b.String())
	}
}
