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

// Options say what to install and how to ask before installing.
type Options struct {
	// Dir is the game directory.
	Dir string

	// Dependencies are the game's, from dragon.toml: where each plugin
	// comes from and the versions the game accepts.
	Dependencies map[string]plugin.Constraint

	// Update lists sources whose locked versions are let go, so they move
	// to the newest version every constraint allows. UpdateAll lets go of
	// every one. Otherwise a plugin keeps the version in dragon.lock while
	// every constraint still allows it.
	Update    []string
	UpdateAll bool

	// Confirm is shown what installing will change, so it can show what
	// each new or changed plugin may do and ask. Installing goes ahead if
	// it returns true. Nil installs without asking. It isn't called when
	// nothing is installed or changed.
	Confirm func(changes []Change) (bool, error)
}

// Change is one plugin that Sync installs, changes the version of, or
// removes.
type Change struct {
	Name   string
	Source string

	// Before is what was installed, nil for a new plugin, and After what
	// will be, nil for one removed.
	Before, After *Locked

	// Manifest is After's manifest, and Old Before's.
	Manifest, Old *plugin.Manifest
}

// ErrDeclined is returned when Confirm says not to install.
var ErrDeclined = errors.New("nothing was installed")

// Spec splits what dragon add and update take into a source and a
// version: github.com/johns/rooms@1.8, or no @ for the newest.
func Spec(s string) (source, version string) {
	if i := strings.LastIndex(s, "@"); i > 0 {
		v := s[i+1:]
		if _, err := plugin.ParseConstraint(v); err == nil {
			return s[:i], v
		}
		if _, err := plugin.ParseVersion(strings.TrimPrefix(v, "v")); err == nil {
			return s[:i], v
		}
	}

	return s, ""
}

// Constraint turns a version given to dragon add into what dragon.toml
// records: 1.8 or v1.8 accepts 1.8 up to 2.0 ("^1.8"), and ~1.8 or =1.8.2
// are kept as written.
func Constraint(version string) (string, error) {
	version = strings.TrimPrefix(version, "v")
	if version != "" && !strings.ContainsAny(version[:1], "^~=") {
		version = "^" + version
	}
	if _, err := plugin.ParseConstraint(version); err != nil {
		return "", err
	}

	return version, nil
}

// Newest returns the newest version tag of the plugin at source, as a
// constraint accepting it and later compatible versions, for dragon add
// without a version.
func Newest(ctx context.Context, source string) (string, error) {
	found, err := tags(ctx, cloneURL(source))
	if err != nil {
		return "", err
	}
	if len(found) == 0 {
		return "", noTags(source)
	}

	v := found[0].Version
	if v.Patch == 0 {
		return fmt.Sprintf("^%d.%d", v.Major, v.Minor), nil
	}
	return "^" + v.String(), nil
}

// Sync installs what the game's dependencies resolve to into plugins/,
// with their own dependencies, and records it in dragon.lock: one version
// of each source, the newest every plugin that needs it accepts.
// Plugins nothing needs any more are removed. It returns what changed.
func Sync(ctx context.Context, opts Options) ([]Change, error) {
	lock, err := ReadLock(opts.Dir)
	if err != nil {
		return nil, err
	}

	f := &fetcher{dir: opts.Dir, lock: lock, tags: make(map[string][]tag), staged: make(map[string]staged)}
	defer f.cleanup()

	prefer := make(map[string]string)
	if !opts.UpdateAll {
		for _, p := range lock.Plugins {
			if !slices.Contains(opts.Update, p.Source) {
				prefer[p.Source] = p.Version
			}
		}
	}

	resolved, err := f.resolve(ctx, opts.Dependencies, prefer)
	if err != nil {
		return nil, err
	}

	changes, err := f.plan(resolved)
	if err != nil {
		return nil, err
	}
	if len(changes) == 0 {
		return nil, WriteLock(opts.Dir, Lock{Plugins: lockedOf(resolved)})
	}
	if opts.Confirm != nil {
		ok, err := opts.Confirm(changes)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, ErrDeclined
		}
	}

	if err := f.apply(changes); err != nil {
		return nil, err
	}

	return changes, WriteLock(opts.Dir, Lock{Plugins: lockedOf(resolved)})
}

// resolution is one source's resolved version.
type resolution struct {
	locked   Locked
	manifest plugin.Manifest
}

func lockedOf(resolved map[string]*resolution) []Locked {
	var locked []Locked
	for _, source := range slices.Sorted(maps.Keys(resolved)) {
		locked = append(locked, resolved[source].locked)
	}

	return locked
}

// requirement is one plugin's, or the game's, constraint on a source.
type requirement struct {
	by         string
	constraint plugin.Constraint
}

// maxRounds limits how many times resolve reconsiders its choices before
// giving up on constraints that never settle.
const maxRounds = 32

// resolve picks a version of every source the game needs, directly or
// through other plugins: the version in prefer if every requirement
// allows it, otherwise the newest that does. A choice can change which
// plugins need what, so it repeats until nothing changes.
func (f *fetcher) resolve(ctx context.Context, roots map[string]plugin.Constraint, prefer map[string]string) (map[string]*resolution, error) {
	chosen := make(map[string]*resolution)
	for round := 0; round < maxRounds; round++ {
		requirements := make(map[string][]requirement)
		for source, c := range roots {
			requirements[source] = append(requirements[source], requirement{GameRequires, c})
		}
		for _, source := range slices.Sorted(maps.Keys(chosen)) {
			m := chosen[source].manifest
			for dep, c := range m.Dependencies {
				requirements[dep] = append(requirements[dep], requirement{m.Name, c})
			}
		}

		next := make(map[string]*resolution)
		changed := len(requirements) != len(chosen)
		for _, source := range slices.Sorted(maps.Keys(requirements)) {
			reqs := requirements[source]
			t, err := f.choose(ctx, source, reqs, prefer[source])
			if err != nil {
				return nil, err
			}
			if prev, ok := chosen[source]; ok && prev.locked.Version == t {
				prev.locked.RequiredBy = requiredBy(reqs)
				next[source] = prev
				continue
			}
			changed = true
			r, err := f.fetch(ctx, source, t)
			if err != nil {
				return nil, err
			}
			r.locked.RequiredBy = requiredBy(reqs)
			next[source] = r
		}

		chosen = next
		if !changed {
			return chosen, f.checkNames(chosen)
		}
	}

	return nil, errors.New("the plugins' dependencies keep changing which versions they need and never settle; check their [dependencies] for constraints that contradict each other")
}

func requiredBy(reqs []requirement) []string {
	var by []string
	for _, r := range reqs {
		if !slices.Contains(by, r.by) {
			by = append(by, r.by)
		}
	}
	slices.Sort(by)

	return by
}

// choose picks the tag of source to install for reqs: prefer if every one
// allows it, or the newest tag that every one allows.
func (f *fetcher) choose(ctx context.Context, source string, reqs []requirement, prefer string) (string, error) {
	allows := func(v plugin.Version) bool {
		for _, r := range reqs {
			if !r.constraint.Allows(v) {
				return false
			}
		}
		return true
	}

	if prefer != "" {
		if v, err := plugin.ParseVersion(strings.TrimPrefix(prefer, "v")); err == nil && allows(v) {
			return prefer, nil
		}
	}

	found, err := f.tagsOf(ctx, source)
	if err != nil {
		return "", err
	}
	if len(found) == 0 {
		return "", noTags(source)
	}
	for _, t := range found {
		if allows(t.Version) {
			return t.Name, nil
		}
	}

	wants := make([]string, len(reqs))
	for i, r := range reqs {
		wants[i] = fmt.Sprintf("%s wants %s", r.by, r.constraint)
	}
	names := make([]string, len(found))
	for i, t := range found {
		names[i] = t.Name
	}
	return "", fmt.Errorf("no version of %s is accepted by everything that needs it: %s. Its versions are %s. Change a constraint in dragon.toml, or update the plugin whose constraint is too narrow.",
		source, strings.Join(wants, ", "), strings.Join(names, ", "))
}

// checkNames checks that the resolved plugins' names, which name their
// directories in plugins/, are unique and not those of local plugins.
func (f *fetcher) checkNames(resolved map[string]*resolution) error {
	sources := make(map[string]string)
	for _, source := range slices.Sorted(maps.Keys(resolved)) {
		name := resolved[source].manifest.Name
		if other, ok := sources[name]; ok {
			return fmt.Errorf("%s and %s are both plugins called %s, and plugin names must be unique. Remove one of them from whatever needs it.", other, source, name)
		}
		sources[name] = source
		if isDir(filepath.Join(f.dir, "game", plugin.LocalDir, name)) {
			return fmt.Errorf("%s is a plugin called %s, but game/%s/%s is a local plugin with that name. Plugin names must be unique; rename or remove the local one.", source, name, plugin.LocalDir, name)
		}
	}

	return nil
}

// plan compares resolved with what's installed.
func (f *fetcher) plan(resolved map[string]*resolution) ([]Change, error) {
	var changes []Change
	for _, source := range slices.Sorted(maps.Keys(resolved)) {
		r := resolved[source]
		after := r.locked
		m := r.manifest
		before, installed := f.lock.Source(source)
		if installed && before.Version == after.Version && before.Hash == after.Hash && f.intact(before) {
			continue
		}

		c := Change{Name: after.Name, Source: source, After: &after, Manifest: &m}
		if installed {
			c.Before = &before
			if old, err := plugin.ReadManifest(os.DirFS(filepath.Join(f.dir, Dir, before.Name))); err == nil {
				c.Old = &old
			}
		}
		changes = append(changes, c)
	}
	for _, p := range f.lock.Plugins {
		if _, ok := resolved[p.Source]; !ok {
			before := p
			changes = append(changes, Change{Name: p.Name, Source: p.Source, Before: &before})
		}
	}

	return changes, nil
}

// apply makes the planned changes in plugins/.
func (f *fetcher) apply(changes []Change) error {
	for _, c := range changes {
		if c.Before != nil {
			if err := os.RemoveAll(filepath.Join(f.dir, Dir, c.Before.Name)); err != nil {
				return err
			}
		}
	}
	for _, c := range changes {
		if c.After == nil {
			continue
		}
		s, ok := f.staged[c.Source+"@"+c.After.Version]
		if !ok {
			return fmt.Errorf("%s %s wasn't fetched", c.Source, c.After.Version)
		}
		if err := os.Rename(s.dir, filepath.Join(f.dir, Dir, c.After.Name)); err != nil {
			return err
		}
	}

	return nil
}

// fetcher fetches plugins from git for one Sync, caching what it learns.
type fetcher struct {
	dir  string
	lock Lock

	// tags are each source's version tags, and staged the plugins it has
	// cloned, by source@tag.
	tags   map[string][]tag
	staged map[string]staged

	// parent holds the clones, beside plugins/ so they can be moved in.
	parent string
}

type staged struct {
	dir string
}

func (f *fetcher) cleanup() {
	if f.parent != "" {
		os.RemoveAll(f.parent)
	}
}

func (f *fetcher) tagsOf(ctx context.Context, source string) ([]tag, error) {
	if found, ok := f.tags[source]; ok {
		return found, nil
	}
	found, err := tags(ctx, cloneURL(source))
	if err != nil {
		return nil, err
	}
	f.tags[source] = found

	return found, nil
}

// intact reports whether p's installed files match the lock.
func (f *fetcher) intact(p Locked) bool {
	hash, err := Hash(os.DirFS(filepath.Join(f.dir, Dir, p.Name)))

	return err == nil && hash == p.Hash
}

// fetch returns source at tag: what's installed, when the lock has that
// version and its files are intact, or a fresh clone.
func (f *fetcher) fetch(ctx context.Context, source, t string) (*resolution, error) {
	if p, ok := f.lock.Source(source); ok && p.Version == t && f.intact(p) {
		m, err := plugin.ReadManifest(os.DirFS(filepath.Join(f.dir, Dir, p.Name)))
		if err == nil {
			return &resolution{locked: p, manifest: m}, nil
		}
	}

	if f.parent == "" {
		if err := os.MkdirAll(filepath.Join(f.dir, Dir), 0o755); err != nil {
			return nil, err
		}
		parent, err := os.MkdirTemp(filepath.Join(f.dir, Dir), ".installing-")
		if err != nil {
			return nil, err
		}
		f.parent = parent
	}

	dest := filepath.Join(f.parent, fmt.Sprint(len(f.staged)))
	commit, err := clone(ctx, cloneURL(source), t, dest)
	if err != nil {
		return nil, err
	}
	m, err := plugin.ReadManifest(os.DirFS(dest))
	if err != nil {
		return nil, fmt.Errorf("%s %s: %w", source, t, err)
	}
	hash, err := Hash(os.DirFS(dest))
	if err != nil {
		return nil, err
	}
	f.staged[source+"@"+t] = staged{dir: dest}

	return &resolution{locked: Locked{Name: m.Name, Source: source, Version: t, Commit: commit, Hash: hash}, manifest: m}, nil
}

// Verify checks that the game's plugins/ directory holds exactly what
// dragon.lock says, file for file, and that the lock satisfies the
// game's dependencies, and returns the lock. A plugin whose files
// changed, one in plugins/ that dragon add didn't install, or a
// dependency the lock doesn't satisfy is an error saying what to do.
func Verify(dir string, dependencies map[string]plugin.Constraint) (Lock, error) {
	lock, err := ReadLock(dir)
	if err != nil {
		return Lock{}, err
	}

	for _, p := range lock.Plugins {
		pluginDir := filepath.Join(dir, Dir, p.Name)
		if !isDir(pluginDir) {
			return Lock{}, fmt.Errorf("%s lists %s, but %s/%s is missing. Run dragon update to install it again.",
				LockFile, p.Name, Dir, p.Name)
		}
		hash, err := Hash(os.DirFS(pluginDir))
		if err != nil {
			return Lock{}, err
		}
		if hash != p.Hash {
			return Lock{}, fmt.Errorf("%s/%s has changed since dragon add installed %s %s. Installed plugins aren't edited in place: run dragon update %s to reinstall it, or move it to game/%s/%s to make it a local plugin you can change (and remove it from dragon.toml's [dependencies]).",
				Dir, p.Name, p.Name, p.Version, p.Name, plugin.LocalDir, p.Name)
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

	for _, source := range slices.Sorted(maps.Keys(dependencies)) {
		c := dependencies[source]
		p, ok := lock.Source(source)
		if !ok {
			return Lock{}, fmt.Errorf("dragon.toml depends on %s, which isn't installed. Run dragon update to install it.", source)
		}
		v, err := plugin.ParseVersion(strings.TrimPrefix(p.Version, "v"))
		if err != nil || !c.Allows(v) {
			return Lock{}, fmt.Errorf("dragon.toml accepts %s %s, but %s is installed. Run dragon update %s.", source, c, p.Version, p.Name)
		}
	}
	for _, p := range lock.Plugins {
		if slices.Contains(p.RequiredBy, GameRequires) {
			if _, ok := dependencies[p.Source]; !ok {
				return Lock{}, fmt.Errorf("%s is installed for dragon.toml, which no longer depends on %s. Run dragon update to remove it.", p.Name, p.Source)
			}
		}
	}

	return lock, nil
}

func noTags(source string) error {
	return fmt.Errorf("%s has no version tags, like v1.0.0, so there's nothing to install. Its author tags a commit with git tag v1.0.0 to release it.", source)
}

// NotInstalled is the error for a plugin dragon update or remove was
// given that isn't installed.
func NotInstalled(lock Lock, nameOrSource string) error {
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
