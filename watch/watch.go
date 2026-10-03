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
	"time"
)

type stamp struct {
	size    int64
	modTime time.Time
}

// Poll checks fsys every interval for files whose base name matches one of
// patterns (see path.Match) being added, removed or modified, and calls
// changed once per check that finds any. It returns when ctx is done.
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

// snapshot records every matching file. Files that vanish mid-walk are
// skipped; the next snapshot sees the result.
func snapshot(fsys fs.FS, patterns []string) map[string]stamp {
	files := make(map[string]stamp)

	fs.WalkDir(fsys, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if !slices.ContainsFunc(patterns, func(pattern string) bool {
			ok, _ := path.Match(pattern, d.Name())
			return ok
		}) {
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
