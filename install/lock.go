// Package install installs plugins into a game: dragon add, update, remove
// and list. Installed plugins are fetched from git at a version tag and
// copied into the game's plugins/ directory, which is committed with the
// game, and dragon.lock records where each came from and a hash of its
// files. The server won't start if an installed plugin's files don't match
// the lock. See docs/plugins.md.
package install

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"
)

// LockFile records the installed plugins, in the game directory.
const LockFile = "dragon.lock"

// Dir holds the installed plugins in the game directory, each in a
// directory named for it.
const Dir = "plugins"

// Locked is one installed plugin.
type Locked struct {
	// Name is the plugin's name, from its manifest, and its directory in
	// plugins/.
	Name string `toml:"name"`

	// Source is where it was installed from, as given to dragon add:
	// github.com/usera/pluginb, or a path or URL git can clone.
	Source string `toml:"source"`

	// Version is the tag installed, like v1.2.0, and Commit the commit
	// it names.
	Version string `toml:"version"`
	Commit  string `toml:"commit"`

	// Hash is a hash of the installed files; see Hash.
	Hash string `toml:"hash"`
}

// Lock is the contents of dragon.lock.
type Lock struct {
	Plugins []Locked `toml:"plugin"`
}

const lockHeader = `# Plugins installed with dragon add, and a hash of each one's files. dragon
# serve refuses to start if a plugin in plugins/ doesn't match. Written by
# dragon add, update and remove; don't edit it by hand.

`

// ReadLock reads the game's dragon.lock, or returns an empty lock if
// there's none.
func ReadLock(dir string) (Lock, error) {
	data, err := os.ReadFile(filepath.Join(dir, LockFile))
	if errors.Is(err, fs.ErrNotExist) {
		return Lock{}, nil
	}
	if err != nil {
		return Lock{}, err
	}

	var lock Lock
	if _, err := toml.Decode(string(data), &lock); err != nil {
		return Lock{}, fmt.Errorf("%s: %w", LockFile, err)
	}

	return lock, nil
}

// WriteLock writes lock to the game's dragon.lock, sorted by name, or
// removes the file when nothing is installed.
func WriteLock(dir string, lock Lock) error {
	path := filepath.Join(dir, LockFile)
	if len(lock.Plugins) == 0 {
		err := os.Remove(path)
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}

	slices.SortFunc(lock.Plugins, func(a, b Locked) int { return strings.Compare(a.Name, b.Name) })
	var buf bytes.Buffer
	buf.WriteString(lockHeader)
	if err := toml.NewEncoder(&buf).Encode(lock); err != nil {
		return err
	}

	return os.WriteFile(path, buf.Bytes(), 0o644)
}

// Find returns the installed plugin with the name or source given, as
// dragon update and remove take either.
func (l Lock) Find(nameOrSource string) (Locked, int, bool) {
	for i, p := range l.Plugins {
		if p.Name == nameOrSource || p.Source == nameOrSource {
			return p, i, true
		}
	}

	return Locked{}, -1, false
}

// Names returns the installed plugins' names.
func (l Lock) Names() []string {
	names := make([]string, len(l.Plugins))
	for i, p := range l.Plugins {
		names[i] = p.Name
	}

	return names
}
