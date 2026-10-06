package game

import (
	"context"
	"errors"
	"fmt"
	"html"
	"maps"
	"slices"
	"strings"

	"bbuck.dev/dragon-mud/ansi"
	"bbuck.dev/dragon-mud/command"
	"bbuck.dev/dragon-mud/message"
	"bbuck.dev/dragon-mud/scripting"
	"bbuck.dev/dragon-mud/session"
	"bbuck.dev/dragon-mud/store"
	"bbuck.dev/dragon-mud/world"
)

var errClosing = errors.New("the session is disconnecting")

// sessionType is how scripts see a connected session. Input modes are
// called with one; game.session(o) finds the session playing o.
//
//	s.account                    the logged-in account, or nil
//	s.character                  the character being played, or nil
//	s.mode                       the mode input goes to first, or nil
//
//	s:send(text)                 send text
//	s:send(kind, data[, block])  send a message kind
//	s:prompt(text)               ask for input
//	s:prompt(options)            ask with options: text, or view, data and
//	                             block; choices, a list of answers players
//	                             can also give by number; and secret, to
//	                             hide what they type
//	s:push_mode(name[, state])   start a mode on top of the current one
//	s:pop_mode([result])         end the top mode, passing result to the
//	                             resume handler of the mode below
//	s:replace_mode(name[, state]) swap the top mode for another
//	s:play(character)            enter the game as one of the account's
//	                             characters, ending every mode
//	s:close([text])              send an optional farewell and disconnect
//	s:push(name[, data])         send an event to the web client's code,
//	                             such as a plugin's JavaScript
func (g *Game) makeSessionType() *scripting.Type {
	return &scripting.Type{
		Name: "session",
		Fields: map[string]scripting.Field{
			"account": func(key any) (any, error) {
				p, err := g.sessionPlayer(key)
				if err != nil || p.account.ID == "" {
					return nil, err
				}
				return scripting.Handle{Type: g.accountType, Key: p.account.ID}, nil
			},
			"character": func(key any) (any, error) {
				p, err := g.sessionPlayer(key)
				if err != nil {
					return nil, err
				}
				return g.handle(p.character), nil
			},
			"mode": func(key any) (any, error) {
				p, err := g.sessionPlayer(key)
				if err != nil {
					return nil, err
				}
				if f := p.top(); f != nil {
					return f.name, nil
				}
				return nil, nil
			},
		},
		Methods: map[string]scripting.Method{
			"send":         g.mutating(g.sessionSend),
			"prompt":       g.mutating(g.sessionPrompt),
			"push_mode":    g.mutating(g.sessionPushMode),
			"pop_mode":     g.mutating(g.sessionPopMode),
			"replace_mode": g.mutating(g.sessionReplaceMode),
			"play":         g.mutating(g.sessionPlay),
			"close":        g.mutating(g.sessionClose),
			"push":         g.mutating(g.sessionPush),
		},
		String: func(key any) string {
			if p, ok := g.players[key.(session.ID)]; ok && p.account.Name != "" {
				return fmt.Sprintf("session %d (%s)", key, p.account.Name)
			}
			return fmt.Sprintf("session %d", key)
		},
	}
}

// accountType is how scripts see a logged-in account.
//
//	a.name                       the account's name
//	a.characters                 list of its characters
//
//	a:add_character(o)           make o one of its characters
func (g *Game) makeAccountType() *scripting.Type {
	return &scripting.Type{
		Name: "account",
		Fields: map[string]scripting.Field{
			"name": func(key any) (any, error) {
				account, err := g.loggedIn(key)
				if err != nil {
					return nil, err
				}
				return account.Name, nil
			},
			"characters": func(key any) (any, error) {
				ids, err := g.characters(key)
				if err != nil {
					return nil, err
				}
				characters := []any{}
				for _, id := range ids {
					if o, ok := g.world.Get(id); ok {
						characters = append(characters, g.handle(o))
					}
				}
				return characters, nil
			},
		},
		Methods: map[string]scripting.Method{
			"add_character": g.mutating(g.accountAddCharacter),
		},
		String: func(key any) string {
			if account, err := g.loggedIn(key); err == nil {
				return "account " + account.Name
			}
			return "account (logged out)"
		},
	}
}

// arg returns argument i, or nil when it's missing.
func arg(args scripting.Args, i int) any {
	if i < 0 || i >= args.Len() {
		return nil
	}

	return args[i]
}

// sessionHandle returns the script handle for p's session.
func (g *Game) sessionHandle(p *player) scripting.Handle {
	return scripting.Handle{Type: g.sessionType, Key: p.s.ID()}
}

// sessionPlayer returns the player a session handle's key refers to.
func (g *Game) sessionPlayer(key any) (*player, error) {
	p, ok := g.players[key.(session.ID)]
	if !ok {
		return nil, errors.New("the session has disconnected")
	}

	return p, nil
}

// loggedIn returns the account an account handle's key refers to, while
// someone is logged in to it.
func (g *Game) loggedIn(key any) (store.Account, error) {
	for _, p := range g.players {
		if p.account.ID == key.(string) {
			return p.account, nil
		}
	}

	return store.Account{}, errors.New("no one is logged in to that account anymore")
}

// characters returns the ids of an account's characters.
func (g *Game) characters(key any) ([]world.ID, error) {
	if _, err := g.loggedIn(key); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), saveTimeout)
	defer cancel()

	return g.store.Characters(ctx, key.(string))
}

func (g *Game) accountAddCharacter(key any, args scripting.Args) (any, error) {
	o, err := g.objectArg(args, 0)
	if err != nil {
		return nil, err
	}
	ids, err := g.characters(key)
	if err != nil || slices.Contains(ids, o.ID()) {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), saveTimeout)
	defer cancel()

	// The object must be saved before the account can own it.
	g.save(ctx)

	return nil, g.store.AddCharacter(ctx, key.(string), o.ID())
}

func (g *Game) sessionSend(key any, args scripting.Args) (any, error) {
	p, err := g.sessionPlayer(key)
	if err != nil {
		return nil, err
	}
	m, _, err := g.outgoing(args)
	if err != nil {
		return nil, err
	}

	p.s.Send(m)

	return nil, nil
}

// promptKeys are the options s:prompt takes.
var promptKeys = []string{"text", "view", "data", "block", "choices", "secret"}

func (g *Game) sessionPrompt(key any, args scripting.Args) (any, error) {
	p, err := g.sessionPlayer(key)
	if err != nil {
		return nil, err
	}

	var opts map[string]any
	switch v := arg(args, 0).(type) {
	case string:
		opts = map[string]any{"text": v}
	case map[string]any:
		opts = v
	default:
		return nil, fmt.Errorf("argument #1: expected the prompt's text, or a table like { text = \"Choose:\", choices = { ... } }, got %s", scripting.TypeName(v))
	}
	for _, k := range slices.Sorted(maps.Keys(opts)) {
		if !slices.Contains(promptKeys, k) {
			return nil, fmt.Errorf("prompt has an unknown option %q.%s Options: %s.", k, command.DidYouMean(k, promptKeys), strings.Join(promptKeys, ", "))
		}
	}

	choices, err := promptChoices(opts["choices"])
	if err != nil {
		return nil, err
	}
	top := p.top()
	if len(choices) > 0 && (top == nil || top.login != nil) {
		return nil, errors.New("choices need an input mode to answer them. Push a mode with session:push_mode(name) and prompt from its enter handler")
	}

	m, err := g.promptMessage(opts, choices)
	if err != nil {
		return nil, err
	}
	if secret, ok := opts["secret"]; ok {
		b, ok := secret.(bool)
		if !ok {
			return nil, fmt.Errorf("prompt: secret must be true or false, not a %s", scripting.TypeName(secret))
		}
		m.Secret = b
	}

	if top != nil && top.login == nil {
		top.choices = choices
	}
	p.s.Send(m)

	return nil, nil
}

// promptMessage renders a prompt: the text with its choices listed, or a
// view, whose template shows the choices itself.
func (g *Game) promptMessage(opts map[string]any, choices []string) (message.Message, error) {
	text, hasText := opts["text"]
	name, hasView := opts["view"]

	switch {
	case hasText && hasView:
		return message.Message{}, errors.New("prompt has both text and view. Give text for a plain prompt, or view and data to render a view")
	case hasView:
		view, ok := name.(string)
		if !ok {
			return message.Message{}, fmt.Errorf("prompt: view must be a view's name, not a %s", scripting.TypeName(name))
		}
		data := map[string]any{}
		if raw, ok := opts["data"]; ok && raw != nil {
			if data, ok = raw.(map[string]any); !ok {
				if list, isList := raw.([]any); !isList || len(list) > 0 {
					return message.Message{}, fmt.Errorf("prompt: data must be a table, not a %s", scripting.TypeName(raw))
				}
				data = map[string]any{}
			}
		}
		block, _ := opts["block"].(string)
		return g.render(view, data, block)
	case hasText:
		s, ok := text.(string)
		if !ok {
			return message.Message{}, fmt.Errorf("prompt: text must be a string, not a %s", scripting.TypeName(text))
		}
		if _, ok := opts["data"]; ok {
			return message.Message{}, errors.New("prompt: data needs a view to render it; give view = \"...\" instead of text")
		}
		return textPrompt(s, choices), nil
	default:
		return message.Message{}, errors.New("prompt needs text, or a view and data, like session:prompt({ text = \"Choose:\", choices = { ... } })")
	}
}

func promptChoices(raw any) ([]string, error) {
	if raw == nil {
		return nil, nil
	}
	list, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("prompt: choices must be a list of strings like { \"yes\", \"no\" }, not a %s", scripting.TypeName(raw))
	}

	choices := make([]string, len(list))
	for i, item := range list {
		s, ok := item.(string)
		if !ok || strings.TrimSpace(s) == "" {
			return nil, fmt.Errorf("prompt: choice #%d must be a non-empty string, not %v", i+1, item)
		}
		choices[i] = s
	}

	return choices, nil
}

// textPrompt is a plain prompt. Its choices are numbered in text and
// clickable on the web.
func textPrompt(text string, choices []string) message.Message {
	m := message.Message{Kind: message.KindPrompt, Text: text}
	if len(choices) == 0 {
		return m
	}

	var t, h strings.Builder
	t.WriteString(text)
	h.WriteString(strings.ReplaceAll(ansi.HTML(text), "\n", "<br>"))
	h.WriteString(`<ol class="choices">`)
	for i, c := range choices {
		fmt.Fprintf(&t, "\n  %d. %s", i+1, c)
		fmt.Fprintf(&h, `<li><dragon-choice value="%s">%s</dragon-choice></li>`, html.EscapeString(c), ansi.HTML(c))
	}
	h.WriteString("</ol>")
	m.Text, m.HTML = t.String(), h.String()

	return m
}

// stateArg returns argument i as a mode's state.
func stateArg(args scripting.Args, i int) (map[string]any, error) {
	state, err := modeState(arg(args, i))
	if err != nil {
		return nil, fmt.Errorf("argument #%d: %w", i+1, err)
	}

	return state, nil
}

func (g *Game) sessionPushMode(key any, args scripting.Args) (any, error) {
	p, name, state, err := g.modeArgs(key, args)
	if err != nil {
		return nil, err
	}

	return nil, g.pushMode(context.Background(), p, name, state)
}

func (g *Game) sessionReplaceMode(key any, args scripting.Args) (any, error) {
	p, name, state, err := g.modeArgs(key, args)
	if err != nil {
		return nil, err
	}

	return nil, g.replaceMode(context.Background(), p, name, state)
}

func (g *Game) modeArgs(key any, args scripting.Args) (*player, string, map[string]any, error) {
	p, err := g.sessionPlayer(key)
	if err != nil {
		return nil, "", nil, err
	}
	if p.closing {
		return nil, "", nil, errClosing
	}
	name, err := args.String(0)
	if err != nil {
		return nil, "", nil, err
	}
	if _, err := g.lookupMode(name); err != nil {
		return nil, "", nil, err
	}
	state, err := stateArg(args, 1)

	return p, name, state, err
}

func (g *Game) sessionPopMode(key any, args scripting.Args) (any, error) {
	p, err := g.sessionPlayer(key)
	if err != nil {
		return nil, err
	}
	if err := checkState(arg(args, 0), "result"); err != nil {
		return nil, err
	}

	return nil, g.popMode(context.Background(), p, arg(args, 0))
}

func (g *Game) sessionPlay(key any, args scripting.Args) (any, error) {
	p, err := g.sessionPlayer(key)
	if err != nil {
		return nil, err
	}
	if p.closing {
		return nil, errClosing
	}
	o, err := g.objectArg(args, 0)
	if err != nil {
		return nil, err
	}

	return nil, g.play(context.Background(), p, o)
}

func (g *Game) sessionClose(key any, args scripting.Args) (any, error) {
	p, err := g.sessionPlayer(key)
	if err != nil {
		return nil, err
	}

	if args.Len() > 0 && args[0] != nil {
		text, err := args.String(0)
		if err != nil {
			return nil, err
		}
		p.s.Send(message.Text(text))
	}
	p.s.Close()

	return nil, nil
}

// chooseCharacter starts the characters mode for a logged-in player. If it
// can't start, the player can't do anything, so they're disconnected.
func (g *Game) chooseCharacter(ctx context.Context, p *player) {
	if err := g.pushMode(ctx, p, g.charactersMode(), map[string]any{}); err != nil {
		g.log.Error("starting the characters mode failed", "account", p.account.Name, "error", err)
		p.s.Send(message.System(fmt.Sprintf("[R]Something went wrong choosing your character: %v[x]", err)))
		p.s.Close()
	}
}

// play puts p into the game as character, one of its account's
// characters, ending every mode it was in.
func (g *Game) play(ctx context.Context, p *player, character *world.Object) error {
	if p.account.ID == "" {
		return errors.New("only a logged-in session can play a character")
	}
	ids, err := g.characters(p.account.ID)
	if err != nil {
		return err
	}
	if !slices.Contains(ids, character.ID()) {
		return fmt.Errorf("%s isn't one of %s's characters. Add it first with session.account:add_character(character)", character, p.account.Name)
	}
	if p.character == character {
		return nil
	}

	// Playing a character takes it over from any other connection.
	takeover := false
	for _, other := range g.players {
		if other != p && other.character == character {
			other.s.Send(message.System("[Y]You have connected from somewhere else.[x]"))
			other.character = nil
			other.s.Close()
			takeover = true
		}
	}

	g.clearModes(ctx, p, leavePlaying)

	if p.character != nil {
		g.notify(ctx, notifyDisconnected, map[string]any{"actor": g.handle(p.character)})
	}
	p.character = character
	name := p.displayName()

	g.log.Info("player arrived", "name", name, "account", p.account.Name)
	p.s.Send(message.System(fmt.Sprintf("Welcome, [W]%s[x]! Type [c]help[x] to see what you can do.", name)))
	g.notify(ctx, notifyConnected, map[string]any{
		"actor":       g.handle(character),
		"reconnected": takeover,
	})

	return nil
}
