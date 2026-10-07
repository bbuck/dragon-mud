package install

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"bbuck.dev/dragon-mud/plugin"
)

// repo makes a git repository with a commit and tag for each version,
// each a map of files.
func repo(t *testing.T, versions []string, files []map[string]string) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git isn't installed")
	}

	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q")
	for i, version := range versions {
		for name, data := range files[i] {
			if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		run("add", "-A")
		run("commit", "-qm", version)
		run("tag", version)
	}

	return dir
}

// world makes two plugin repositories: rooms, at v1.0.0, v1.1.0 and
// v2.0.0, and mapping, at v0.1.0, which depends on rooms ^1.0 and asks
// for the tasks capability.
func world(t *testing.T) (rooms, mapping string) {
	rooms = repo(t, []string{"v1.0.0", "v1.1.0", "v2.0.0", "not-a-version"}, []map[string]string{
		{"plugin.toml": "name = \"rooms\"\nversion = \"1.0.0\"\n", "init.lua": "return {}\n"},
		{"plugin.toml": "name = \"rooms\"\nversion = \"1.1.0\"\n"},
		{"plugin.toml": "name = \"rooms\"\nversion = \"2.0.0\"\n"},
		{"README.md": "unreleased\n"},
	})
	mapping = repo(t, []string{"v0.1.0"}, []map[string]string{
		{"plugin.toml": "name = \"mapping\"\nversion = \"0.1.0\"\ncapabilities = [\"tasks\"]\n[dependencies]\n\"" + rooms + "\" = \"^1.0\"\n"},
	})

	return rooms, mapping
}

func deps(t *testing.T, pairs ...string) map[string]plugin.Constraint {
	t.Helper()

	d := make(map[string]plugin.Constraint)
	for i := 0; i < len(pairs); i += 2 {
		c, err := plugin.ParseConstraint(pairs[i+1])
		if err != nil {
			t.Fatal(err)
		}
		d[pairs[i]] = c
	}

	return d
}

func sync(t *testing.T, opts Options) []Change {
	t.Helper()

	changes, err := Sync(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}

	return changes
}

func installed(t *testing.T, game string) map[string]Locked {
	t.Helper()

	lock, err := ReadLock(game)
	if err != nil {
		t.Fatal(err)
	}
	m := make(map[string]Locked)
	for _, p := range lock.Plugins {
		m[p.Name] = p
	}

	return m
}

func TestSyncInstallsDependencies(t *testing.T) {
	rooms, mapping := world(t)
	game := t.TempDir()

	changes := sync(t, Options{Dir: game, Dependencies: deps(t, mapping, "^0.1")})
	if len(changes) != 2 {
		t.Fatalf("changes = %+v", changes)
	}

	got := installed(t, game)
	if got["mapping"].Version != "v0.1.0" || !slices.Equal(got["mapping"].RequiredBy, []string{GameRequires}) {
		t.Errorf("mapping = %+v", got["mapping"])
	}
	if got["rooms"].Version != "v1.1.0" || got["rooms"].Source != rooms || !slices.Equal(got["rooms"].RequiredBy, []string{"mapping"}) || len(got["rooms"].Commit) != 40 {
		t.Errorf("rooms = %+v", got["rooms"])
	}
	if _, err := os.Stat(filepath.Join(game, Dir, "rooms", ".git")); !os.IsNotExist(err) {
		t.Error("the plugin's .git directory was copied")
	}
	if _, err := os.Stat(filepath.Join(game, Dir, "rooms", "README.md")); !os.IsNotExist(err) {
		t.Error("installed files past the newest tag")
	}
	entries, _ := os.ReadDir(filepath.Join(game, Dir))
	if len(entries) != 2 {
		t.Errorf("plugins/ has %d entries, want rooms and mapping", len(entries))
	}
	if _, err := Verify(game, deps(t, mapping, "^0.1")); err != nil {
		t.Fatal(err)
	}

	// Nothing changed, so nothing happens.
	if changes := sync(t, Options{Dir: game, Dependencies: deps(t, mapping, "^0.1")}); len(changes) != 0 {
		t.Errorf("a second sync changed %+v", changes)
	}

	// Removing mapping removes rooms, which only mapping needed.
	changes = sync(t, Options{Dir: game})
	if len(changes) != 2 || changes[0].After != nil || changes[1].After != nil {
		t.Errorf("changes = %+v", changes)
	}
	if entries, _ := os.ReadDir(filepath.Join(game, Dir)); len(entries) != 0 {
		t.Errorf("plugins/ still has %d entries", len(entries))
	}
	if _, err := os.Stat(filepath.Join(game, LockFile)); !os.IsNotExist(err) {
		t.Error("dragon.lock is left with nothing installed")
	}
}

// Locked versions stay while every constraint allows them, until updated.
func TestSyncKeepsLockedVersions(t *testing.T) {
	rooms, _ := world(t)
	game := t.TempDir()

	sync(t, Options{Dir: game, Dependencies: deps(t, rooms, "=1.0.0")})
	sync(t, Options{Dir: game, Dependencies: deps(t, rooms, "^1.0")})
	if v := installed(t, game)["rooms"].Version; v != "v1.0.0" {
		t.Errorf("relaxing the constraint moved rooms to %s", v)
	}

	changes := sync(t, Options{Dir: game, Dependencies: deps(t, rooms, "^1.0"), Update: []string{rooms}})
	if v := installed(t, game)["rooms"].Version; v != "v1.1.0" || len(changes) != 1 || changes[0].Before.Version != "v1.0.0" {
		t.Errorf("updating rooms: %s, %+v", v, changes)
	}

	sync(t, Options{Dir: game, Dependencies: deps(t, rooms, "^2.0")})
	if v := installed(t, game)["rooms"].Version; v != "v2.0.0" {
		t.Errorf("a constraint the locked version breaks left rooms at %s", v)
	}
}

func TestSyncConflicts(t *testing.T) {
	rooms, mapping := world(t)

	_, err := Sync(context.Background(), Options{Dir: t.TempDir(), Dependencies: deps(t, rooms, "^2.0", mapping, "^0.1")})
	want := "no version of " + rooms + " is accepted by everything that needs it: dragon.toml wants ^2.0, mapping wants ^1.0. Its versions are v2.0.0, v1.1.0, v1.0.0."
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Errorf("err = %v\nwant it to contain %q", err, want)
	}
}

func TestDeclining(t *testing.T) {
	_, mapping := world(t)
	game := t.TempDir()

	var shown []Change
	_, err := Sync(context.Background(), Options{Dir: game, Dependencies: deps(t, mapping, "^0.1"), Confirm: func(changes []Change) (bool, error) {
		shown = changes
		return false, nil
	}})
	if err != ErrDeclined {
		t.Fatalf("err = %v", err)
	}
	if len(shown) != 2 || shown[0].Manifest == nil {
		t.Errorf("confirm was shown %+v", shown)
	}
	entries, _ := os.ReadDir(filepath.Join(game, Dir))
	if len(entries) != 0 {
		t.Errorf("declining left %d entries in plugins/", len(entries))
	}
	if _, err := os.Stat(filepath.Join(game, LockFile)); !os.IsNotExist(err) {
		t.Error("declining wrote dragon.lock")
	}
}

func TestVerify(t *testing.T) {
	rooms, mapping := world(t)
	game := t.TempDir()
	sync(t, Options{Dir: game, Dependencies: deps(t, mapping, "^0.1")})

	tests := []struct {
		deps map[string]plugin.Constraint
		want string
	}{
		{deps(t, mapping, "^0.1", rooms, "^2.0"), "dragon.toml accepts " + rooms + " ^2.0, but v1.1.0 is installed. Run dragon update rooms."},
		{deps(t, mapping, "^0.1", "example.com/other", "^1.0"), "dragon.toml depends on example.com/other, which isn't installed."},
		{nil, "mapping is installed for dragon.toml, which no longer depends on " + mapping + "."},
	}
	for _, tt := range tests {
		if _, err := Verify(game, tt.deps); err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("err = %v\nwant it to contain %q", err, tt.want)
		}
	}

	if err := os.WriteFile(filepath.Join(game, Dir, "rooms", "init.lua"), []byte("return { changed = true }"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Verify(game, deps(t, mapping, "^0.1")); err == nil || !strings.Contains(err.Error(), "plugins/rooms has changed since dragon add installed rooms v1.1.0") {
		t.Errorf("changed files: %v", err)
	}

	// A changed plugin is reinstalled by the next sync.
	sync(t, Options{Dir: game, Dependencies: deps(t, mapping, "^0.1")})
	if _, err := Verify(game, deps(t, mapping, "^0.1")); err != nil {
		t.Errorf("after reinstalling: %v", err)
	}

	os.RemoveAll(filepath.Join(game, Dir, "rooms"))
	if _, err := Verify(game, deps(t, mapping, "^0.1")); err == nil || !strings.Contains(err.Error(), "dragon.lock lists rooms, but plugins/rooms is missing") {
		t.Errorf("missing plugin: %v", err)
	}

	os.Remove(filepath.Join(game, LockFile))
	os.RemoveAll(filepath.Join(game, Dir, "mapping"))
	os.MkdirAll(filepath.Join(game, Dir, "stray"), 0o755)
	if _, err := Verify(game, nil); err == nil || !strings.Contains(err.Error(), "plugins/stray isn't in dragon.lock, so dragon add didn't install it") {
		t.Errorf("stray plugin: %v", err)
	}
}

func TestNoTags(t *testing.T) {
	src := repo(t, []string{"draft"}, []map[string]string{{"plugin.toml": "name = \"x\"\n"}})
	_, err := Sync(context.Background(), Options{Dir: t.TempDir(), Dependencies: deps(t, src, "^1.0")})
	if err == nil || !strings.Contains(err.Error(), "has no version tags, like v1.0.0") {
		t.Errorf("err = %v", err)
	}
}

func TestNewestAndConstraint(t *testing.T) {
	rooms, _ := world(t)
	if c, err := Newest(context.Background(), rooms); err != nil || c != "^2.0" {
		t.Errorf("Newest = %q, %v", c, err)
	}
	for in, want := range map[string]string{"1.8": "^1.8", "v1.8": "^1.8", "~1.8": "~1.8", "=1.8.2": "=1.8.2"} {
		if got, err := Constraint(in); err != nil || got != want {
			t.Errorf("Constraint(%q) = %q, %v", in, got, err)
		}
	}
}

func TestSpec(t *testing.T) {
	tests := []struct{ in, source, version string }{
		{"github.com/usera/mapping", "github.com/usera/mapping", ""},
		{"github.com/usera/mapping@v1.2.0", "github.com/usera/mapping", "v1.2.0"},
		{"github.com/johns/rooms@1.8", "github.com/johns/rooms", "1.8"},
		{"github.com/johns/rooms@^1.8", "github.com/johns/rooms", "^1.8"},
		{"git@github.com:usera/mapping.git", "git@github.com:usera/mapping.git", ""},
		{"mapping@1.2", "mapping", "1.2"},
	}
	for _, tt := range tests {
		source, version := Spec(tt.in)
		if source != tt.source || version != tt.version {
			t.Errorf("Spec(%q) = %q, %q", tt.in, source, version)
		}
	}
}

func TestCloneURL(t *testing.T) {
	for in, want := range map[string]string{
		"github.com/usera/mapping": "https://github.com/usera/mapping",
		"https://example.com/x":    "https://example.com/x",
		"/srv/plugins/x":           "/srv/plugins/x",
		"./x":                      "./x",
		"git@github.com:usera/x":   "git@github.com:usera/x",
	} {
		if got := cloneURL(in); got != want {
			t.Errorf("cloneURL(%q) = %q, want %q", in, got, want)
		}
	}
}

// Install puts back exactly what the lock pins, and refuses a tag that
// moved.
func TestInstall(t *testing.T) {
	rooms, mapping := world(t)
	game := t.TempDir()
	sync(t, Options{Dir: game, Dependencies: deps(t, mapping, "^0.1")})
	before, _ := ReadLock(game)

	os.RemoveAll(filepath.Join(game, Dir, "rooms"))
	os.WriteFile(filepath.Join(game, Dir, "mapping", "extra.lua"), []byte("x"), 0o644)
	os.MkdirAll(filepath.Join(game, Dir, "stray"), 0o755)

	changes, err := Install(context.Background(), game)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 3 {
		t.Errorf("changes = %+v", changes)
	}
	after, err := Verify(game, deps(t, mapping, "^0.1"))
	if err != nil {
		t.Fatal(err)
	}
	if len(after.Plugins) != len(before.Plugins) || !reflect.DeepEqual(after.Plugins, before.Plugins) {
		t.Errorf("the lock changed: %+v", after.Plugins)
	}
	if changes, err := Install(context.Background(), game); err != nil || len(changes) != 0 {
		t.Errorf("a second install changed %+v, %v", changes, err)
	}

	// The author moves v1.1.0 to another commit.
	cmd := exec.Command("sh", "-c", "echo moved > moved.txt && git add -A && git -c user.email=t@t -c user.name=t commit -qm moved && git tag -f v1.1.0")
	cmd.Dir = rooms
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	os.RemoveAll(filepath.Join(game, Dir, "rooms"))
	_, err = Install(context.Background(), game)
	if err == nil || !strings.Contains(err.Error(), "'s tag v1.1.0 names commit") || !strings.Contains(err.Error(), "its author moved the tag") {
		t.Errorf("moved tag: %v", err)
	}
}
