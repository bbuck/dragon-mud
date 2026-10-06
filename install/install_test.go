package install

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
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

func greeter(t *testing.T) string {
	return repo(t, []string{"v0.1.0", "v0.2.0", "not-a-version"}, []map[string]string{
		{"plugin.toml": "name = \"greeter\"\nversion = \"0.1.0\"\n", "init.lua": "return {}\n"},
		{"plugin.toml": "name = \"greeter\"\nversion = \"0.2.0\"\ncapabilities = [\"tasks\"]\n"},
		{"README.md": "unreleased\n"},
	})
}

func TestAddInstallsTheNewestVersion(t *testing.T) {
	src, game := greeter(t), t.TempDir()

	locked, err := Add(context.Background(), Options{Dir: game}, src, "")
	if err != nil {
		t.Fatal(err)
	}
	if locked.Name != "greeter" || locked.Version != "v0.2.0" || len(locked.Commit) != 40 || !strings.HasPrefix(locked.Hash, "sha256:") {
		t.Errorf("locked = %+v", locked)
	}
	if _, err := os.Stat(filepath.Join(game, Dir, "greeter", ".git")); !os.IsNotExist(err) {
		t.Error("the plugin's .git directory was copied")
	}
	if _, err := os.Stat(filepath.Join(game, Dir, "greeter", "README.md")); !os.IsNotExist(err) {
		t.Error("installed files past the newest tag")
	}

	lock, err := Verify(game)
	if err != nil {
		t.Fatal(err)
	}
	if len(lock.Plugins) != 1 || lock.Plugins[0] != locked {
		t.Errorf("lock = %+v", lock)
	}
	entries, _ := os.ReadDir(filepath.Join(game, Dir))
	if len(entries) != 1 {
		t.Errorf("plugins/ has %d entries, want just greeter", len(entries))
	}
}

func TestAddAVersion(t *testing.T) {
	src, game := greeter(t), t.TempDir()

	locked, err := Add(context.Background(), Options{Dir: game}, src, "v0.1.0")
	if err != nil || locked.Version != "v0.1.0" {
		t.Fatalf("locked %+v, %v", locked, err)
	}

	_, err = Add(context.Background(), Options{Dir: game}, src, "")
	if err == nil || !strings.Contains(err.Error(), "is installed already, as greeter v0.1.0. Run dragon update greeter") {
		t.Errorf("adding twice: %v", err)
	}

	_, err = Add(context.Background(), Options{Dir: t.TempDir()}, src, "v0.3.0")
	if err == nil || !strings.Contains(err.Error(), "has no version v0.3.0.") || !strings.Contains(err.Error(), "Versions: v0.2.0, v0.1.0.") {
		t.Errorf("missing version: %v", err)
	}
}

func TestDeclining(t *testing.T) {
	src, game := greeter(t), t.TempDir()

	var shown plugin.Manifest
	_, err := Add(context.Background(), Options{Dir: game, Confirm: func(m plugin.Manifest, before *plugin.Manifest) (bool, error) {
		shown = m
		return false, nil
	}}, src, "")
	if err != ErrDeclined {
		t.Fatalf("err = %v", err)
	}
	if shown.Name != "greeter" || shown.Capabilities[0] != "tasks" {
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

func TestUpdateAndRemove(t *testing.T) {
	src, game := greeter(t), t.TempDir()
	if _, err := Add(context.Background(), Options{Dir: game}, src, "v0.1.0"); err != nil {
		t.Fatal(err)
	}

	var added []string
	before, after, err := Update(context.Background(), Options{Dir: game, Confirm: func(m plugin.Manifest, old *plugin.Manifest) (bool, error) {
		added = NewCapabilities(m, old)
		return true, nil
	}}, "greeter", "")
	if err != nil {
		t.Fatal(err)
	}
	if before.Version != "v0.1.0" || after.Version != "v0.2.0" || len(added) != 1 || added[0] != "tasks" {
		t.Errorf("before %s, after %s, new capabilities %v", before.Version, after.Version, added)
	}
	if _, err := Verify(game); err != nil {
		t.Fatal(err)
	}

	if _, err := Remove(game, src); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(game, Dir, "greeter")); !os.IsNotExist(err) {
		t.Error("remove left plugins/greeter")
	}
	if _, err := Remove(game, "greeter"); err == nil || !strings.Contains(err.Error(), "greeter isn't installed, and no plugins are") {
		t.Errorf("removing twice: %v", err)
	}
}

func TestVerify(t *testing.T) {
	src, game := greeter(t), t.TempDir()
	if _, err := Add(context.Background(), Options{Dir: game}, src, ""); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(game, Dir, "greeter", "init.lua"), []byte("return { changed = true }"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Verify(game); err == nil || !strings.Contains(err.Error(), "plugins/greeter has changed since dragon add installed greeter v0.2.0") {
		t.Errorf("changed files: %v", err)
	}

	os.RemoveAll(filepath.Join(game, Dir, "greeter"))
	if _, err := Verify(game); err == nil || !strings.Contains(err.Error(), "dragon.lock lists greeter, but plugins/greeter is missing") {
		t.Errorf("missing plugin: %v", err)
	}

	os.Remove(filepath.Join(game, LockFile))
	os.MkdirAll(filepath.Join(game, Dir, "stray"), 0o755)
	if _, err := Verify(game); err == nil || !strings.Contains(err.Error(), "plugins/stray isn't in dragon.lock, so dragon add didn't install it") {
		t.Errorf("stray plugin: %v", err)
	}
}

func TestNoTags(t *testing.T) {
	src := repo(t, []string{"draft"}, []map[string]string{{"plugin.toml": "name = \"x\"\n"}})
	_, err := Add(context.Background(), Options{Dir: t.TempDir()}, src, "")
	if err == nil || !strings.Contains(err.Error(), "has no version tags, like v1.0.0") {
		t.Errorf("err = %v", err)
	}
}

func TestSpec(t *testing.T) {
	tests := []struct{ in, source, version string }{
		{"github.com/usera/mapping", "github.com/usera/mapping", ""},
		{"github.com/usera/mapping@v1.2.0", "github.com/usera/mapping", "v1.2.0"},
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
