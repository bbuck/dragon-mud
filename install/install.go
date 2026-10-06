package install

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"bbuck.dev/dragon-mud/command"
	"bbuck.dev/dragon-mud/plugin"
)

// Options say where to install and how to ask before installing.
type Options struct {
	// Dir is the game directory.
	Dir string

	// Confirm is shown the plugin about to be installed, and before, the
	// manifest of the version it replaces, if any, so it can show what
	// the plugin may do and ask. Installing goes ahead if it returns true.
	// Nil installs without asking.
	Confirm func(m plugin.Manifest, before *plugin.Manifest) (bool, error)
}

// ErrDeclined is returned when Confirm says not to install.
var ErrDeclined = errors.New("nothing was installed")

// Spec splits what dragon add and update take into a source and a
// version: github.com/usera/pluginb@v1.2.0, or no @ for the newest.
func Spec(s string) (source, version string) {
	if i := strings.LastIndex(s, "@"); i > 0 {
		v := s[i+1:]
		if _, err := plugin.ParseVersion(strings.TrimPrefix(v, "v")); err == nil {
			return s[:i], v
		}
	}

	return s, ""
}

// Add installs the plugin at source into the game's plugins/ directory, at
// version (a tag like v1.2.0), or the newest version tag when version is
// empty, and records it in dragon.lock.
func Add(ctx context.Context, opts Options, source, version string) (Locked, error) {
	lock, err := ReadLock(opts.Dir)
	if err != nil {
		return Locked{}, err
	}
	if p, _, ok := lock.Find(source); ok {
		return Locked{}, fmt.Errorf("%s is installed already, as %s %s. Run dragon update %s to change its version.", source, p.Name, p.Version, p.Name)
	}

	installed, m, staged, err := fetch(ctx, opts.Dir, source, version)
	if err != nil {
		return Locked{}, err
	}
	defer os.RemoveAll(staged)

	if other, _, ok := lock.Find(m.Name); ok {
		return Locked{}, fmt.Errorf("%s's plugin.toml names it %s, but %s from %s has that name already. Plugin names must be unique; remove one of them.", source, m.Name, other.Name, other.Source)
	}
	if isDir(filepath.Join(opts.Dir, "game", plugin.LocalDir, m.Name)) {
		return Locked{}, fmt.Errorf("%s's plugin.toml names it %s, but game/%s/%s is a local plugin with that name. Plugin names must be unique; rename or remove the local one.", source, m.Name, plugin.LocalDir, m.Name)
	}
	if err := confirm(opts, m, nil); err != nil {
		return Locked{}, err
	}

	if err := place(opts.Dir, staged, m.Name); err != nil {
		return Locked{}, err
	}
	lock.Plugins = append(lock.Plugins, installed)

	return installed, WriteLock(opts.Dir, lock)
}

// Update reinstalls an installed plugin, named by its name or source, at
// version, or the newest version tag when version is empty.
func Update(ctx context.Context, opts Options, nameOrSource, version string) (before, after Locked, err error) {
	lock, err := ReadLock(opts.Dir)
	if err != nil {
		return Locked{}, Locked{}, err
	}
	before, i, ok := lock.Find(nameOrSource)
	if !ok {
		return Locked{}, Locked{}, notInstalled(lock, nameOrSource)
	}
	old, err := plugin.ReadManifest(os.DirFS(filepath.Join(opts.Dir, Dir, before.Name)))
	if err != nil {
		return Locked{}, Locked{}, fmt.Errorf("%s/%s: %w", Dir, before.Name, err)
	}

	after, m, staged, err := fetch(ctx, opts.Dir, before.Source, version)
	if err != nil {
		return Locked{}, Locked{}, err
	}
	defer os.RemoveAll(staged)
	if m.Name != before.Name {
		return Locked{}, Locked{}, fmt.Errorf("%s %s renamed the plugin from %s to %s. Remove it with dragon remove %s, then add it again.", before.Source, after.Version, before.Name, m.Name, before.Name)
	}
	if err := confirm(opts, m, &old); err != nil {
		return Locked{}, Locked{}, err
	}

	if err := os.RemoveAll(filepath.Join(opts.Dir, Dir, before.Name)); err != nil {
		return Locked{}, Locked{}, err
	}
	if err := place(opts.Dir, staged, m.Name); err != nil {
		return Locked{}, Locked{}, err
	}
	lock.Plugins[i] = after

	return before, after, WriteLock(opts.Dir, lock)
}

// Remove deletes an installed plugin, named by its name or source, and its
// entry in dragon.lock.
func Remove(dir, nameOrSource string) (Locked, error) {
	lock, err := ReadLock(dir)
	if err != nil {
		return Locked{}, err
	}
	p, i, ok := lock.Find(nameOrSource)
	if !ok {
		return Locked{}, notInstalled(lock, nameOrSource)
	}

	if err := os.RemoveAll(filepath.Join(dir, Dir, p.Name)); err != nil {
		return Locked{}, err
	}
	lock.Plugins = slices.Delete(lock.Plugins, i, i+1)

	return p, WriteLock(dir, lock)
}

// Verify checks that the game's plugins/ directory holds exactly what
// dragon.lock says, file for file, and returns the lock. A plugin whose
// files changed, or one in plugins/ that dragon add didn't install, is an
// error saying what to do.
func Verify(dir string) (Lock, error) {
	lock, err := ReadLock(dir)
	if err != nil {
		return Lock{}, err
	}

	for _, p := range lock.Plugins {
		pluginDir := filepath.Join(dir, Dir, p.Name)
		if !isDir(pluginDir) {
			return Lock{}, fmt.Errorf("%s lists %s, but %s/%s is missing. Run dragon update %s to install it again, or dragon remove %s if the game no longer uses it.",
				LockFile, p.Name, Dir, p.Name, p.Name, p.Name)
		}
		hash, err := Hash(os.DirFS(pluginDir))
		if err != nil {
			return Lock{}, err
		}
		if hash != p.Hash {
			return Lock{}, fmt.Errorf("%s/%s has changed since dragon add installed %s %s. Installed plugins aren't edited in place: run dragon update %s to reinstall it, or move it to game/%s/%s to make it a local plugin you can change (and dragon remove %s).",
				Dir, p.Name, p.Name, p.Version, p.Name, plugin.LocalDir, p.Name, p.Name)
		}
	}

	entries, err := os.ReadDir(filepath.Join(dir, Dir))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return Lock{}, err
	}
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		if _, _, ok := lock.Find(e.Name()); !ok {
			return Lock{}, fmt.Errorf("%s/%s isn't in %s, so dragon add didn't install it. Install plugins with dragon add, or move it to game/%s/%s to make it a local plugin.",
				Dir, e.Name(), LockFile, plugin.LocalDir, e.Name())
		}
	}

	return lock, nil
}

// fetch clones source at version, or its newest version tag, into a
// directory beside plugins/, and returns what it fetched, its manifest
// and the directory, which the caller removes.
func fetch(ctx context.Context, dir, source, version string) (Locked, plugin.Manifest, string, error) {
	url := cloneURL(source)
	found, err := tags(ctx, url)
	if err != nil {
		return Locked{}, plugin.Manifest{}, "", err
	}
	if len(found) == 0 {
		return Locked{}, plugin.Manifest{}, "", fmt.Errorf("%s has no version tags, like v1.0.0, so there's nothing to install. Its author tags a commit with git tag v1.0.0 to release it.", source)
	}

	t := found[0]
	if version != "" {
		want, _ := plugin.ParseVersion(strings.TrimPrefix(version, "v"))
		i := slices.IndexFunc(found, func(t tag) bool { return t.Version == want })
		if i < 0 {
			names := make([]string, len(found))
			for i, t := range found {
				names[i] = t.Name
			}
			return Locked{}, plugin.Manifest{}, "", fmt.Errorf("%s has no version %s.%s Versions: %s.", source, version, command.DidYouMean(version, names), strings.Join(names, ", "))
		}
		t = found[i]
	}

	if err := os.MkdirAll(filepath.Join(dir, Dir), 0o755); err != nil {
		return Locked{}, plugin.Manifest{}, "", err
	}
	parent, err := os.MkdirTemp(filepath.Join(dir, Dir), ".installing-")
	if err != nil {
		return Locked{}, plugin.Manifest{}, "", err
	}
	staged := filepath.Join(parent, "plugin")
	fail := func(err error) (Locked, plugin.Manifest, string, error) {
		os.RemoveAll(parent)
		return Locked{}, plugin.Manifest{}, "", err
	}

	commit, err := clone(ctx, url, t.Name, staged)
	if err != nil {
		return fail(err)
	}
	m, err := plugin.ReadManifest(os.DirFS(staged))
	if err != nil {
		return fail(fmt.Errorf("%s %s: %w", source, t.Name, err))
	}
	hash, err := Hash(os.DirFS(staged))
	if err != nil {
		return fail(err)
	}

	return Locked{Name: m.Name, Source: source, Version: t.Name, Commit: commit, Hash: hash}, m, parent, nil
}

// place moves the plugin fetch staged into plugins/<name>.
func place(dir, staged, name string) error {
	return os.Rename(filepath.Join(staged, "plugin"), filepath.Join(dir, Dir, name))
}

func confirm(opts Options, m plugin.Manifest, before *plugin.Manifest) error {
	if opts.Confirm == nil {
		return nil
	}
	ok, err := opts.Confirm(m, before)
	if err != nil {
		return err
	}
	if !ok {
		return ErrDeclined
	}

	return nil
}

func notInstalled(lock Lock, nameOrSource string) error {
	if len(lock.Plugins) == 0 {
		return fmt.Errorf("%s isn't installed, and no plugins are; dragon add installs them", nameOrSource)
	}

	return fmt.Errorf("%s isn't installed.%s Installed: %s.", nameOrSource, command.DidYouMean(nameOrSource, lock.Names()), strings.Join(lock.Names(), ", "))
}

// NewCapabilities returns the capabilities m declares that before
// doesn't, sorted, for dragon update to point out.
func NewCapabilities(m plugin.Manifest, before *plugin.Manifest) []string {
	had := map[string]bool{}
	if before != nil {
		for _, c := range before.Capabilities {
			had[c] = true
		}
	}

	added := map[string]bool{}
	for _, c := range m.Capabilities {
		if !had[c] {
			added[c] = true
		}
	}

	return slices.Sorted(maps.Keys(added))
}

func isDir(path string) bool {
	info, err := os.Stat(path)

	return err == nil && info.IsDir()
}
