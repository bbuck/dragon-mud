// Package event implements the engine's events: named chains of plugin
// handlers. An event is one of two kinds. A hook runs its handlers in
// order, and each can change the payload the next one sees or cancel it
// with a reason. A notification tells every handler something already
// happened.
//
// Handlers run in plugin load order (built-ins, then plugins, then the
// game), adjusted by the before and after each handler declares. The game's
// wiring can set an event's order outright, disable handlers, or redirect a
// plugin's handler to a different event.
//
// Every event is declared by the plugin that sends it, or by the engine:
// the declaration says what it's for and which fields its payload has, and
// payloads are checked against it. See docs/design.md §4.
package event

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

// Handler is one plugin's handler for an event.
type Handler struct {
	Event  string
	Plugin string

	// Before and After name plugins whose handlers for the same event this
	// one runs before or after. Plugins that aren't installed, or have no
	// handler for the event, are ignored.
	Before []string
	After  []string

	// Fn is called with the payload. For a hook it returns nothing to keep
	// the payload, a new payload to change it, or false and a reason to
	// cancel. What it returns for a notification is ignored.
	Fn scripting.Function

	// From is the event the plugin wrote the handler for, when the game's
	// wiring redirected it to Event.
	From string
}

// Where is where the handler is defined, for messages: its file and line
// when the scripting language knows them.
func (h Handler) Where() string {
	if h.Fn != nil {
		if source := h.Fn.Source(); source != "" {
			return source
		}
	}

	name := h.Event
	if h.From != "" {
		name = h.From
	}

	return fmt.Sprintf("events.handlers[%q] in %s", name, h.Plugin)
}

// Wiring is how the game rearranges one event's handlers.
type Wiring struct {
	// Order lists, by plugin, every handler that isn't disabled in the
	// order they run. Nil keeps the default order.
	Order []string

	// Disable lists plugins whose handlers don't run.
	Disable []string

	// Redirect moves plugins' handlers to other events: each plugin's
	// handler for this event runs when the event it maps to is sent
	// instead, ordered there like that event's own handlers.
	Redirect map[string]string
}

// Config is everything a Registry is built from.
type Config struct {
	// Plugins are the loaded plugins' ids in load order.
	Plugins []string

	// Handlers are every plugin's handlers, in any order.
	Handlers []Handler

	// Decls are every event's declaration, the engine's and the plugins'.
	Decls []Decl

	// Wiring is keyed by event name.
	Wiring map[string]Wiring
}

// Chain is an event's handlers in the order they run.
type Chain struct {
	Name     string
	Handlers []Handler

	// Disabled are handlers the game's wiring turned off.
	Disabled []Handler

	// Wired is true when the game's wiring set the order.
	Wired bool
}

// Registry holds every event's chain and declaration.
type Registry struct {
	chains   map[string]*Chain
	decls    map[string]Decl
	prefixes []Decl
	plugins  []string

	// redirected are the handlers the game's wiring moved away from each
	// event, keyed by the event they were written for.
	redirected map[string][]Handler
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

// New orders every event's handlers. A cycle in before and after, or wiring
// that doesn't match the handlers, is an error that says how to fix it.
func New(cfg Config) (*Registry, error) {
	loadIndex := make(map[string]int, len(cfg.Plugins))
	for i, id := range cfg.Plugins {
		loadIndex[id] = i
	}

	byEvent := make(map[string][]Handler)
	for _, h := range cfg.Handlers {
		if _, ok := loadIndex[h.Plugin]; !ok {
			return nil, fmt.Errorf("event %q: handler from plugin %q, which isn't loaded", h.Event, h.Plugin)
		}
		for _, other := range byEvent[h.Event] {
			if other.Plugin == h.Plugin {
				return nil, fmt.Errorf("%s: two handlers for %q; a plugin has one handler per event", h.Where(), h.Event)
			}
		}
		byEvent[h.Event] = append(byEvent[h.Event], h)
	}

	r := &Registry{chains: make(map[string]*Chain), decls: make(map[string]Decl), plugins: cfg.Plugins, redirected: make(map[string][]Handler)}
	for _, d := range cfg.Decls {
		if d.Prefix {
			r.prefixes = append(r.prefixes, d)
			continue
		}
		if other, ok := r.decls[d.Name]; ok {
			return nil, fmt.Errorf("%s and %s both declare %q. An event has one declaration, from the plugin that sends it; rename one of them, with its plugin's name in front, like %q.",
				other.Where(), d.Where(), d.Name, d.Plugin+":"+localName(d.Name))
		}
		r.decls[d.Name] = d
	}
	names := slices.Sorted(maps.Keys(byEvent))

	for _, name := range slices.Sorted(maps.Keys(cfg.Wiring)) {
		if _, ok := byEvent[name]; !ok {
			return nil, fmt.Errorf("%s[%q] is wired, but no plugin handles %q.%s Remove it, or add a handler to a plugin's events.handlers.",
				WiringWhere, name, name, command.DidYouMean(name, names))
		}
	}

	if err := r.redirect(byEvent, cfg.Wiring); err != nil {
		return nil, err
	}
	names = slices.Sorted(maps.Keys(byEvent))

	for _, name := range names {
		handlers := byEvent[name]
		slices.SortFunc(handlers, func(a, b Handler) int {
			return cmp.Compare(loadIndex[a.Plugin], loadIndex[b.Plugin])
		})

		chain := &Chain{Name: name}
		wiring, wired := cfg.Wiring[name]

		var err error
		if wired {
			err = chain.wire(handlers, wiring)
		} else {
			chain.Handlers, err = sortHandlers(name, handlers)
		}
		if err != nil {
			return nil, err
		}

		r.chains[name] = chain
	}

	return r, nil
}

// redirect moves the handlers the game's wiring redirects to the events
// they're redirected to, in byEvent.
func (r *Registry) redirect(byEvent map[string][]Handler, wiring map[string]Wiring) error {
	type move struct {
		from, plugin, to string
	}
	var moves []move
	for _, from := range slices.Sorted(maps.Keys(wiring)) {
		w := wiring[from]
		where := fmt.Sprintf("%s[%q]", WiringWhere, from)
		var plugins []string
		for _, h := range byEvent[from] {
			plugins = append(plugins, h.Plugin)
		}

		for _, id := range slices.Sorted(maps.Keys(w.Redirect)) {
			to := w.Redirect[id]
			switch {
			case !slices.Contains(plugins, id):
				return fmt.Errorf("%s redirects %q, which has no %s handler.%s Plugins with a %s handler: %s.",
					where, id, from, command.DidYouMean(id, plugins), from, quoteAll(plugins))
			case slices.Contains(w.Disable, id):
				return fmt.Errorf("%s both redirects and disables %q. Remove it from one of them.", where, id)
			case slices.Contains(w.Order, id):
				return fmt.Errorf("%s both redirects and orders %q. Its handler runs on %s now, so remove it from order here, and order it in [%q] if it needs a place there.",
					where, id, to, to)
			case to == from:
				return fmt.Errorf("%s redirects %q to %s, the event it already handles. Remove the redirect.", where, id, from)
			}
			if _, ok := r.Decl(to); !ok {
				return fmt.Errorf("%s redirects %q to %q, which no plugin declares, so it would never run.%s Redirect it to a declared event; dragon events lists them.",
					where, id, to, command.DidYouMean(to, r.Declared()))
			}
			for _, h := range byEvent[to] {
				if h.Plugin == id && wiring[to].Redirect[id] == "" {
					return fmt.Errorf("%s redirects %q to %s, but %s has a %s handler already, and a plugin has one handler per event. Disable one of them instead, or redirect %s's %s handler somewhere else too.",
						where, id, to, id, to, id, to)
				}
			}
			moves = append(moves, move{from, id, to})
		}
	}

	// Take every redirected handler out before adding any back, so two
	// events can trade a plugin's handlers.
	var moved []Handler
	for _, m := range moves {
		i := slices.IndexFunc(byEvent[m.from], func(h Handler) bool { return h.Plugin == m.plugin })
		h := byEvent[m.from][i]
		byEvent[m.from] = slices.Delete(byEvent[m.from], i, i+1)
		if len(byEvent[m.from]) == 0 {
			delete(byEvent, m.from)
		}

		h.From, h.Event = m.from, m.to
		r.redirected[m.from] = append(r.redirected[m.from], h)
		moved = append(moved, h)
	}
	for _, h := range moved {
		byEvent[h.Event] = append(byEvent[h.Event], h)
	}

	return nil
}

// Redirected returns the handlers the game's wiring moved away from the
// event name, each with the event it runs on now.
func (r *Registry) Redirected(name string) []Handler {
	return r.redirected[name]
}

// wire applies the game's wiring to handlers, which are in load order.
func (c *Chain) wire(handlers []Handler, w Wiring) error {
	where := fmt.Sprintf("%s[%q]", WiringWhere, c.Name)

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
		c.Handlers, err = sortHandlers(c.Name, enabled)
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
func sortHandlers(name string, handlers []Handler) ([]Handler, error) {
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
				return nil, fmt.Errorf("%s: %s handler says after = %q, its own plugin. Remove it.", h.Where(), name, id)
			}
			if j, ok := index[id]; ok {
				addEdge(j, i, fmt.Sprintf("%s says after %s", h.Plugin, id))
			}
		}
		for _, id := range h.Before {
			if id == h.Plugin {
				return nil, fmt.Errorf("%s: %s handler says before = %q, its own plugin. Remove it.", h.Where(), name, id)
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
			return nil, cycleError(name, handlers, edges, done, why)
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
func cycleError(name string, handlers []Handler, edges [][]int, done []bool, why map[[2]int]string) error {
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

	return fmt.Errorf("event %q: handlers can't be ordered because they go in a circle: %s. Remove one of those before/after entries, or set the order yourself in %s: [%q] = { order = { %s } }.",
		name, strings.Join(reasons, ", "), WiringWhere, name, strings.Join(plugins, ", "))
}

// Chain returns the chain for the event name.
func (r *Registry) Chain(name string) (Chain, bool) {
	c, ok := r.chains[name]
	if !ok {
		return Chain{}, false
	}

	return *c, true
}

// Decl returns the declaration of the event name.
func (r *Registry) Decl(name string) (Decl, bool) {
	if d, ok := r.decls[name]; ok {
		return d, true
	}
	for _, d := range r.prefixes {
		if strings.HasPrefix(name, d.Name) {
			return d, true
		}
	}

	return Decl{}, false
}

// Declared returns every declared event, sorted. Prefix declarations aren't
// included.
func (r *Registry) Declared() []string {
	return slices.Sorted(maps.Keys(r.decls))
}

// Undeclared returns the events that have handlers but no declaration,
// sorted: handlers for a plugin that isn't loaded, or for a misspelled
// hook.
func (r *Registry) Undeclared() []string {
	var names []string
	for _, name := range r.Names() {
		if _, ok := r.Decl(name); !ok {
			names = append(names, name)
		}
	}

	return names
}

// Check returns an error if the event name isn't declared or payload doesn't
// match its declaration. Run and Notify check their events with it.
func (r *Registry) Check(name string, event map[string]any) error {
	d, ok := r.Decl(name)
	if !ok {
		return undeclared(name, r.Declared())
	}
	if problem := d.problem(name, event); problem != "" {
		return fmt.Errorf("%s: the event %s", name, problem)
	}

	return nil
}

// WiringWhere is where the game's wiring is, for messages.
const WiringWhere = "the game's events.wiring"

// Names returns every event with handlers, sorted.
func (r *Registry) Names() []string {
	return slices.Sorted(maps.Keys(r.chains))
}

// Run runs the hook name's handlers in order on payload. Each handler sees
// the payload the one before it returned. A handler that cancels stops the
// chain. A handler that fails, or returns a payload that doesn't match the
// hook's declaration, stops it with an error that names the handler. A
// hook with no handlers returns payload unchanged. An undeclared hook, or
// a payload that doesn't match, is an error.
func (r *Registry) Run(ctx context.Context, name string, payload map[string]any) (Result, error) {
	result := Result{Payload: payload}
	if result.Payload == nil {
		result.Payload = map[string]any{}
	}
	if err := r.Check(name, result.Payload); err != nil {
		return result, err
	}
	d, _ := r.Decl(name)

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
			if problem := d.problem(name, v); problem != "" {
				return result, fmt.Errorf("%s: the %s handler returned an event that %s Change the event it was given and return that.", h.Where(), name, problem)
			}
			result.Payload = v
		case bool:
			if v {
				return result, fmt.Errorf("%s: the %s handler returned true. %s", h.Where(), name, returnsHelp)
			}
			result.Cancelled = true
			result.By = h.Plugin
			if len(values) > 1 && values[1] != nil {
				reason, ok := values[1].(string)
				if !ok {
					return result, fmt.Errorf("%s: the %s handler cancelled with a %s as its reason; the reason must be a string, such as return false, \"The door is locked.\"",
						h.Where(), name, scripting.TypeName(values[1]))
				}
				result.Reason = reason
			}
			return result, nil
		default:
			return result, fmt.Errorf("%s: the %s handler returned a %s. %s", h.Where(), name, scripting.TypeName(first), returnsHelp)
		}
	}

	return result, nil
}

const returnsHelp = "A hook handler returns nothing to leave the event as it is, the event to change it, or false and a reason to cancel it."

// Notify calls every handler of the notification name with payload. A
// handler that fails doesn't stop the others; every failure is returned,
// each naming its handler. An undeclared notification, or a payload that
// doesn't match its declaration, is an error and no handler runs.
func (r *Registry) Notify(ctx context.Context, name string, payload map[string]any) error {
	if payload == nil {
		payload = map[string]any{}
	}
	if err := r.Check(name, payload); err != nil {
		return err
	}
	c, ok := r.chains[name]
	if !ok {
		return nil
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
// "after dragon:chat", for dragon events.
func (r *Registry) Explain(c Chain, h Handler) string {
	var parts []string
	if h.From != "" {
		parts = append(parts, "redirected from "+h.From)
	}
	if c.Wired {
		return strings.Join(parts, "; ")
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

	if len(h.After) > 0 {
		parts = append(parts, "after "+describe(h.After))
	}
	if len(h.Before) > 0 {
		parts = append(parts, "before "+describe(h.Before))
	}

	return strings.Join(parts, "; ")
}

// localName is name without its namespace.
func localName(name string) string {
	if _, after, ok := strings.Cut(name, ":"); ok {
		return after
	}

	return name
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
