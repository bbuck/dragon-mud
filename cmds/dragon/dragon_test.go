package main

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"bbuck.dev/dragon-mud/random"
)

func TestDragonGreetsReloadsAndLeaves(t *testing.T) {
	for seed := range uint64(50) {
		var b bytes.Buffer
		d := summon(random.New(seed), &b, false)

		d.greet("The [R]ed Inn", 214)
		d.reloaded()
		d.farewell()

		lines := strings.Split(strings.TrimSuffix(b.String(), "\n"), "\n")
		if len(lines) != 3 {
			t.Fatalf("seed %d: got %d lines: %q", seed, len(lines), b.String())
		}
		greeting, reload, farewell := lines[0], lines[1], lines[2]

		if !strings.HasSuffix(greeting, "All 214 things in the world are where it left them.") {
			t.Errorf("seed %d: greeting = %q", seed, greeting)
		}
		// The game's name appears as written, color codes and all.
		if strings.Contains(greeting, " Inn") && !strings.Contains(greeting, "The [R]ed Inn") {
			t.Errorf("seed %d: game name changed in %q", seed, greeting)
		}
		// The same dragon stays for the whole run.
		name := strings.Join(strings.Fields(reload)[:3], " ")
		if !strings.HasPrefix(farewell, name) {
			t.Errorf("seed %d: %q came, but %q left", seed, reload, farewell)
		}
		if strings.ContainsAny(b.String(), "\033\x00") {
			t.Errorf("seed %d: control characters without color: %q", seed, b.String())
		}
	}
}

func TestDragonWorldSizes(t *testing.T) {
	for objects, want := range map[int]string{
		0: "The world is empty, waiting to be built.",
		1: "The one thing in the world is where it left it.",
	} {
		var b bytes.Buffer
		summon(random.New(1), &b, false).greet("Test", objects)
		if !strings.HasSuffix(strings.TrimSpace(b.String()), want) {
			t.Errorf("%d objects: %q", objects, b.String())
		}
	}
}

func TestDragonColor(t *testing.T) {
	var b bytes.Buffer
	summon(random.New(1), &b, true).greet("Test", 2)

	if !strings.Contains(b.String(), "\033[") || strings.Contains(b.String(), "[x]") {
		t.Errorf("colored greeting = %q", b.String())
	}
}

func TestLegendaryDragonsAreRare(t *testing.T) {
	legendary := 0
	for seed := range uint64(10000) {
		d := summon(random.New(seed), &bytes.Buffer{}, false)
		if !strings.HasPrefix(d.title, "A ") {
			legendary++
		}
	}

	// 1 in 100 over 10,000 summons is about 100.
	if legendary < 50 || legendary > 200 {
		t.Errorf("%d legendary dragons in 10000 summons, want about 100", legendary)
	}
}

func TestNoDragon(t *testing.T) {
	var d *dragon
	d.greet("Test", 1)
	d.reloaded()
	d.farewell()
}

func TestActivityCountsLoggedRecords(t *testing.T) {
	a := newActivity(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelInfo}))
	a.last.Store(time.Now().Add(-time.Hour).UnixNano())
	log := slog.New(a).With("prefix", "game")

	log.Debug("filtered out")
	if a.quiet() < time.Hour {
		t.Error("a record no target wanted counted as activity")
	}

	log.Info("hello")
	if a.quiet() > time.Second {
		t.Errorf("quiet for %v after logging", a.quiet())
	}
}

// syncBuffer is a bytes.Buffer safe to read while the dragon writes.
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) lines() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return strings.Split(strings.TrimSpace(s.b.String()), "\n")
}

func TestDragonSpeaksWhenLogsAreQuiet(t *testing.T) {
	var out syncBuffer
	d := summon(random.New(1), &out, false)
	logs := newActivity(slog.NewTextHandler(io.Discard, nil))
	log := slog.New(logs)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error)
	go func() { done <- d.keepWatch(ctx, logs, 50*time.Millisecond) }()

	// Logging keeps it quiet.
	for range 6 {
		time.Sleep(20 * time.Millisecond)
		log.Info("busy")
	}
	if got := out.lines(); got[0] != "" {
		t.Errorf("the dragon spoke while the logs were busy: %q", got)
	}

	// Silence wakes it, and its own words count as activity.
	time.Sleep(130 * time.Millisecond)
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}

	got := out.lines()
	if len(got) < 1 || len(got) > 3 || !strings.HasSuffix(got[0], "It has kept watch for less than a minute.") {
		t.Errorf("idle lines = %q, want one or two", got)
	}
}

func TestSince(t *testing.T) {
	tests := []struct {
		ago  time.Duration
		want string
	}{
		{10 * time.Second, "less than a minute"},
		{50 * time.Second, "1 minute"},
		{15 * time.Minute, "15 minutes"},
		{time.Hour, "1 hour"},
		{3*time.Hour + 12*time.Minute, "3 hours and 12 minutes"},
		{48 * time.Hour, "2 days"},
		{50*time.Hour + 30*time.Minute, "2 days and 2 hours"},
	}
	for _, tt := range tests {
		if got := since(time.Now().Add(-tt.ago)); got != tt.want {
			t.Errorf("%v ago: got %q, want %q", tt.ago, got, tt.want)
		}
	}
}
