package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"maps"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"text/tabwriter"

	"bbuck.dev/dragon-mud/config"
	"bbuck.dev/dragon-mud/install"
	"bbuck.dev/dragon-mud/plugin"
)

// installFlags parses the flags dragon add, update and remove share, and
// returns the one argument they take.
func installFlags(name string, args []string) (dir string, yes bool, arg string, err error) {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	dirFlag := flags.String("dir", ".", "the game directory")
	yesFlag := flags.Bool("y", false, "install without asking")

	// Allow the argument before or after flags.
	if len(args) > 0 && args[0] != "" && args[0][0] != '-' {
		arg, args = args[0], args[1:]
	}
	if err := flags.Parse(args); err != nil {
		return "", false, "", err
	}
	if arg == "" {
		arg = flags.Arg(0)
	}

	return *dirFlag, *yesFlag, arg, nil
}

// runAdd installs a plugin, after showing what it asks for.
func runAdd(args []string, in io.Reader, out io.Writer) error {
	dir, yes, spec, err := installFlags("add", args)
	if err != nil {
		return err
	}
	if spec == "" {
		return errors.New("usage: dragon add <source>[@version], like dragon add github.com/usera/mapping@v1.2.0")
	}
	if _, err := config.Load(dir); err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	source, version := install.Spec(spec)
	locked, err := install.Add(ctx, install.Options{Dir: dir, Confirm: confirmer(in, out, yes, source)}, source, version)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "Installed %s %s in %s/%s.\n", locked.Name, locked.Version, install.Dir, locked.Name)

	return reportUnmet(dir, out)
}

// runUpdate reinstalls a plugin at a new version.
func runUpdate(args []string, in io.Reader, out io.Writer) error {
	dir, yes, spec, err := installFlags("update", args)
	if err != nil {
		return err
	}
	if spec == "" {
		return errors.New("usage: dragon update <plugin>[@version]")
	}
	if _, err := config.Load(dir); err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	name, version := install.Spec(spec)
	before, after, err := install.Update(ctx, install.Options{Dir: dir, Confirm: confirmer(in, out, yes, name)}, name, version)
	if err != nil {
		return err
	}
	if before.Version == after.Version && before.Hash == after.Hash {
		fmt.Fprintf(out, "%s is at %s already.\n", after.Name, after.Version)
	} else {
		fmt.Fprintf(out, "Updated %s from %s to %s.\n", after.Name, before.Version, after.Version)
	}

	return reportUnmet(dir, out)
}

// runRemove deletes an installed plugin.
func runRemove(args []string, out io.Writer) error {
	dir, _, name, err := installFlags("remove", args)
	if err != nil {
		return err
	}
	if name == "" {
		return errors.New("usage: dragon remove <plugin>")
	}
	if _, err := config.Load(dir); err != nil {
		return err
	}

	removed, err := install.Remove(dir, name)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "Removed %s %s.\n", removed.Name, removed.Version)

	return reportUnmet(dir, out)
}

// runList lists the installed plugins.
func runList(args []string, out io.Writer) error {
	flags := flag.NewFlagSet("list", flag.ContinueOnError)
	dir := flags.String("dir", ".", "the game directory")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if _, err := config.Load(*dir); err != nil {
		return err
	}

	lock, err := install.Verify(*dir)
	if err != nil {
		return err
	}
	if len(lock.Plugins) == 0 {
		fmt.Fprintln(out, "No plugins are installed. Install one with dragon add <source>, like dragon add github.com/usera/mapping.")
		return nil
	}

	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "Plugin\tVersion\tSource\tCapabilities")
	for _, p := range lock.Plugins {
		caps := "none"
		if m, err := plugin.ReadManifest(os.DirFS(filepath.Join(*dir, install.Dir, p.Name))); err == nil && len(m.Capabilities) > 0 {
			caps = strings.Join(m.Capabilities, ", ")
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", p.Name, p.Version, p.Source, caps)
	}

	return w.Flush()
}

// confirmer shows what a plugin asks for and asks whether to install it,
// unless yes is set. For an update, it only asks when the new version asks
// for capabilities the old one didn't.
func confirmer(in io.Reader, out io.Writer, yes bool, source string) func(plugin.Manifest, *plugin.Manifest) (bool, error) {
	return func(m plugin.Manifest, before *plugin.Manifest) (bool, error) {
		caps := m.Capabilities
		if before != nil {
			caps = install.NewCapabilities(m, before)
			if len(caps) == 0 {
				return true, nil
			}
			fmt.Fprintf(out, "%s %s asks for capabilities the installed version doesn't:\n", m.Name, m.Version)
		} else {
			fmt.Fprintf(out, "%s %s, from %s, ", m.Name, m.Version, source)
			if len(caps) == 0 {
				fmt.Fprintln(out, "asks for no capabilities: it uses only game features.")
			} else {
				fmt.Fprintln(out, "asks for these capabilities:")
			}
		}
		w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
		for _, c := range caps {
			fmt.Fprintf(w, "  %s\t%s\n", c, plugin.CapabilityDescs[c])
		}
		w.Flush()

		if yes {
			return true, nil
		}
		fmt.Fprint(out, "Install it? [y/N] ")
		answer, err := bufio.NewReader(in).ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return false, err
		}
		answer = strings.ToLower(strings.TrimSpace(answer))

		return answer == "y" || answer == "yes", nil
	}
}

// reportUnmet points out APIs the game's plugins depend on that no plugin
// it loads provides, or provides at a version they don't accept, which
// dragon serve would refuse to start with. Installing doesn't fetch
// dependencies: a dependency names an API, not where to get a plugin
// providing it.
func reportUnmet(dir string, out io.Writer) error {
	cfg, err := config.Load(dir)
	if err != nil {
		return err
	}
	sources, err := pluginSources(dir, cfg.Builtins)
	if err != nil {
		return err
	}

	type provider struct {
		id      string
		version plugin.Version
	}
	manifests := make(map[string]plugin.Manifest)
	providers := make(map[string]provider)
	for _, src := range sources {
		if src.Game {
			continue
		}
		m, err := plugin.ReadManifest(src.Files)
		if err != nil {
			continue
		}
		id := m.Name
		if src.Builtin {
			id = plugin.BuiltinPrefix + m.Name
		}
		manifests[id] = m
		for api, v := range m.Provides {
			providers[api] = provider{id, v}
		}
	}

	var problems []string
	for _, id := range slices.Sorted(maps.Keys(manifests)) {
		m := manifests[id]
		for _, api := range slices.Sorted(maps.Keys(m.Depends)) {
			dep := m.Depends[api]
			p, ok := providers[api]
			switch {
			case !ok && !dep.Optional:
				problems = append(problems, fmt.Sprintf("%s depends on the %s API (%s), but no plugin the game loads provides it.", id, api, dep.Version))
			case ok && !dep.Version.Allows(p.version):
				problems = append(problems, fmt.Sprintf("%s depends on the %s API %s, but %s provides %s.", id, api, dep.Version, p.id, p.version))
			}
		}
	}
	if len(problems) > 0 {
		fmt.Fprintln(out)
		fmt.Fprintln(out, "The game won't start until these are fixed:")
		for _, p := range problems {
			fmt.Fprintln(out, "  "+p)
		}
		fmt.Fprintln(out, "Install a plugin that provides each API with dragon add, or change versions with dragon update.")
	}

	return nil
}
