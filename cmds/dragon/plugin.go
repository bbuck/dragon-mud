package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"
	"text/tabwriter"

	"bbuck.dev/dragon-mud/ansi"
	"bbuck.dev/dragon-mud/command"
	"bbuck.dev/dragon-mud/config"
	"bbuck.dev/dragon-mud/event"
	"bbuck.dev/dragon-mud/game"
	"bbuck.dev/dragon-mud/plugin"
	"bbuck.dev/dragon-mud/scripting"
	"bbuck.dev/dragon-mud/scripting/lua"
)

// runPlugin lists the game's plugins, or shows everything one provides.
func runPlugin(args []string, out io.Writer) error {
	flags := flag.NewFlagSet("plugin", flag.ContinueOnError)
	dir := flags.String("dir", ".", "the game directory")

	var name string
	if len(args) > 0 && args[0] != "" && args[0][0] != '-' {
		name, args = args[0], args[1:]
	}
	if err := flags.Parse(args); err != nil {
		return err
	}
	if name == "" {
		name = flags.Arg(0)
	}

	cfg, err := config.Load(*dir)
	if err != nil {
		return err
	}
	sources, err := pluginSources(*dir, cfg)
	if err != nil {
		return err
	}
	info, err := game.Inspect(context.Background(), game.Options{
		Name:      cfg.Name,
		NewEngine: func() scripting.Engine { return lua.New() },
		Plugins:   sources,
	})
	if err != nil {
		return err
	}

	if name == "" {
		return listPlugins(out, info)
	}
	p, err := findPlugin(info.Plugins, name)
	if err != nil {
		return err
	}

	return showPlugin(out, info, p)
}

// findPlugin finds a plugin by its id, or by its name when only one
// plugin has it: chat for dragon:chat.
func findPlugin(plugins []*plugin.Plugin, name string) (*plugin.Plugin, error) {
	var ids, matches []string
	var found *plugin.Plugin
	for _, p := range plugins {
		ids = append(ids, p.ID)
		if p.ID == name {
			return p, nil
		}
		if p.Manifest.Name == name {
			matches = append(matches, p.ID)
			found = p
		}
	}

	switch len(matches) {
	case 1:
		return found, nil
	case 0:
		return nil, fmt.Errorf("the game loads no plugin called %s.%s Plugins: %s.", name, command.DidYouMean(name, ids), strings.Join(ids, ", "))
	}

	return nil, fmt.Errorf("more than one plugin is called %s: %s. Name the one you mean.", name, strings.Join(matches, " and "))
}

func listPlugins(out io.Writer, info *game.Inspection) error {
	fmt.Fprintln(out, "Plugins, in the order they load:")
	fmt.Fprintln(out)
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	for _, p := range info.Plugins {
		version := p.Manifest.Version
		if version == "" {
			version = "-"
		}
		fmt.Fprintf(w, "  %s\t%s\t%s\n", p.ID, version, p.Origin)
	}
	w.Flush()
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Run dragon plugin <name> to see everything one provides.")

	return nil
}

// showPlugin prints everything p provides, a section for each part, so a
// game author can see what they can add to, replace or wire.
func showPlugin(out io.Writer, info *game.Inspection, p *plugin.Plugin) error {
	title := p.ID
	if p.Manifest.Version != "" {
		title += " " + p.Manifest.Version
	}
	fmt.Fprintf(out, "%s, from %s\n", title, p.Origin)

	apis := p.APIs()
	for _, api := range slices.Sorted(maps.Keys(p.Manifest.Provides)) {
		fmt.Fprintf(out, "  Provides the %s API %s, as require(\"@%s\") (%s/%s.lua).\n", api, p.Manifest.Provides[api], api, plugin.ModulesDir, strings.ReplaceAll(apis[api], ".", "/"))
	}
	for _, api := range slices.Sorted(maps.Keys(p.Manifest.Depends)) {
		dep := p.Manifest.Depends[api]
		optional := ""
		if dep.Optional {
			optional = ", optionally"
		}
		fmt.Fprintf(out, "  Depends on the %s API %s%s.\n", api, dep.Version, optional)
	}
	switch {
	case p.ID == plugin.GameID:
		fmt.Fprintln(out, "  Has every capability, as the game's own plugin.")
	case len(p.Manifest.Capabilities) > 0:
		fmt.Fprintf(out, "  Capabilities: %s.\n", strings.Join(p.Manifest.Capabilities, ", "))
	}

	sections := []func(io.Writer, *game.Inspection, *plugin.Plugin) error{
		showCommands, showSlots, showModes, showDeclared, showHandlers, showViews, showTypes, showTasks, showClient,
	}
	for _, section := range sections {
		if err := section(out, info, p); err != nil {
			return err
		}
	}

	return nil
}

// section starts a section of dragon plugin's output.
func section(out io.Writer, title string) *tabwriter.Writer {
	fmt.Fprintf(out, "\n%s\n", title)

	return tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
}

func showCommands(out io.Writer, _ *game.Inspection, p *plugin.Plugin) error {
	cmds, err := p.Commands()
	if err != nil || len(cmds) == 0 {
		return err
	}

	w := section(out, "Commands, and the forms it gives them:")
	for _, c := range cmds {
		desc := c.Desc
		if c.Replace {
			desc = strings.TrimSpace(desc + " (replaces earlier forms)")
		}
		fmt.Fprintf(w, "  %s\t%s\n", c.Name, desc)
		for _, f := range c.Forms {
			fmt.Fprintf(w, "    %s\t%s\n", f.Pattern, f.Desc)
		}
	}

	return w.Flush()
}

func showSlots(out io.Writer, _ *game.Inspection, p *plugin.Plugin) error {
	slots, err := p.Slots()
	if err != nil || len(slots) == 0 {
		return err
	}

	w := section(out, "Slot types:")
	for _, s := range slots {
		desc := s.Desc
		if len(s.Modifiers) > 0 {
			desc = strings.TrimSpace(desc + " Modifiers: " + strings.Join(s.Modifiers, ", ") + ".")
		}
		fmt.Fprintf(w, "  <name:%s>\t%s\n", s.Name, desc)
	}

	return w.Flush()
}

func showModes(out io.Writer, _ *game.Inspection, p *plugin.Plugin) error {
	modes, err := p.Modes()
	if err != nil || len(modes) == 0 {
		return err
	}

	w := section(out, "Input modes:")
	for _, m := range modes {
		var parts []string
		for _, h := range plugin.ModeHandlers {
			if _, ok := m.Handlers[h]; ok {
				parts = append(parts, h)
			}
		}
		if len(m.Forms) > 0 {
			parts = append(parts, fmt.Sprintf("%d forms", len(m.Forms)))
		}
		desc := m.Desc
		if len(parts) > 0 {
			desc = strings.TrimSpace(desc + " (" + strings.Join(parts, ", ") + ")")
		}
		fmt.Fprintf(w, "  %s\t%s\n", m.Name, desc)
	}

	return w.Flush()
}

func showDeclared(out io.Writer, _ *game.Inspection, p *plugin.Plugin) error {
	decls, err := p.Declarations()
	if err != nil || len(decls) == 0 {
		return err
	}

	w := section(out, "Events it sends, which other plugins can handle:")
	for _, d := range decls {
		var fields []string
		for _, f := range d.Fields {
			name := f.Name
			if f.Optional {
				name += "?"
			}
			fields = append(fields, name)
		}
		fmt.Fprintf(w, "  %s\t(%s)\t%s\n", d.Name, strings.Join(fields, ", "), firstSentence(d.Desc))
	}
	w.Flush()
	fmt.Fprintln(out, "  Run dragon events <name> for an event's fields and handlers.")

	return nil
}

func showHandlers(out io.Writer, info *game.Inspection, p *plugin.Plugin) error {
	handlers, err := p.Handlers()
	if err != nil || len(handlers) == 0 {
		return err
	}

	w := section(out, "Events it handles, and where its handler runs:")
	for _, h := range handlers {
		fmt.Fprintf(w, "  %s\t%s\n", h.Event, handlerPlace(info.Events, h))
	}

	return w.Flush()
}

// handlerPlace says where h runs among its event's handlers: its place in
// the order, or that the game's wiring disabled or redirected it.
func handlerPlace(events *event.Registry, h event.Handler) string {
	for _, moved := range events.Redirected(h.Event) {
		if moved.Plugin == h.Plugin {
			return "redirected to " + moved.Event + " by " + event.WiringWhere
		}
	}

	c, ok := events.Chain(h.Event)
	if !ok {
		return "no plugin declares it, so it never runs"
	}
	for _, d := range c.Disabled {
		if d.Plugin == h.Plugin {
			return "disabled by " + event.WiringWhere
		}
	}
	for i, other := range c.Handlers {
		if other.Plugin == h.Plugin {
			place := fmt.Sprintf("runs %d of %d", i+1, len(c.Handlers))
			if _, declared := events.Decl(h.Event); !declared {
				place += "; no plugin declares it, so it never runs"
			}
			return place
		}
	}

	return ""
}

func showViews(out io.Writer, _ *game.Inspection, p *plugin.Plugin) error {
	for _, part := range []struct{ dir, title string }{
		{plugin.ViewsDir, "Views it sends, which the game can restyle with its own files:"},
		{plugin.TemplatesDir, "Other templates:"},
	} {
		files, err := p.Templates(part.dir)
		if err != nil {
			return err
		}
		if len(files) == 0 {
			continue
		}

		formats := make(map[string][]string)
		for _, f := range files {
			formats[f.Name] = append(formats[f.Name], f.Format)
		}
		w := section(out, part.title)
		for _, name := range slices.Sorted(maps.Keys(formats)) {
			slices.Sort(formats[name])
			fmt.Fprintf(w, "  %s\t%s\n", name, strings.Join(formats[name], ", "))
		}
		w.Flush()
	}

	return nil
}

func showTypes(out io.Writer, info *game.Inspection, p *plugin.Plugin) error {
	var own, added []string
	for _, name := range info.Schema.Names() {
		t, _ := info.Schema.Type(name)
		if t.Plugin == p.ID {
			own = append(own, name)
		}
		for _, f := range t.Added {
			if f.Plugin == p.ID {
				added = append(added, name)
				break
			}
		}
	}

	if len(own) > 0 {
		w := section(out, "Object types:")
		for _, name := range own {
			t, _ := info.Schema.Type(name)
			fmt.Fprintf(w, "  %s\t%s\n", name, t.Desc)
			for _, f := range append(slices.Clone(t.Fields), t.Added...) {
				desc := f.Desc
				if f.HasDefault {
					desc += fmt.Sprintf(" (default %v)", f.Default)
				}
				if f.Plugin != p.ID {
					desc += " (added by " + f.Plugin + ")"
				}
				fmt.Fprintf(w, "    %s\t%s\t%s\n", f.Name, f.Kind, desc)
			}
		}
		w.Flush()
	}
	if len(added) > 0 {
		w := section(out, "Fields it adds to other plugins' types:")
		for _, name := range added {
			t, _ := info.Schema.Type(name)
			for _, f := range t.Added {
				if f.Plugin == p.ID {
					fmt.Fprintf(w, "  %s\t%s\t%s\t%s\n", name, f.Name, f.Kind, f.Desc)
				}
			}
		}
		w.Flush()
	}

	return nil
}

func showTasks(out io.Writer, _ *game.Inspection, p *plugin.Plugin) error {
	tasks, err := p.Tasks()
	if err != nil || len(tasks) == 0 {
		return err
	}

	w := section(out, "Tasks, run with dragon <task>:")
	for _, t := range tasks {
		fmt.Fprintf(w, "  %s\t%s\n", t.Name, t.Desc)
	}

	return w.Flush()
}

func showClient(out io.Writer, _ *game.Inspection, p *plugin.Plugin) error {
	defs, err := p.Client()
	if err != nil {
		return err
	}
	web, err := p.Web()
	if err != nil {
		return err
	}
	if len(defs) == 0 && web == nil {
		return nil
	}

	w := section(out, "Web client:")
	if web != nil {
		fmt.Fprintf(w, "  files\timported as %s/", web.Namespace)
		for _, api := range web.APIs {
			fmt.Fprintf(w, " and %s/", api)
		}
		fmt.Fprintln(w)
		if web.Main {
			fmt.Fprintf(w, "  loads\t%s on every page\n", plugin.WebMain)
		}
		for _, style := range web.Styles {
			fmt.Fprintf(w, "  links\t%s on every page\n", style)
		}
	}
	for _, d := range defs {
		fmt.Fprintf(w, "  handles\t%s, from client.push or client.request\n", d.Name)
	}

	return w.Flush()
}

// firstSentence is desc up to its first full stop, for one-line listings.
func firstSentence(desc string) string {
	if i := strings.Index(desc, ". "); i >= 0 {
		return desc[:i+1]
	}

	return ansi.Purge(desc)
}
