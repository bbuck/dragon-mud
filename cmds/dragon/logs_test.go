package main

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"bbuck.dev/dragon-mud/config"
)

func TestOpenLogsLevelsAndFormatsPerTarget(t *testing.T) {
	dir := t.TempDir()
	h, closeLogs, err := openLogs(dir, []config.Log{
		{Target: "logs/all.json", Level: "debug", Format: "json"},
		{Target: "logs/warn.log", Level: "warn", Format: "plain"},
	})
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(h)

	log.Debug("details", "n", 1)
	log.Warn("careful", "prefix", "game")
	if err := closeLogs(); err != nil {
		t.Fatal(err)
	}

	read := func(name string) string {
		b, err := os.ReadFile(filepath.Join(dir, "logs", name))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}

	all := read("all.json")
	if !strings.Contains(all, `"msg":"details","n":1`) || !strings.Contains(all, `"msg":"careful"`) {
		t.Errorf("all.json = %s", all)
	}

	warn := read("warn.log")
	if strings.Contains(warn, "details") || !strings.Contains(warn, " WARN game: careful") || strings.Contains(warn, "\033") {
		t.Errorf("warn.log = %q", warn)
	}
}

func TestOpenLogsBadFile(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "taken"), nil, 0o644)

	_, _, err := openLogs(dir, []config.Log{{Target: "taken/game.log", Level: "info", Format: "plain"}})
	if err == nil || !strings.Contains(err.Error(), "can't create the directory for log file") {
		t.Errorf("error = %v", err)
	}
}
