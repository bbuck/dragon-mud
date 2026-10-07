// Package watch notices when files change. It polls rather than using OS
// file events: game directories are small, and polling isn't confused by
// editors that save by renaming.
package watch

import (
	"context"
	"io/fs"
	"maps"
	"path"
	"slices"
	"strings"
	"time"
)

type stamp struct {
	size    int64
	modTime time.Time
}

// Poll checks fsys every interval for files whose base name matches one of
// patterns (see path.Match) being added, removed or modified, and calls
// changed once per check that finds any. A pattern ending in a slash, like
// "web/", matches every file inside a directory of that name, at any depth.
// It returns when ctx is done.
func Poll(ctx context.Context, fsys fs.FS, patterns []string, interval time.Duration, changed func()) error {
	for _, pattern := range patterns {
		if _, err := path.Match(pattern, ""); err != nil {
			return err
		}
	}

	last := snapshot(fsys, patterns)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			current := snapshot(fsys, patterns)
			if !maps.Equal(last, current) {
				last = current
				changed()
			}
		}
	}
}

// matches reports whether the file at name matches pattern: by its base
// name, or for a pattern ending in a slash, by being inside a directory of
// that name.
func matches(pattern, name string) bool {
	if dir, ok := strings.CutSuffix(pattern, "/"); ok {
		parts := strings.Split(name, "/")
		return slices.Contains(parts[:len(parts)-1], dir)
	}
	ok, _ := path.Match(pattern, path.Base(name))

	return ok
}

// snapshot records every matching file. Files that vanish mid-walk are
// skipped; the next snapshot sees the result.
func snapshot(fsys fs.FS, patterns []string) map[string]stamp {
	files := make(map[string]stamp)

	fs.WalkDir(fsys, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if !slices.ContainsFunc(patterns, func(pattern string) bool { return matches(pattern, name) }) {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return nil
		}
		files[name] = stamp{size: info.Size(), modTime: info.ModTime()}

		return nil
	})

	return files
}
