package install

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"bbuck.dev/dragon-mud/plugin"
)

// scpRx matches git's SSH shorthand, like git@github.com:usera/pluginb.git.
var scpRx = regexp.MustCompile(`^[A-Za-z0-9._-]+@[A-Za-z0-9.-]+:`)

// cloneURL is where git fetches source from: a URL, SSH address or local
// path as it is, and anything else, like github.com/usera/pluginb, over
// HTTPS.
func cloneURL(source string) string {
	switch {
	case strings.Contains(source, "://"), scpRx.MatchString(source), strings.HasPrefix(source, "/"), strings.HasPrefix(source, "."), filepath.IsAbs(source):
		return source
	}

	return "https://" + source
}

// tag is a version tag in a plugin's repository.
type tag struct {
	Name    string
	Version plugin.Version
}

// tags lists the version tags at url, newest first: v1.2.3, or 1.2.3,
// with one to three numbers. Other tags, including pre-releases, are left
// out.
func tags(ctx context.Context, url string) ([]tag, error) {
	out, err := git(ctx, "", "ls-remote", "--tags", "--refs", url)
	if err != nil {
		return nil, err
	}

	var found []tag
	for line := range strings.SplitSeq(out, "\n") {
		_, ref, ok := strings.Cut(strings.TrimSpace(line), "\t")
		if !ok {
			continue
		}
		name := strings.TrimPrefix(ref, "refs/tags/")
		v, err := plugin.ParseVersion(strings.TrimPrefix(name, "v"))
		if err != nil {
			continue
		}
		found = append(found, tag{name, v})
	}
	slices.SortFunc(found, func(a, b tag) int {
		switch {
		case b.Version.Less(a.Version):
			return -1
		case a.Version.Less(b.Version):
			return 1
		}
		return strings.Compare(a.Name, b.Name)
	})

	return found, nil
}

// clone fetches the tag at url into dest, without its git history, and
// returns the commit the tag names.
func clone(ctx context.Context, url, tag, dest string) (string, error) {
	if _, err := git(ctx, "", "-c", "advice.detachedHead=false", "clone", "--quiet", "--depth", "1", "--branch", tag, url, dest); err != nil {
		return "", err
	}
	commit, err := git(ctx, dest, "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(commit), os.RemoveAll(filepath.Join(dest, ".git"))
}

// git runs git with args in dir and returns what it printed.
func git(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr

	if err := cmd.Run(); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return "", errors.New("dragon installs plugins with git, which isn't installed or isn't on your PATH")
		}
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("git %s: %s", args[len(args)-1], msg)
	}

	return stdout.String(), nil
}
