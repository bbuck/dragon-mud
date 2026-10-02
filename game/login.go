package game

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"bbuck.dev/dragon-mud/message"
	"bbuck.dev/dragon-mud/store"
	"bbuck.dev/dragon-mud/world"
)

const (
	minPasswordLength = 8
	maxPasswordLength = 1024
	maxLoginFailures  = 3
)

type loginStep int

const (
	stepName loginStep = iota
	stepPassword
	stepConfirmNew
	stepNewPassword
	stepRepeatPassword

	// stepWaiting is while a password is checked or hashed off the loop.
	stepWaiting
)

// login is a session's progress through logging in. It's the first,
// hardcoded case of a pending prompt (see docs/design.md §5).
type login struct {
	step     loginStep
	name     string
	account  store.Account
	password string // a new password waiting to be typed again
	failures int
}

// checkedEvent reports a password checked off the loop.
type checkedEvent struct {
	p   *player
	ok  bool
	err error
}

// rehashedEvent carries a stronger hash for an account whose stored hash
// used old parameters.
type rehashedEvent struct {
	accountID string
	hash      string
	err       error
}

// hashedEvent reports a new password hashed off the loop.
type hashedEvent struct {
	p    *player
	hash string
	err  error
}

func (g *Game) askName(p *player) {
	p.login = &login{step: stepName}
	p.s.Send(message.System("By what name shall we know you?"))
}

// login handles a line from a session that is logging in.
func (g *Game) login(ctx context.Context, p *player, line string) {
	l := p.login

	switch l.step {
	case stepName:
		name := strings.TrimSpace(line)
		if !playerNameRx.MatchString(name) {
			p.s.Send(message.System("Names are 2 to 20 letters. Try again:"))
			return
		}
		l.name = strings.ToUpper(name[:1]) + strings.ToLower(name[1:])

		account, ok, err := g.store.Account(ctx, l.name)
		if err != nil {
			g.loginFailed(p, "looking up account", err)
			return
		}
		if ok {
			l.account = account
			l.step = stepPassword
			p.s.Send(message.Secret("Password:"))
			return
		}

		l.step = stepConfirmNew
		p.s.Send(message.System(fmt.Sprintf("No one here is called [W]%s[x]. Create a new account? (yes/no)", l.name)))

	case stepPassword:
		l.step = stepWaiting
		accountID := l.account.ID
		g.hasher.Check(line, l.account.PasswordHash, func(ok, rehash bool, err error) {
			g.post(checkedEvent{p: p, ok: ok, err: err})
			if rehash {
				g.hasher.Hash(line, func(hash string, err error) {
					g.post(rehashedEvent{accountID: accountID, hash: hash, err: err})
				})
			}
		})

	case stepConfirmNew:
		switch strings.ToLower(strings.TrimSpace(line)) {
		case "y", "yes":
			l.step = stepNewPassword
			p.s.Send(message.Secret(fmt.Sprintf("Choose a password (at least %d characters):", minPasswordLength)))
		case "n", "no":
			g.askName(p)
		default:
			p.s.Send(message.System("Please answer yes or no."))
		}

	case stepNewPassword:
		switch {
		case len(line) < minPasswordLength:
			p.s.Send(message.Secret(fmt.Sprintf("Passwords need at least %d characters. Choose a password:", minPasswordLength)))
		case len(line) > maxPasswordLength:
			p.s.Send(message.Secret(fmt.Sprintf("Passwords can be at most %d characters. Choose a password:", maxPasswordLength)))
		default:
			l.password = line
			l.step = stepRepeatPassword
			p.s.Send(message.Secret("Type it again:"))
		}

	case stepRepeatPassword:
		if line != l.password {
			l.password = ""
			l.step = stepNewPassword
			p.s.Send(message.Secret("Those didn't match. Choose a password:"))
			return
		}

		l.password = ""
		l.step = stepWaiting
		g.hasher.Hash(line, func(hash string, err error) {
			g.post(hashedEvent{p: p, hash: hash, err: err})
		})

	case stepWaiting:
		// Input while a password is being checked is ignored.
	}
}

// checked finishes logging in to an existing account.
func (g *Game) checked(ctx context.Context, e checkedEvent) {
	p := e.p
	if !g.waiting(p) {
		return
	}
	l := p.login

	if e.err != nil {
		g.loginFailed(p, "checking password", e.err)
		return
	}

	if !e.ok {
		l.failures++
		g.log.Warn("wrong password", "account", l.account.Name, "failures", l.failures)
		if l.failures >= maxLoginFailures {
			p.s.Send(message.System("[R]Too many wrong passwords.[x]"))
			p.s.Close()
			return
		}

		l.step = stepPassword
		p.s.Send(message.Secret("Wrong password. Password:"))
		return
	}

	g.enter(ctx, p, l.account)
}

// hashed finishes creating an account.
func (g *Game) hashed(ctx context.Context, e hashedEvent) {
	p := e.p
	if !g.waiting(p) {
		return
	}

	if e.err != nil {
		g.loginFailed(p, "hashing password", e.err)
		return
	}

	account, err := g.store.CreateAccount(ctx, p.login.name, e.hash)
	if errors.Is(err, store.ErrNameTaken) {
		p.s.Send(message.System("Someone just took that name."))
		g.askName(p)
		return
	}
	if err != nil {
		g.loginFailed(p, "creating account", err)
		return
	}

	g.log.Info("account created", "account", account.Name)
	g.enter(ctx, p, account)
}

// rehashed stores a stronger hash for an account. Failing only means the
// old hash stays, so it's logged and otherwise ignored.
func (g *Game) rehashed(ctx context.Context, e rehashedEvent) {
	err := e.err
	if err == nil {
		err = g.store.SetPasswordHash(ctx, e.accountID, e.hash)
	}
	if err != nil {
		g.log.Error("updating password hash failed", "account", e.accountID, "error", err)
		return
	}

	g.log.Info("updated password hash", "account", e.accountID)
}

// waiting reports whether p is still connected and waiting on its password.
func (g *Game) waiting(p *player) bool {
	current, ok := g.players[p.s.ID()]

	return ok && current == p && p.login != nil && p.login.step == stepWaiting
}

// loginFailed reports an unexpected error and starts the login again.
func (g *Game) loginFailed(p *player, doing string, err error) {
	g.log.Error("login failed", "doing", doing, "error", err)
	p.s.Send(message.System("[R]Something went wrong on our end. Please try again.[x]"))
	g.askName(p)
}

// enter puts a logged-in player into the game as the account's character.
// For now every account has one character, named after it; choosing among
// several will be a pending prompt the game controls.
func (g *Game) enter(ctx context.Context, p *player, account store.Account) {
	character, err := g.character(ctx, account)
	if err != nil {
		g.loginFailed(p, "finding character", err)
		return
	}

	// Logging in again takes over the character from the old connection.
	takeover := false
	for _, other := range g.players {
		if other != p && other.character == character {
			other.s.Send(message.System("[Y]You have connected from somewhere else.[x]"))
			other.character = nil
			other.s.Close()
			takeover = true
		}
	}

	p.login = nil
	p.account = account
	p.character = character
	name := p.displayName()

	g.log.Info("player arrived", "name", name, "account", account.Name)
	p.s.Send(message.System(fmt.Sprintf("Welcome, [W]%s[x]! Type [c]help[x] to see what you can do.", name)))
	g.notify(ctx, "player_entered", map[string]any{
		"player":      g.handle(character),
		"reconnected": takeover,
	})

	if _, ok := g.commands.Lookup("look"); ok {
		g.dispatch(ctx, p, "look")
	}
}

// character returns the account's character, creating it if the account
// has none.
func (g *Game) character(ctx context.Context, account store.Account) (*world.Object, error) {
	ids, err := g.store.Characters(ctx, account.ID)
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		if o, ok := g.world.Get(id); ok {
			return o, nil
		}
	}

	o := g.world.Create()
	if err := o.Set("name", account.Name); err != nil {
		return nil, err
	}

	// The object must be saved before the account can own it.
	g.save(ctx)
	if err := g.store.AddCharacter(ctx, account.ID, o.ID()); err != nil {
		return nil, err
	}

	g.log.Info("character created", "name", account.Name, "id", o.ID())

	return o, nil
}
