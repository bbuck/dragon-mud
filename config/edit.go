package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// dependenciesHeader is the line that starts dragon.toml's dependencies.
const dependenciesHeader = "[dependencies]"

var (
	headerRx = regexp.MustCompile(`^\s*\[\s*dependencies\s*\]\s*(#.*)?$`)
	tableRx  = regexp.MustCompile(`^\s*\[`)
	entryRx  = regexp.MustCompile(`^\s*("(?:[^"\\]|\\.)*"|'[^']*'|[A-Za-z0-9_-]+)\s*=`)
)

// SetDependency sets the versions the game accepts of the plugin from
// source in dir's dragon.toml, adding it if it isn't there. Only that line
// changes, so the rest of the file, comments included, stays as written.
func SetDependency(dir, source, constraint string) error {
	return editDependencies(dir, source, fmt.Sprintf("%s = %s", strconv.Quote(source), strconv.Quote(constraint)))
}

// RemoveDependency removes the plugin from source from dir's dragon.toml.
func RemoveDependency(dir, source string) error {
	return editDependencies(dir, source, "")
}

// editDependencies replaces source's line in [dependencies] with line,
// or removes it when line is empty, adding the section if needed.
func editDependencies(dir, source, line string) error {
	path := filepath.Join(dir, FileName)
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")

	start := -1
	for i, l := range lines {
		if headerRx.MatchString(l) {
			start = i
			break
		}
	}
	if start < 0 {
		if line == "" {
			return nil
		}
		lines = append(lines, "", dependenciesHeader, line)
		return write(path, lines)
	}

	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		if tableRx.MatchString(lines[i]) {
			end = i
			break
		}
	}

	last := start
	for i := start + 1; i < end; i++ {
		m := entryRx.FindStringSubmatch(lines[i])
		if m == nil {
			continue
		}
		last = i
		if key(m[1]) != source {
			continue
		}
		if line == "" {
			lines = append(lines[:i], lines[i+1:]...)
		} else {
			lines[i] = line
		}
		return write(path, lines)
	}
	if line == "" {
		return nil
	}

	lines = append(lines[:last+1], append([]string{line}, lines[last+1:]...)...)
	return write(path, lines)
}

// key reads a TOML key as written: quoted or bare.
func key(raw string) string {
	switch {
	case strings.HasPrefix(raw, `"`):
		if s, err := strconv.Unquote(raw); err == nil {
			return s
		}
	case strings.HasPrefix(raw, "'"):
		return strings.Trim(raw, "'")
	}

	return raw
}

func write(path string, lines []string) error {
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644)
}
