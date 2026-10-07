package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"text/tabwriter"

	"bbuck.dev/dragon-mud/config"
	"bbuck.dev/dragon-mud/install"
	"bbuck.dev/dragon-mud/plugin"
	"bbuck.dev/dragon-mud/scaffold"
)

// generators are what dragon gen can make.
var generators = []string{"plugin"}

// runGen generates code in a game: dragon gen plugin <name> makes a local
// plugin in game/plugins/<name>.
func runGen(args []string, out io.Writer) error {
	const usage = "usage: dragon gen plugin <name> [-dir <directory>]"
	if len(args) == 0 {
		return errors.New(usage)
	}
	kind, args := args[0], args[1:]
	if kind != "plugin" {
		return fmt.Errorf("dragon gen can't make a %q. It makes: %s. %s", kind, generators[0], usage)
	}

	flags := flag.NewFlagSet("gen plugin", flag.ContinueOnError)
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
	if name == "" {
		return errors.New(usage)
	}

	if _, err := os.Stat(filepath.Join(*dir, config.FileName)); err != nil {
		return fmt.Errorf("%s has no %s, so it isn't a game directory. Run dragon gen in a game made with dragon new, or pass -dir.", *dir, config.FileName)
	}
	if err := plugin.ValidName(name); err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(*dir, install.Dir, name)); err == nil {
		return fmt.Errorf("%s/%s is an installed plugin called %s already. Name the new one something else.", install.Dir, name, name)
	}

	target := filepath.Join(*dir, "game", plugin.LocalDir, name)
	if err := scaffold.Plugin(target, name); err != nil {
		return err
	}

	rel, err := filepath.Rel(*dir, target)
	if err != nil {
		rel = target
	}
	fmt.Fprintf(out, "Created the %s plugin in %s.\n\n", name, rel)
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "  dragon plugin %s\tsee what it provides\n", name)
	fmt.Fprintf(w, "  dragon test -run %s\trun its test\n", name)
	w.Flush()
	fmt.Fprintln(out, "\nIt loads with the game, and reloads when you change it while dragon serve runs.")

	return nil
}
