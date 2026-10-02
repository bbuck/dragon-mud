// Package hook implements the engine's hooks and notifications: named
// chains of plugin handlers. A hook runs its handlers in order, and each can
// change the payload the next one sees or cancel it with a reason. A
// notification tells every handler something already happened.
//
// Handlers run in plugin load order (built-ins, then plugins, then the
// game), adjusted by the before and after each handler declares. The game's
// wiring can set a hook's order outright or disable handlers.
// See docs/design.md §3.
package hook

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"bbuck.dev/dragon-mud/command"
	"bbuck.dev/dragon-mud/scripting"
)

// Handler is one plugin's handler for a hook.
type Handler struct {
	Hook   string
	Plugin string

	// Before and After name plugins whose handlers for the same hook this
	// one runs before or after. Plugins that aren't installed, or have no
	// handler for the hook, are ignored.
	Before []string
	After  []string

	// Fn is called with the payload. For a hook it returns nothing to keep
	// the payload, a new payload to change it, or false and a reason to
	// cancel. What it returns for a notification is ignored.
	Fn scripting.Function
}

// File is where the handler is declared, for messages.
func (h Handler) File() string {
	return h.Plugin + "/hooks.lua"
}

// Wiring is how the game rearranges one hook's handlers.
type Wiring struct {
	// Order lists, by plugin, every handler that isn't disabled in the
	// order they run. Nil keeps the default order.
	Order []string

	// Disable lists plugins whose handlers don't run.
	Disable []string
}

// Config is everything a Registry is built from.
type Config struct {
	// Plugins are the loaded plugins' ids in load order.
	Plugins []string

	// Handlers are every plugin's handlers, in any order.
	Handlers []Handler

	// Wiring is keyed by hook name.
	Wiring map[string]Wiring

	// WiringFile is where Wiring came from, for messages, such as
	// "game/wiring.lua".
	WiringFile string
}

// Chain is a hook's handlers in the order they run.
type Chain struct {
	Name     string
	Handlers []Handler

	// Disabled are handlers the game's wiring turned off.
	Disabled []Handler

	// Wired is true when the game's wiring set the order.
	Wired bool
}

// Registry holds every hook's chain.
type Registry struct {
	chains     map[string]*Chain
	plugins    []string
	wiringFile string
}

// Result is what running a hook decided.
type Result struct {
	// Payload is the payload as the last handler left it.
	Payload map[string]any

	// Cancelled is set when a handler cancelled. Reason is what it gave,
	// and By is its plugin.
	Cancelled bool
	Reason    string
	By        string
}

// New orders every hook's handlers. A cycle in before and after, or wiring
// that doesn't match the handlers, is an error that says how to fix it.
func New(cfg Config) (*Registry, error) {
	if cfg.WiringFile == "" {
		cfg.WiringFile = "the game's wiring.lua"
	}

	loadIndex := make(map[string]int, len(cfg.Plugins))
	for i, id := range cfg.Plugins {
		loadIndex[id] = i
	}

	byHook := make(map[string][]Handler)
	for _, h := range cfg.Handlers {
		if _, ok := loadIndex[h.Plugin]; !ok {
			return nil, fmt.Errorf("hook %q: handler from plugin %q, which isn't loaded", h.Hook, h.Plugin)
		}
		for _, other := range byHook[h.Hook] {
			if other.Plugin == h.Plugin {
				return nil, fmt.Errorf("%s: two handlers for %q; a plugin has one handler per hook", h.File(), h.Hook)
			}
		}
		byHook[h.Hook] = append(byHook[h.Hook], h)
	}

	r := &Registry{chains: make(map[string]*Chain), plugins: cfg.Plugins, wiringFile: cfg.WiringFile}
	names := slices.Sorted(maps.Keys(byHook))

	for _, name := range slices.Sorted(maps.Keys(cfg.Wiring)) {
		if _, ok := byHook[name]; !ok {
			return nil, fmt.Errorf("%s: hooks.%s is wired, but no plugin handles %q.%s Remove it, or add a handler to a plugin's hooks.lua.",
				cfg.WiringFile, name, name, command.DidYouMean(name, names))
		}
	}

	for _, name := range names {
		handlers := byHook[name]
		slices.SortFunc(handlers, func(a, b Handler) int {
			return cmp.Compare(loadIndex[a.Plugin], loadIndex[b.Plugin])
		})

		chain := &Chain{Name: name}
		wiring, wired := cfg.Wiring[name]

		var err error
		if wired {
			err = chain.wire(handlers, wiring, cfg.WiringFile)
		} else {
			chain.Handlers, err = sortHandlers(name, handlers, cfg.WiringFile)
		}
		if err != nil {
			return nil, err
		}

		r.chains[name] = chain
	}

	return r, nil
}

// wire applies the game's wiring to handlers, which are in load order.
func (c *Chain) wire(handlers []Handler, w Wiring, file string) error {
	where := fmt.Sprintf("%s: hooks.%s", file, c.Name)

	plugins := make([]string, len(handlers))
	byPlugin := make(map[string]Handler, len(handlers))
	for i, h := range handlers {
		plugins[i] = h.Plugin
		byPlugin[h.Plugin] = h
	}
	handledBy := fmt.Sprintf("Plugins with a %s handler: %s.", c.Name, quoteAll(plugins))

	disabled := make(map[string]bool)
	for _, id := range w.Disable {
		if _, ok := byPlugin[id]; !ok {
			return fmt.Errorf("%s disables %q, which has no %s handler.%s %s", where, id, c.Name, command.DidYouMean(id, plugins), handledBy)
		}
		if disabled[id] {
			return fmt.Errorf("%s disables %q twice. Remove one.", where, id)
		}
		disabled[id] = true
	}

	if w.Order == nil {
		var enabled []Handler
		for _, h := range handlers {
			if disabled[h.Plugin] {
				c.Disabled = append(c.Disabled, h)
			} else {
				enabled = append(enabled, h)
			}
		}

		var err error
		c.Handlers, err = sortHandlers(c.Name, enabled, file)
		return err
	}

	c.Wired = true
	placed := make(map[string]bool)
	for _, id := range w.Order {
		h, ok := byPlugin[id]
		switch {
		case !ok:
			return fmt.Errorf("%s orders %q, which has no %s handler.%s %s", where, id, c.Name, command.DidYouMean(id, plugins), handledBy)
		case disabled[id]:
			return fmt.Errorf("%s both orders and disables %q. Remove it from one of them.", where, id)
		case placed[id]:
			return fmt.Errorf("%s orders %q twice. List each plugin once.", where, id)
		}
		placed[id] = true
		c.Handlers = append(c.Handlers, h)
	}

	var missing []string
	for _, h := range handlers {
		switch {
		case disabled[h.Plugin]:
			c.Disabled = append(c.Disabled, h)
		case !placed[h.Plugin]:
			missing = append(missing, h.Plugin)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("%s sets the order but leaves out %s. Add %s to order, or to disable to turn %s off.",
			where, quoteAll(missing), them(missing), them(missing))
	}

	return nil
}

// sortHandlers orders handlers, which are in load order, by their before
// and after. Handlers with nothing between them keep load order.
func sortHandlers(name string, handlers []Handler, wiringFile string) ([]Handler, error) {
	index := make(map[string]int, len(handlers))
	for i, h := range handlers {
		index[h.Plugin] = i
	}

	// edges[i] are the handlers that must run after handlers[i]; why
	// explains each edge for error messages.
	edges := make([][]int, len(handlers))
	why := make(map[[2]int]string)
	indegree := make([]int, len(handlers))
	addEdge := func(from, to int, reason string) {
		if _, ok := why[[2]int{from, to}]; ok {
			return
		}
		why[[2]int{from, to}] = reason
		edges[from] = append(edges[from], to)
		indegree[to]++
	}

	for i, h := range handlers {
		for _, id := range h.After {
			if id == h.Plugin {
				return nil, fmt.Errorf("%s: %s handler says after = %q, its own plugin. Remove it.", h.File(), name, id)
			}
			if j, ok := index[id]; ok {
				addEdge(j, i, fmt.Sprintf("%s says after %s", h.Plugin, id))
			}
		}
		for _, id := range h.Before {
			if id == h.Plugin {
				return nil, fmt.Errorf("%s: %s handler says before = %q, its own plugin. Remove it.", h.File(), name, id)
			}
			if j, ok := index[id]; ok {
				addEdge(i, j, fmt.Sprintf("%s says before %s", h.Plugin, id))
			}
		}
	}

	// Kahn's algorithm, always taking the earliest-loaded ready handler.
	sorted := make([]Handler, 0, len(handlers))
	done := make([]bool, len(handlers))
	for len(sorted) < len(handlers) {
		next := -1
		for i := range handlers {
			if !done[i] && indegree[i] == 0 {
				next = i
				break
			}
		}
		if next < 0 {
			return nil, cycleError(name, handlers, edges, done, why, wiringFile)
		}

		done[next] = true
		sorted = append(sorted, handlers[next])
		for _, to := range edges[next] {
			indegree[to]--
		}
	}

	return sorted, nil
}

// cycleError describes one cycle among the handlers not yet sorted.
func cycleError(name string, handlers []Handler, edges [][]int, done []bool, why map[[2]int]string, wiringFile string) error {
	// Every unsorted handler has an unsorted predecessor, so walking
	// backwards from any of them must revisit one.
	preds := make([][]int, len(handlers))
	for from, tos := range edges {
		for _, to := range tos {
			preds[to] = append(preds[to], from)
		}
	}

	seen := make(map[int]int) // handler index -> position in path
	var path []int
	for at := slices.Index(done, false); ; {
		if pos, ok := seen[at]; ok {
			path = path[pos:]
			break
		}
		seen[at] = len(path)
		path = append(path, at)
		for _, from := range preds[at] {
			if !done[from] {
				at = from
				break
			}
		}
	}
	slices.Reverse(path)

	reasons := make([]string, len(path))
	plugins := make([]string, len(path))
	for i, from := range path {
		to := path[(i+1)%len(path)]
		reasons[i] = why[[2]int{from, to}]
		plugins[i] = fmt.Sprintf("%q", handlers[from].Plugin)
	}

	return fmt.Errorf("hook %q: handlers can't be ordered because they go in a circle: %s. Remove one of those before/after entries, or set the order yourself in %s: hooks = { %s = { order = { %s } } }.",
		name, strings.Join(reasons, ", "), wiringFile, name, strings.Join(plugins, ", "))
}

// Chain returns the chain for the hook name.
func (r *Registry) Chain(name string) (Chain, bool) {
	c, ok := r.chains[name]
	if !ok {
		return Chain{}, false
	}

	return *c, true
}

// WiringFile is where the game's wiring is, for messages.
func (r *Registry) WiringFile() string {
	return r.wiringFile
}

// Names returns every hook with handlers, sorted.
func (r *Registry) Names() []string {
	return slices.Sorted(maps.Keys(r.chains))
}

// Run runs the hook name's handlers in order on payload. Each handler sees
// the payload the one before it returned. A handler that cancels stops the
// chain. A handler that fails stops it with an error that names the
// handler. A hook with no handlers returns payload unchanged.
func (r *Registry) Run(ctx context.Context, name string, payload map[string]any) (Result, error) {
	result := Result{Payload: payload}
	if result.Payload == nil {
		result.Payload = map[string]any{}
	}

	c, ok := r.chains[name]
	if !ok {
		return result, nil
	}

	for _, h := range c.Handlers {
		values, err := h.Fn.CallAll(ctx, result.Payload)
		if err != nil {
			return result, fmt.Errorf("%s handler from %s failed: %w", name, h.Plugin, err)
		}

		var first any
		if len(values) > 0 {
			first = values[0]
		}

		switch v := first.(type) {
		case nil:
		case map[string]any:
			result.Payload = v
		case bool:
			if v {
				return result, fmt.Errorf("%s: the %s handler returned true. %s", h.File(), name, returnsHelp)
			}
			result.Cancelled = true
			result.By = h.Plugin
			if len(values) > 1 && values[1] != nil {
				reason, ok := values[1].(string)
				if !ok {
					return result, fmt.Errorf("%s: the %s handler cancelled with a %s as its reason; the reason must be a string, such as return false, \"The door is locked.\"",
						h.File(), name, scripting.TypeName(values[1]))
				}
				result.Reason = reason
			}
			return result, nil
		default:
			return result, fmt.Errorf("%s: the %s handler returned a %s. %s", h.File(), name, scripting.TypeName(first), returnsHelp)
		}
	}

	return result, nil
}

const returnsHelp = "A hook handler returns nothing to leave the event as it is, the event to change it, or false and a reason to cancel it."

// Notify calls every handler of the notification name with payload. A
// handler that fails doesn't stop the others; every failure is returned,
// each naming its handler.
func (r *Registry) Notify(ctx context.Context, name string, payload map[string]any) error {
	c, ok := r.chains[name]
	if !ok {
		return nil
	}
	if payload == nil {
		payload = map[string]any{}
	}

	var errs []error
	for _, h := range c.Handlers {
		if _, err := h.Fn.Call(ctx, payload); err != nil {
			errs = append(errs, fmt.Errorf("%s handler from %s failed: %w", name, h.Plugin, err))
		}
	}

	return errors.Join(errs...)
}

// Explain describes why h runs where it does in c, such as
// "after dragon:chat", for dragon hooks.
func (r *Registry) Explain(c Chain, h Handler) string {
	if c.Wired {
		return ""
	}

	loaded := make(map[string]bool, len(r.plugins))
	for _, id := range r.plugins {
		loaded[id] = true
	}
	handles := make(map[string]bool, len(c.Handlers))
	for _, other := range c.Handlers {
		handles[other.Plugin] = true
	}

	describe := func(ids []string) string {
		parts := make([]string, len(ids))
		for i, id := range ids {
			switch {
			case !loaded[id]:
				parts[i] = id + " (not installed)"
			case !handles[id]:
				parts[i] = id + " (no handler)"
			default:
				parts[i] = id
			}
		}
		return strings.Join(parts, ", ")
	}

	var parts []string
	if len(h.After) > 0 {
		parts = append(parts, "after "+describe(h.After))
	}
	if len(h.Before) > 0 {
		parts = append(parts, "before "+describe(h.Before))
	}

	return strings.Join(parts, "; ")
}

func quoteAll(ids []string) string {
	quoted := make([]string, len(ids))
	for i, id := range ids {
		quoted[i] = fmt.Sprintf("%q", id)
	}

	return strings.Join(quoted, ", ")
}

func them(ids []string) string {
	if len(ids) == 1 {
		return "it"
	}

	return "them"
}
