package watch

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPoll(t *testing.T) {
	dir := t.TempDir()
	write := func(name, data string) {
		t.Helper()
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("plugin.lua", "return {}")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	changes := make(chan struct{}, 10)
	go Poll(ctx, os.DirFS(dir), []string{"*.lua"}, 10*time.Millisecond, func() { changes <- struct{}{} })

	expect := func(want bool) {
		t.Helper()
		select {
		case <-changes:
			if !want {
				t.Fatal("unexpected change")
			}
		case <-time.After(100 * time.Millisecond):
			if want {
				t.Fatal("change not noticed")
			}
		}
	}

	expect(false)

	write("notes.txt", "ignored")
	expect(false)

	write("plugin.lua", "return { name = 'game' }")
	expect(true)

	write("sub/commands.lua", "return {}")
	expect(true)

	if err := os.Remove(filepath.Join(dir, "sub", "commands.lua")); err != nil {
		t.Fatal(err)
	}
	expect(true)
}

func TestMatches(t *testing.T) {
	tests := []struct {
		pattern, name string
		want          bool
	}{
		{"*.lua", "lua/commands.lua", true},
		{"*.lua", "web/main.mjs", false},
		{"web/", "web/main.mjs", true},
		{"web/", "plugins/mapping/web/icons/door.png", true},
		{"web/", "lua/web.lua", false},
		{"web/", "web", false},
	}
	for _, tt := range tests {
		if got := matches(tt.pattern, tt.name); got != tt.want {
			t.Errorf("matches(%q, %q) = %v, want %v", tt.pattern, tt.name, got, tt.want)
		}
	}
}
