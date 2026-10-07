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

// runAdd adds a plugin to dragon.toml's dependencies and installs it,
// with what it depends on, after showing what each asks for.
func runAdd(args []string, in io.Reader, out io.Writer) error {
	dir, yes, spec, err := installFlags("add", args)
	if err != nil {
		return err
	}
	if spec == "" {
		return errors.New("usage: dragon add <source>[@version], like dragon add github.com/johns/rooms@1.8")
	}
	cfg, err := config.Load(dir)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	source, version := install.Spec(spec)
	constraint := ""
	if version != "" {
		if constraint, err = install.Constraint(version); err != nil {
			return err
		}
	} else if constraint, err = install.Newest(ctx, source); err != nil {
		return err
	}

	// Install with the new constraint, and put dragon.toml back as it was
	// if that fails.
	old, had := cfg.Dependencies[source]
	if err := config.SetDependency(dir, source, constraint); err != nil {
		return err
	}
	if err := syncPlugins(ctx, dir, install.Options{Update: []string{source}}, in, out, yes); err != nil {
		if had {
			config.SetDependency(dir, source, old)
		} else {
			config.RemoveDependency(dir, source)
		}
		return err
	}

	return reportUnmet(dir, out)
}

// runUpdate moves installed plugins to the newest versions their
// constraints allow: one plugin, with a version to change its constraint
// in dragon.toml, or every plugin.
func runUpdate(args []string, in io.Reader, out io.Writer) error {
	dir, yes, spec, err := installFlags("update", args)
	if err != nil {
		return err
	}
	cfg, err := config.Load(dir)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	opts := install.Options{UpdateAll: spec == ""}
	if spec != "" {
		name, version := install.Spec(spec)
		source, err := sourceOf(dir, cfg, name)
		if err != nil {
			return err
		}
		opts.Update = []string{source}
		if version != "" {
			if _, direct := cfg.Dependencies[source]; !direct {
				return fmt.Errorf("%s is installed because other plugins depend on it, so their constraints pick its version. Add it to dragon.toml with dragon add %s@%s to choose one yourself.", name, source, version)
			}
			constraint, err := install.Constraint(version)
			if err != nil {
				return err
			}
			if err := config.SetDependency(dir, source, constraint); err != nil {
				return err
			}
		}
	}

	if err := syncPlugins(ctx, dir, opts, in, out, yes); err != nil {
		return err
	}

	return reportUnmet(dir, out)
}

// runRemove removes a plugin from dragon.toml's dependencies and uninstalls
// it, with anything it alone needed.
func runRemove(args []string, in io.Reader, out io.Writer) error {
	dir, _, name, err := installFlags("remove", args)
	if err != nil {
		return err
	}
	if name == "" {
		return errors.New("usage: dragon remove <plugin>")
	}
	cfg, err := config.Load(dir)
	if err != nil {
		return err
	}

	source, err := sourceOf(dir, cfg, name)
	if err != nil {
		return err
	}
	if _, direct := cfg.Dependencies[source]; !direct {
		lock, _ := install.ReadLock(dir)
		p, _ := lock.Source(source)
		return fmt.Errorf("%s is installed because %s depends on it, not dragon.toml, so it goes when they do.", name, strings.Join(p.RequiredBy, " and "))
	}
	if err := config.RemoveDependency(dir, source); err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := syncPlugins(ctx, dir, install.Options{}, in, out, true); err != nil {
		config.SetDependency(dir, source, cfg.Dependencies[source])
		return err
	}

	return reportUnmet(dir, out)
}

// sourceOf finds the source of an installed plugin, given its name or
// source, or a dependency in dragon.toml that isn't installed yet.
func sourceOf(dir string, cfg config.Config, nameOrSource string) (string, error) {
	lock, err := install.ReadLock(dir)
	if err != nil {
		return "", err
	}
	if p, _, ok := lock.Find(nameOrSource); ok {
		return p.Source, nil
	}
	if _, ok := cfg.Dependencies[nameOrSource]; ok {
		return nameOrSource, nil
	}

	return "", install.NotInstalled(lock, nameOrSource)
}

// syncPlugins installs what dragon.toml's dependencies resolve to, and says what
// changed.
func syncPlugins(ctx context.Context, dir string, opts install.Options, in io.Reader, out io.Writer, yes bool) error {
	cfg, err := config.Load(dir)
	if err != nil {
		return err
	}
	opts.Dir = dir
	if opts.Dependencies, err = cfg.Constraints(); err != nil {
		return err
	}
	opts.Confirm = confirmer(in, out, yes)

	changes, err := install.Sync(ctx, opts)
	if err != nil {
		return err
	}
	if len(changes) == 0 {
		fmt.Fprintln(out, "Every plugin is installed and up to date.")
		return nil
	}
	for _, c := range changes {
		switch {
		case c.Before == nil:
			fmt.Fprintf(out, "Installed %s %s in %s/%s.\n", c.Name, c.After.Version, install.Dir, c.Name)
		case c.After == nil:
			fmt.Fprintf(out, "Removed %s %s.\n", c.Name, c.Before.Version)
		case c.Before.Version == c.After.Version:
			fmt.Fprintf(out, "Reinstalled %s %s.\n", c.Name, c.After.Version)
		default:
			fmt.Fprintf(out, "Updated %s from %s to %s.\n", c.Name, c.Before.Version, c.After.Version)
		}
	}

	return nil
}

// runList lists the installed plugins.
func runList(args []string, out io.Writer) error {
	flags := flag.NewFlagSet("list", flag.ContinueOnError)
	dir := flags.String("dir", ".", "the game directory")
	if err := flags.Parse(args); err != nil {
		return err
	}
	cfg, err := config.Load(*dir)
	if err != nil {
		return err
	}
	deps, err := cfg.Constraints()
	if err != nil {
		return err
	}

	lock, err := install.Verify(*dir, deps)
	if err != nil {
		return err
	}
	if len(lock.Plugins) == 0 {
		fmt.Fprintln(out, "No plugins are installed. Install one with dragon add <source>, like dragon add github.com/johns/rooms.")
		return nil
	}

	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "Plugin\tVersion\tSource\tNeeded by\tCapabilities")
	for _, p := range lock.Plugins {
		caps := "none"
		if m, err := plugin.ReadManifest(os.DirFS(filepath.Join(*dir, install.Dir, p.Name))); err == nil && len(m.Capabilities) > 0 {
			caps = strings.Join(m.Capabilities, ", ")
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", p.Name, p.Version, p.Source, strings.Join(p.RequiredBy, ", "), caps)
	}

	return w.Flush()
}

// confirmer shows what installing will change and asks whether to go
// ahead, unless yes is set. It asks only when a plugin is new, or a new
// version asks for capabilities the old one didn't.
func confirmer(in io.Reader, out io.Writer, yes bool) func([]install.Change) (bool, error) {
	return func(changes []install.Change) (bool, error) {
		ask := false
		for _, c := range changes {
			if c.After == nil {
				continue
			}
			var caps []string
			switch {
			case c.Before == nil:
				caps = c.Manifest.Capabilities
				fmt.Fprintf(out, "%s %s, from %s, ", c.Name, c.After.Version, c.Source)
				if len(caps) == 0 {
					fmt.Fprintln(out, "asks for no capabilities: it uses only game features.")
				} else {
					fmt.Fprintln(out, "asks for these capabilities:")
				}
				ask = true
			default:
				caps = install.NewCapabilities(*c.Manifest, c.Old)
				if len(caps) == 0 {
					continue
				}
				fmt.Fprintf(out, "%s %s asks for capabilities %s doesn't:\n", c.Name, c.After.Version, c.Before.Version)
				ask = true
			}
			w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
			for _, cap := range caps {
				fmt.Fprintf(w, "  %s\t%s\n", cap, plugin.CapabilityDescs[cap])
			}
			w.Flush()
		}

		if !ask || yes {
			return true, nil
		}
		fmt.Fprint(out, "Install? [y/N] ")
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
	sources, err := pluginSources(dir, cfg)
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
		for _, api := range slices.Sorted(maps.Keys(m.Uses)) {
			dep := m.Uses[api]
			p, ok := providers[api]
			switch {
			case !ok && !dep.Optional:
				problems = append(problems, fmt.Sprintf("%s uses the %s API (%s), but no plugin the game loads provides it.", id, api, dep.Version))
			case ok && !dep.Version.Allows(p.version):
				problems = append(problems, fmt.Sprintf("%s uses the %s API %s, but %s provides %s.", id, api, dep.Version, p.id, p.version))
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
