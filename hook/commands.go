package hook

import (
	"fmt"
	"slices"
	"strings"

	"bbuck.dev/dragon-mud/scripting"
)

// Command is a player command provided by a plugin.
type Command struct {
	Name    string
	Desc    string
	Plugin  string
	Execute scripting.Function

	// Override allows this command to replace one registered earlier by
	// another plugin.
	Override bool
}

// Commands is the default command dispatcher's registry: one owner per
// command name.
type Commands struct {
	byName map[string]*Command
}

// NewCommands returns an empty registry.
func NewCommands() *Commands {
	return &Commands{byName: make(map[string]*Command)}
}

// Register adds cmd. Registering a name another plugin already owns is an
// error unless cmd declares Override.
func (c *Commands) Register(cmd Command) error {
	name := strings.ToLower(cmd.Name)
	if name == "" || strings.ContainsAny(name, " \t") {
		return fmt.Errorf("plugin %q: invalid command name %q", cmd.Plugin, cmd.Name)
	}
	if cmd.Execute == nil {
		return fmt.Errorf("plugin %q: command %q has no execute function", cmd.Plugin, name)
	}

	if existing, ok := c.byName[name]; ok && !cmd.Override {
		return fmt.Errorf(
			"plugin %q: command %q is already provided by %q (declare override = true to replace it)",
			cmd.Plugin, name, existing.Plugin,
		)
	}

	cmd.Name = name
	c.byName[name] = &cmd

	return nil
}

// Lookup returns the command registered under name.
func (c *Commands) Lookup(name string) (*Command, bool) {
	cmd, ok := c.byName[strings.ToLower(name)]

	return cmd, ok
}

// All returns every command, sorted by name.
func (c *Commands) All() []*Command {
	all := make([]*Command, 0, len(c.byName))
	for _, cmd := range c.byName {
		all = append(all, cmd)
	}

	slices.SortFunc(all, func(a, b *Command) int {
		return strings.Compare(a.Name, b.Name)
	})

	return all
}
