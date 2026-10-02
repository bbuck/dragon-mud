// Package store persists the game to SQLite. The database is the source of
// truth: the game loads every object at startup and saves what changed after
// each event, in one transaction. See docs/design.md §8.
package store

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"

	"bbuck.dev/dragon-mud/world"
)

// migrations bring the schema up to date. Each runs once, in order; the
// database's user_version records how many have run. Never edit one that
// has shipped; add a new one.
var migrations = []string{
	`
	CREATE TABLE objects (
		id       TEXT PRIMARY KEY,
		name     TEXT UNIQUE,
		parent   TEXT REFERENCES objects (id) DEFERRABLE INITIALLY DEFERRED,
		location TEXT REFERENCES objects (id) DEFERRABLE INITIALLY DEFERRED
	) WITHOUT ROWID;

	CREATE TABLE properties (
		object TEXT NOT NULL REFERENCES objects (id) ON DELETE CASCADE,
		name   TEXT NOT NULL,
		value  TEXT NOT NULL, -- JSON
		PRIMARY KEY (object, name)
	) WITHOUT ROWID;
	`,
	`
	CREATE TABLE accounts (
		id            TEXT PRIMARY KEY,
		name          TEXT NOT NULL UNIQUE COLLATE NOCASE,
		password_hash TEXT NOT NULL,
		created       TEXT NOT NULL
	) WITHOUT ROWID;

	CREATE TABLE characters (
		account TEXT NOT NULL REFERENCES accounts (id) ON DELETE CASCADE,
		object  TEXT NOT NULL UNIQUE REFERENCES objects (id) ON DELETE CASCADE,
		PRIMARY KEY (account, object)
	);
	`,
}

// Store is a game's database.
type Store struct {
	db *sql.DB
}

// Open opens the database at path, creating it and its directory if needed,
// and brings its schema up to date.
func Open(ctx context.Context, path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}

	pragmas := url.Values{"_pragma": {"foreign_keys(1)", "journal_mode(WAL)", "busy_timeout(5000)"}}
	db, err := sql.Open("sqlite", "file:"+path+"?"+pragmas.Encode())
	if err != nil {
		return nil, err
	}
	// The game loop is the only writer.
	db.SetMaxOpenConns(1)

	s := &Store{db: db}
	if err := s.migrate(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	return s, nil
}

// Close closes the database.
func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate(ctx context.Context) error {
	var version int
	if err := s.db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return err
	}
	if version > len(migrations) {
		return fmt.Errorf("database schema version %d is newer than this engine (%d); upgrade dragon", version, len(migrations))
	}

	for i := version; i < len(migrations); i++ {
		err := s.inTx(ctx, func(tx *sql.Tx) error {
			if _, err := tx.ExecContext(ctx, migrations[i]); err != nil {
				return err
			}
			_, err := tx.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", i+1))
			return err
		})
		if err != nil {
			return fmt.Errorf("migration %d: %w", i+1, err)
		}
	}

	return nil
}

// Load reads every object.
func (s *Store) Load(ctx context.Context) ([]world.Record, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, coalesce(name, ''), coalesce(parent, ''), coalesce(location, '')
		FROM objects ORDER BY id`)
	if err != nil {
		return nil, err
	}

	var records []world.Record
	index := make(map[world.ID]int)
	for rows.Next() {
		var r world.Record
		if err := rows.Scan(&r.ID, &r.Name, &r.Parent, &r.Location); err != nil {
			rows.Close()
			return nil, err
		}
		r.Properties = make(map[string]any)
		index[r.ID] = len(records)
		records = append(records, r)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}

	rows, err = s.db.QueryContext(ctx, `SELECT object, name, value FROM properties`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			id         world.ID
			name, data string
		)
		if err := rows.Scan(&id, &name, &data); err != nil {
			return nil, err
		}

		value, err := decode(data)
		if err != nil {
			return nil, fmt.Errorf("object %s property %q: %w", id, name, err)
		}
		records[index[id]].Properties[name] = value
	}

	return records, rows.Err()
}

// Save writes changes in one transaction.
func (s *Store) Save(ctx context.Context, changes world.Changes) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		for _, id := range changes.Destroyed {
			if _, err := tx.ExecContext(ctx, `DELETE FROM objects WHERE id = ?`, id); err != nil {
				return err
			}
		}

		// Names are unique and SQLite checks that immediately, so clear them
		// first in case two saved objects swapped names.
		for _, r := range changes.Saved {
			if _, err := tx.ExecContext(ctx, `UPDATE objects SET name = NULL WHERE id = ?`, r.ID); err != nil {
				return err
			}
		}

		for _, r := range changes.Saved {
			if err := saveRecord(ctx, tx, r); err != nil {
				return fmt.Errorf("object %s: %w", r.ID, err)
			}
		}

		return nil
	})
}

func saveRecord(ctx context.Context, tx *sql.Tx, r world.Record) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO objects (id, name, parent, location) VALUES (?, ?, ?, ?)
		ON CONFLICT (id) DO UPDATE SET
			name = excluded.name, parent = excluded.parent, location = excluded.location`,
		r.ID, nullable(string(r.Name)), nullable(string(r.Parent)), nullable(string(r.Location)))
	if err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM properties WHERE object = ?`, r.ID); err != nil {
		return err
	}

	for name, value := range r.Properties {
		data, err := json.Marshal(value)
		if err != nil {
			return fmt.Errorf("property %q: %w", name, err)
		}

		_, err = tx.ExecContext(ctx, `INSERT INTO properties (object, name, value) VALUES (?, ?, ?)`, r.ID, name, string(data))
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *Store) inTx(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	if err := fn(tx); err != nil {
		return errors.Join(err, tx.Rollback())
	}

	return tx.Commit()
}

func nullable(s string) any {
	if s == "" {
		return nil
	}

	return s
}

// decode parses a JSON property value. Whole numbers become int64 and other
// numbers float64, matching world.Normalize.
func decode(data string) (any, error) {
	d := json.NewDecoder(bytes.NewReader([]byte(data)))
	d.UseNumber()

	var value any
	if err := d.Decode(&value); err != nil {
		return nil, err
	}

	return numbers(value)
}

func numbers(value any) (any, error) {
	switch v := value.(type) {
	case json.Number:
		if !strings.ContainsAny(string(v), ".eE") {
			if i, err := v.Int64(); err == nil {
				return i, nil
			}
		}
		return v.Float64()
	case []any:
		for i, item := range v {
			n, err := numbers(item)
			if err != nil {
				return nil, err
			}
			v[i] = n
		}
	case map[string]any:
		for key, item := range v {
			n, err := numbers(item)
			if err != nil {
				return nil, err
			}
			v[key] = n
		}
	}

	return value, nil
}
