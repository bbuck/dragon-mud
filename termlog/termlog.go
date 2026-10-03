// Package termlog is a log/slog handler for people reading logs in a
// terminal, in the style of the old engine's logs:
//
//	[2026-10-02 18:33:36]  INFO game: loaded plugin  plugin=dragon:chat commands=2
//
// The level is colored, the "prefix" attribute (which part of the engine is
// talking) leads the message, and attribute keys take the level's color.
// Color is only used when the output is a terminal and NO_COLOR isn't set.
package termlog

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/mattn/go-isatty"
)

// PrefixKey is the attribute shown before the message instead of with the
// other attributes: log.With(termlog.PrefixKey, "web").
const PrefixKey = "prefix"

// timeFormat is how each line's time is shown.
const timeFormat = "2006-01-02 15:04:05"

// messageWidth pads the prefix and message so attributes line up.
const messageWidth = 40

const (
	reset = "\033[0m"
	dim   = "\033[2m"
	red   = "\033[31m"
	green = "\033[32m"
	amber = "\033[33m"
	blue  = "\033[34m"
	cyan  = "\033[36m"
)

// Options configures a Handler.
type Options struct {
	// Level is the lowest level logged. Nil logs Info and above.
	Level slog.Leveler

	// Color forces color on or off. Nil decides from the output.
	Color *bool
}

// Handler writes log records as colored lines.
type Handler struct {
	out   *output
	level slog.Leveler
	color bool

	// prefix is the PrefixKey attribute, attrs the others given to With,
	// and groups the open groups.
	prefix string
	attrs  []groupedAttr
	groups []string
}

// groupedAttr is an attribute and the groups open when it was added.
type groupedAttr struct {
	groups []string
	attr   slog.Attr
}

// output is shared by a handler and every handler derived from it, so
// lines from different loggers never interleave.
type output struct {
	mu sync.Mutex
	w  io.Writer
}

// New returns a handler writing to w.
func New(w io.Writer, opts *Options) *Handler {
	if opts == nil {
		opts = &Options{}
	}

	h := &Handler{out: &output{w: w}, level: opts.Level}
	if h.level == nil {
		h.level = slog.LevelInfo
	}
	if opts.Color != nil {
		h.color = *opts.Color
	} else {
		h.color = UseColor(w)
	}

	return h
}

// UseColor reports whether color should be written to w: it's a terminal,
// and NO_COLOR (no-color.org) isn't set.
func UseColor(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok || os.Getenv("NO_COLOR") != "" {
		return false
	}

	return isatty.IsTerminal(f.Fd()) || isatty.IsCygwinTerminal(f.Fd())
}

// Enabled reports whether level is logged.
func (h *Handler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level.Level()
}

// WithAttrs returns a handler that adds attrs to every record.
func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	h2 := *h
	h2.attrs = slices.Clip(h.attrs)
	for _, a := range attrs {
		if a.Key == PrefixKey && len(h.groups) == 0 {
			h2.prefix = a.Value.String()
			continue
		}
		h2.attrs = append(h2.attrs, groupedAttr{groups: h.groups, attr: a})
	}

	return &h2
}

// WithGroup returns a handler that puts later attributes in the group name.
func (h *Handler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}

	h2 := *h
	h2.groups = append(slices.Clip(h.groups), name)

	return &h2
}

// Handle writes one record as a line.
func (h *Handler) Handle(_ context.Context, r slog.Record) error {
	var b strings.Builder

	if !r.Time.IsZero() {
		h.paint(&b, dim, "["+r.Time.Format(timeFormat)+"]")
		b.WriteByte(' ')
	}

	h.paint(&b, h.levelColor(r.Level), fmt.Sprintf("%5s", levelName(r.Level)))
	b.WriteByte(' ')

	prefix := h.prefix
	var fields strings.Builder
	keyColor := h.levelColor(r.Level)
	for _, ga := range h.attrs {
		h.writeAttr(&fields, ga.groups, ga.attr, keyColor)
	}
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == PrefixKey && len(h.groups) == 0 {
			prefix = a.Value.String()
			return true
		}
		h.writeAttr(&fields, h.groups, a, keyColor)
		return true
	})

	if prefix != "" {
		h.paint(&b, cyan, prefix+":")
		b.WriteByte(' ')
	}

	b.WriteString(r.Message)
	if fields.Len() > 0 {
		width := len(r.Message)
		if prefix != "" {
			width += len(prefix) + 2
		}
		if pad := messageWidth - width; pad > 0 {
			b.WriteString(strings.Repeat(" ", pad))
		}
		b.WriteString(" ")
		b.WriteString(fields.String())
	}
	b.WriteByte('\n')

	h.out.mu.Lock()
	defer h.out.mu.Unlock()
	_, err := io.WriteString(h.out.w, b.String())

	return err
}

// writeAttr writes a as " key=value", flattening groups into dotted keys.
func (h *Handler) writeAttr(b *strings.Builder, groups []string, a slog.Attr, keyColor string) {
	a.Value = a.Value.Resolve()
	if a.Equal(slog.Attr{}) {
		return
	}

	if a.Value.Kind() == slog.KindGroup {
		attrs := a.Value.Group()
		if len(attrs) == 0 {
			return
		}
		if a.Key != "" {
			groups = append(slices.Clip(groups), a.Key)
		}
		for _, ga := range attrs {
			h.writeAttr(b, groups, ga, keyColor)
		}
		return
	}

	key := a.Key
	if len(groups) > 0 {
		key = strings.Join(groups, ".") + "." + key
	}

	b.WriteByte(' ')
	h.paint(b, keyColor, key)
	b.WriteByte('=')
	b.WriteString(quote(value(a.Value)))
}

func value(v slog.Value) string {
	switch v.Kind() {
	case slog.KindTime:
		return v.Time().Format(time.RFC3339)
	case slog.KindAny:
		if err, ok := v.Any().(error); ok {
			return err.Error()
		}
	}

	return v.String()
}

// quote quotes s if it's empty or has spaces, quotes or control
// characters, so every value reads as one token.
func quote(s string) string {
	if s == "" {
		return `""`
	}
	for _, r := range s {
		if unicode.IsSpace(r) || r == '"' || r == '=' || !unicode.IsPrint(r) {
			return strconv.Quote(s)
		}
	}

	return s
}

func (h *Handler) paint(b *strings.Builder, color, text string) {
	if !h.color {
		b.WriteString(text)
		return
	}

	b.WriteString(color)
	b.WriteString(text)
	b.WriteString(reset)
}

func (h *Handler) levelColor(level slog.Level) string {
	switch {
	case level >= slog.LevelError:
		return red
	case level >= slog.LevelWarn:
		return amber
	case level >= slog.LevelInfo:
		return green
	default:
		return blue
	}
}

func levelName(level slog.Level) string {
	switch {
	case level >= slog.LevelError:
		return "ERROR"
	case level >= slog.LevelWarn:
		return "WARN"
	case level >= slog.LevelInfo:
		return "INFO"
	default:
		return "DEBUG"
	}
}
