package game

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	"bbuck.dev/dragon-mud/command"
	"bbuck.dev/dragon-mud/message"
	"bbuck.dev/dragon-mud/plugin"
	"bbuck.dev/dragon-mud/scripting"
)

// Modes the engine knows by name. See docs/design.md §3, Input modes.
const (
	// modeLogin is the engine's own login, always at the bottom of a new
	// session's stack.
	modeLogin = plugin.LoginMode

	// After login the engine starts the game's characters mode if it has
	// one, otherwise dragon:characters. It ends by calling
	// session:play(character).
	modeCharacters        = "characters"
	modeBuiltinCharacters = plugin.BuiltinPrefix + "characters"
)

// Reasons a mode's leave handler is given.
const (
	leaveDone         = "done"
	leaveReplaced     = "replaced"
	leavePlaying      = "playing"
	leaveDisconnected = "disconnected"
)

// mode is an input mode, merged from every plugin that defines it.
type mode struct {
	name string
	desc string

	// plugin defined the mode first, or replaced it.
	plugin string

	handlers    map[string]scripting.Function
	passthrough bool

	// owners says which plugin set each handler, and passthrough.
	owners map[string]string

	// forms are the mode's forms, registered as one command named after
	// the mode.
	forms *command.Registry
}

// frame is one mode on a session's stack. Frames hold the mode's name, not
// the mode, so they survive a reload.
type frame struct {
	name  string
	state map[string]any

	// login is set for the engine's login mode.
	login *login

	// choices are the last prompt's choices, so a number answers with the
	// matching choice.
	choices []string

	// running counts the mode's handlers that are running. A mode above it
	// that ends meanwhile, such as a step that finishes in its own enter,
	// resumes it once they return, so resume sees the state they leave.
	running int
	resumes []any
}

// addMode registers a plugin's definition of a mode. Forms add to any the
// mode already has; handlers can only be defined once unless the
// definition replaces the mode.
func (s *scripts) addMode(def plugin.ModeDef) error {
	m, exists := s.modes[def.Name]
	if def.Replace || !exists {
		m = &mode{
			name:     def.Name,
			plugin:   def.Plugin,
			handlers: make(map[string]scripting.Function),
			owners:   make(map[string]string),
			forms:    s.commands.Scoped(),
		}
	}

	conflict := func(what string) error {
		return fmt.Errorf("%s: mode %q sets %s, but %s already does. Set replace = true on %q in %s to use only its version, or remove %s from one of them. (Plugins avoid this by namespacing their modes, like %q.)",
			def.File, def.Name, what, m.owners[what], def.Name, def.File, what, "myplugin:"+def.Name)
	}

	for _, h := range plugin.ModeHandlers {
		fn, ok := def.Handlers[h]
		if !ok {
			continue
		}
		if _, taken := m.owners[h]; taken {
			return conflict(h)
		}
		m.handlers[h], m.owners[h] = fn, def.Plugin
	}
	if def.Passthrough != nil {
		if _, taken := m.owners["passthrough"]; taken {
			return conflict("passthrough")
		}
		m.passthrough, m.owners["passthrough"] = *def.Passthrough, def.Plugin
	}
	if def.Desc != "" {
		m.desc = def.Desc
	}
	if len(def.Forms) > 0 {
		if err := m.forms.Add(command.CommandDef{Name: def.Name, Plugin: def.Plugin, Forms: def.Forms}); err != nil {
			return err
		}
	}

	s.modes[def.Name] = m

	return nil
}

// checkModes checks what the engine needs from the loaded modes.
func (s *scripts) checkModes() error {
	if s.charactersMode() == "" {
		return fmt.Errorf("nothing defines a mode to run after a player logs in to choose or create their character. Add %q back to builtins in dragon.toml for %s, or define %s in game/modes.lua and have it call session:play(character).",
			"characters", modeBuiltinCharacters, modeCharacters)
	}

	return nil
}

// charactersMode is the mode that runs after login: the game's characters
// mode if there is one, otherwise dragon:characters.
func (s *scripts) charactersMode() string {
	for _, name := range []string{modeCharacters, modeBuiltinCharacters} {
		if _, ok := s.modes[name]; ok {
			return name
		}
	}

	return ""
}

// lookupMode returns the mode called name, or an error naming the modes
// there are.
func (g *Game) lookupMode(name string) (*mode, error) {
	if name == modeLogin {
		return nil, fmt.Errorf("%s is the engine's own login, which only the engine starts. To send a player back to it, close their session with session:close().", modeLogin)
	}
	if m, ok := g.modes[name]; ok {
		return m, nil
	}

	names := slices.Sorted(maps.Keys(g.modes))
	return nil, fmt.Errorf("there's no mode %q. Define it in a plugin's modes.lua.%s Modes: %s.",
		name, command.DidYouMean(name, names), strings.Join(names, ", "))
}

// top returns the mode input goes to first, or nil.
func (p *player) top() *frame {
	if len(p.modes) == 0 {
		return nil
	}

	return p.modes[len(p.modes)-1]
}

// loginState returns the login in progress, or nil.
func (p *player) loginState() *login {
	if f := p.top(); f != nil {
		return f.login
	}

	return nil
}

func (p *player) onStack(f *frame) bool {
	return slices.Contains(p.modes, f)
}

// input handles a line from p: its modes first, from the top, then the
// command dispatcher once it's playing.
func (g *Game) input(ctx context.Context, p *player, line string) {
	ctx, cancel := context.WithTimeout(ctx, scriptTimeout)
	defer cancel()

	for i := len(p.modes) - 1; i >= 0; i-- {
		f := p.modes[i]
		if f.login != nil {
			// Not trimmed: passwords may have spaces at either end.
			g.login(ctx, p, f.login, line)
			return
		}
		if g.modeInput(ctx, p, f, line, i == len(p.modes)-1) {
			return
		}
	}

	if p.character != nil {
		g.dispatch(ctx, p, strings.TrimSpace(line))
	}
}

// modeInput offers line to the mode f: its forms, then its input handler.
// It returns false when the mode passes input it didn't handle to the mode
// below.
func (g *Game) modeInput(ctx context.Context, p *player, f *frame, line string, top bool) bool {
	m, ok := g.modes[f.name]
	if !ok {
		return false
	}

	trimmed := strings.TrimSpace(line)
	if n, err := strconv.Atoi(trimmed); err == nil && top && n >= 1 && n <= len(f.choices) {
		trimmed = f.choices[n-1]
		line = trimmed
	}

	var miss *command.NoMatch
	if trimmed != "" && len(m.forms.All()) > 0 {
		g.resolving = true
		match, noMatch, err := m.forms.Parse(ctx, g.handle(p.character), trimmed)
		g.resolving = false

		switch {
		case err != nil:
			g.log.Error("resolving input failed", "mode", m.name, "input", trimmed, "error", err)
			p.s.Send(message.System(fmt.Sprintf("[R]%v[x]", err)))
			return true
		case match != nil:
			form := match.Form
			if err := g.runMode(ctx, p, f, form.Execute, match.Args, f.state); err != nil {
				g.log.Error("mode form failed", "mode", m.name, "form", form.Pattern.Source, "plugin", form.Plugin, "error", err)
				p.s.Send(message.System(fmt.Sprintf("[R]The %s mode's %q form (from %s) failed: %v[x]", m.name, form.Pattern.Source, form.Plugin, err)))
			}
			return true
		}
		miss = noMatch
	}

	if fn, ok := m.handlers["input"]; ok {
		if err := g.runMode(ctx, p, f, fn, line, f.state); err != nil {
			g.modeFailed(p, m, "input", err)
		}
		return true
	}
	if m.passthrough {
		return false
	}

	switch {
	case miss != nil && miss.Reason != "":
		p.s.Send(message.System(miss.Reason))
	case len(m.forms.All()) > 0:
		var patterns []string
		for _, form := range m.forms.All()[0].Forms {
			if !slices.Contains(patterns, form.Pattern.Source) {
				patterns = append(patterns, form.Pattern.Source)
			}
		}
		p.s.Send(message.System("Huh? You can type:\n  [c]" + strings.Join(patterns, "[x]\n  [c]") + "[x]"))
	case trimmed != "":
		p.s.Send(message.System("Huh?"))
	}

	return true
}

// pushMode puts the mode name on top of p's stack and enters it. If
// entering fails, the mode is taken off again.
func (g *Game) pushMode(ctx context.Context, p *player, name string, state map[string]any) error {
	m, err := g.lookupMode(name)
	if err != nil {
		return err
	}

	f := &frame{name: name, state: state}
	p.modes = append(p.modes, f)

	if fn, ok := m.handlers["enter"]; ok {
		if err := g.runMode(ctx, p, f, fn, f.state); err != nil {
			p.remove(f)
			return fmt.Errorf("entering the %s mode (from %s): %w", name, m.owners["enter"], err)
		}
	}

	return nil
}

// popMode takes the top mode off p's stack and passes result to the mode
// below, if it has a resume handler.
func (g *Game) popMode(ctx context.Context, p *player, result any) error {
	f := p.top()
	if f == nil || f.login != nil {
		return errors.New("the session isn't in a mode; pop_mode ends a mode started with push_mode")
	}
	p.remove(f)

	var errs []error
	if err := g.leave(ctx, p, f, leaveDone); err != nil {
		errs = append(errs, err)
	}

	switch below := p.top(); {
	case below != nil && below.login == nil && below.running > 0:
		below.resumes = append(below.resumes, result)
	case below != nil && below.login == nil:
		if err := g.resume(ctx, p, below, result); err != nil {
			errs = append(errs, err)
		}
	case below == nil && p.character == nil && !p.closing:
		g.log.Error("a mode ended without choosing a character", "mode", f.name)
		p.s.Send(message.System(fmt.Sprintf("[R]The %s mode ended without choosing a character, so there's nothing left to play. (The %s mode must finish with session:play(character).)[x]",
			f.name, g.charactersMode())))
		p.s.Close()
	}

	return errors.Join(errs...)
}

// resume runs f's resume handler, if it has one, with what the mode above
// it ended with.
func (g *Game) resume(ctx context.Context, p *player, f *frame, result any) error {
	m, ok := g.modes[f.name]
	if !ok {
		return nil
	}
	fn, ok := m.handlers["resume"]
	if !ok {
		return nil
	}

	if err := g.runMode(ctx, p, f, fn, f.state, result); err != nil {
		return fmt.Errorf("resuming the %s mode (from %s): %w", m.name, m.owners["resume"], err)
	}

	return nil
}

// replaceMode swaps the top mode for the mode name, without resuming the
// mode below.
func (g *Game) replaceMode(ctx context.Context, p *player, name string, state map[string]any) error {
	if _, err := g.lookupMode(name); err != nil {
		return err
	}

	if f := p.top(); f != nil && f.login == nil {
		p.remove(f)
		if err := g.leave(ctx, p, f, leaveReplaced); err != nil {
			return err
		}
	}

	return g.pushMode(ctx, p, name, state)
}

// clearModes takes every mode off p's stack, top first, telling each why.
func (g *Game) clearModes(ctx context.Context, p *player, reason string) {
	frames := p.modes
	p.modes = nil

	for _, f := range slices.Backward(frames) {
		if err := g.leave(ctx, p, f, reason); err != nil {
			g.log.Error("leaving a mode failed", "mode", f.name, "reason", reason, "error", err)
		}
	}
}

// leave runs f's leave handler, if it has one.
func (g *Game) leave(ctx context.Context, p *player, f *frame, reason string) error {
	m, ok := g.modes[f.name]
	if !ok || f.login != nil {
		return nil
	}
	fn, ok := m.handlers["leave"]
	if !ok {
		return nil
	}

	if err := g.runMode(ctx, p, f, fn, f.state, reason); err != nil {
		return fmt.Errorf("leaving the %s mode (from %s): %w", m.name, m.owners["leave"], err)
	}

	return nil
}

// runMode calls one of a mode's functions with the session first. If it
// returns a table and the mode is still on the stack, that's the mode's
// new state.
func (g *Game) runMode(ctx context.Context, p *player, f *frame, fn scripting.Function, args ...any) error {
	f.running++
	result, err := fn.Call(ctx, append([]any{g.sessionHandle(p)}, args...)...)
	f.running--

	if err == nil && result != nil && p.onStack(f) {
		var state map[string]any
		if state, err = modeState(result); err != nil {
			err = fmt.Errorf("%w. Return the mode's state table to change it, or nothing to keep it", err)
		} else {
			f.state = state
		}
	}

	// Resume with what modes above ended with while this ran.
	for f.running == 0 && len(f.resumes) > 0 && p.top() == f {
		result := f.resumes[0]
		f.resumes = f.resumes[1:]
		if rerr := g.resume(ctx, p, f, result); rerr != nil {
			err = errors.Join(err, rerr)
		}
	}

	return err
}

// modeFailed reports a mode handler that failed.
func (g *Game) modeFailed(p *player, m *mode, handler string, err error) {
	g.log.Error("mode failed", "mode", m.name, "handler", handler, "plugin", m.owners[handler], "error", err)
	p.s.Send(message.System(fmt.Sprintf("[R]The %s mode's %s (from %s) failed: %v[x]", m.name, handler, m.owners[handler], err)))
}

// remove takes f off p's stack.
func (p *player) remove(f *frame) {
	p.modes = slices.DeleteFunc(p.modes, func(other *frame) bool { return other == f })
}

// modeState checks a value scripts gave as a mode's state: a table of plain
// data, so it survives a reload.
func modeState(value any) (map[string]any, error) {
	switch v := value.(type) {
	case nil:
		return map[string]any{}, nil
	case map[string]any:
		return v, checkState(v, "state")
	case []any:
		if len(v) == 0 {
			return map[string]any{}, nil
		}
	}

	return nil, fmt.Errorf("a mode's state must be a table, not a %s", scripting.TypeName(value))
}

func checkState(value any, path string) error {
	switch v := value.(type) {
	case scripting.Function:
		return fmt.Errorf("%s is a function. A mode's state must be plain data so it survives a reload; store what the function needs, or put the function in a mode", path)
	case []any:
		for i, item := range v {
			if err := checkState(item, fmt.Sprintf("%s[%d]", path, i+1)); err != nil {
				return err
			}
		}
	case map[string]any:
		for _, key := range slices.Sorted(maps.Keys(v)) {
			if err := checkState(v[key], path+"."+key); err != nil {
				return err
			}
		}
	}

	return nil
}

// pruneModes drops modes a reload removed from every player's stack.
func (g *Game) pruneModes(ctx context.Context) {
	for _, p := range g.players {
		var dropped []string
		p.modes = slices.DeleteFunc(p.modes, func(f *frame) bool {
			if _, ok := g.modes[f.name]; ok || f.login != nil {
				return false
			}
			dropped = append(dropped, f.name)
			return true
		})
		if len(dropped) == 0 {
			continue
		}

		g.log.Warn("reload removed modes a player was in", "modes", dropped, "account", p.account.Name)
		p.s.Send(message.System("[Y]The game was updated, and what you were doing has ended.[x]"))
		if len(p.modes) == 0 && p.character == nil {
			g.chooseCharacter(ctx, p)
		}
	}
}
