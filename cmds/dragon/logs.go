package main

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"bbuck.dev/dragon-mud/config"
	"bbuck.dev/dragon-mud/termlog"
)

// openLogs returns a handler writing to every [[log]] target in cfg, each
// at its own level and in its own format. File targets are relative to the
// game directory dir. close closes the files.
func openLogs(dir string, targets []config.Log) (h slog.Handler, close func() error, err error) {
	var (
		handlers []slog.Handler
		files    []io.Closer
	)
	close = func() error {
		var errs []error
		for _, f := range files {
			errs = append(errs, f.Close())
		}
		return errors.Join(errs...)
	}

	for _, t := range targets {
		var w io.Writer
		switch t.Target {
		case config.Stderr:
			w = os.Stderr
		case config.Stdout:
			w = os.Stdout
		default:
			path := t.Target
			if !filepath.IsAbs(path) {
				path = filepath.Join(dir, path)
			}
			f, err := openLogFile(path)
			if err != nil {
				close()
				return nil, nil, err
			}
			files = append(files, f)
			w = f
		}

		handlers = append(handlers, logHandler(w, t))
	}

	return slog.NewMultiHandler(handlers...), close, nil
}

func openLogFile(path string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("can't create the directory for log file %s: %w", path, err)
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("can't open log file %s: %w. Check the [[log]] target in %s.", path, err, config.FileName)
	}

	return f, nil
}

func logHandler(w io.Writer, t config.Log) slog.Handler {
	var level slog.Level
	// Validated by config, so this can't fail.
	_ = level.UnmarshalText([]byte(t.Level))

	if t.Format == "json" {
		return slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level})
	}

	// NO_COLOR (no-color.org) beats the config, so one setting turns color
	// off everywhere.
	color := t.Format == "pretty" && os.Getenv("NO_COLOR") == ""

	return termlog.New(w, &termlog.Options{Level: level, Color: &color})
}
