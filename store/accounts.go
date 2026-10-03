package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"strings"
	"time"

	"bbuck.dev/dragon-mud/world"
)

// ErrNameTaken is returned when creating an account whose name is in use.
var ErrNameTaken = errors.New("account name is taken")

// Account is someone who can log in. Accounts are separate from characters:
// an account owns characters, which are objects in the world. How many it
// may have, and how a player picks one, is up to the game.
type Account struct {
	// ID is stable across export and import, like object ids.
	ID           string
	Name         string
	PasswordHash string
	Created      time.Time
}

// Account returns the account named name, ignoring case.
func (s *Store) Account(ctx context.Context, name string) (Account, bool, error) {
	var (
		a       Account
		created string
	)
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, password_hash, created FROM accounts WHERE name = ?`, name,
	).Scan(&a.ID, &a.Name, &a.PasswordHash, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return Account{}, false, nil
	}
	if err != nil {
		return Account{}, false, err
	}

	a.Created, err = time.Parse(time.RFC3339, created)

	return a, err == nil, err
}

// CreateAccount creates an account. It returns ErrNameTaken if another
// account has the name, ignoring case.
func (s *Store) CreateAccount(ctx context.Context, name, passwordHash string) (Account, error) {
	a := Account{
		ID:           accountID(),
		Name:         name,
		PasswordHash: passwordHash,
		Created:      time.Now().UTC().Truncate(time.Second),
	}

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO accounts (id, name, password_hash, created) VALUES (?, ?, ?, ?)`,
		a.ID, a.Name, a.PasswordHash, a.Created.Format(time.RFC3339),
	)
	if err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed: accounts.name") {
		return Account{}, ErrNameTaken
	}
	if err != nil {
		return Account{}, err
	}

	return a, nil
}

// Characters returns the ids of the account's characters, oldest first.
func (s *Store) Characters(ctx context.Context, accountID string) ([]world.ID, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT object FROM characters WHERE account = ? ORDER BY rowid`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []world.ID
	for rows.Next() {
		var id world.ID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}

	return ids, rows.Err()
}

// AddCharacter makes the object id one of the account's characters. The
// object must already be saved.
func (s *Store) AddCharacter(ctx context.Context, accountID string, id world.ID) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO characters (account, object) VALUES (?, ?)`, accountID, id)

	return err
}

func accountID() string {
	const alphabet = "0123456789abcdefghjkmnpqrstvwxyz"

	b := make([]byte, 12)
	rand.Read(b)
	for i := range b {
		b[i] = alphabet[int(b[i])%len(alphabet)]
	}

	return string(b)
}

// SetPasswordHash replaces the account's password hash.
func (s *Store) SetPasswordHash(ctx context.Context, accountID, passwordHash string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE accounts SET password_hash = ? WHERE id = ?`, passwordHash, accountID)

	return err
}

// IsCharacter reports whether some account owns the object id as a
// character.
func (s *Store) IsCharacter(ctx context.Context, id world.ID) (bool, error) {
	var found bool
	err := s.db.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM characters WHERE object = ?)`, id).Scan(&found)

	return found, err
}
