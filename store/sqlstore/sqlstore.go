// Package sqlstore provides a store.Store backed by a SQL key/value table
// via the stdlib database/sql interface. It takes a caller-provided *sql.DB
// and never opens or owns a connection — so shesha takes no SQL-driver
// dependency; the consumer brings its own driver and owns lifecycle/pragmas.
// It targets SQLite-compatible dialects (`?` placeholders, INSERT OR
// REPLACE); tested with modernc.org/sqlite. The adapter does NOT create or
// migrate the table — the consumer owns the schema (key TEXT PRIMARY KEY,
// value TEXT NOT NULL).
package sqlstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"

	"github.com/dangernoodle-io/shesha/store"
)

// validTableName matches a valid, unquoted SQL identifier: it must start
// with a letter or underscore, followed by any number of letters, digits,
// or underscores.
var validTableName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// New returns a store.Store reading/writing the given key/value table on
// db. table must be a valid SQL identifier ([A-Za-z_][A-Za-z0-9_]*); New
// returns an error for a nil db or an invalid/empty table name (the name is
// interpolated into SQL, not parameterizable, so it must be validated). New
// does not touch the database.
func New(db *sql.DB, table string) (store.Store, error) {
	if db == nil {
		return nil, errors.New("sqlstore: db must not be nil")
	}
	if !validTableName.MatchString(table) {
		return nil, fmt.Errorf("sqlstore: invalid table name %q", table)
	}

	return &sqlStore{db: db, table: table}, nil
}

type sqlStore struct {
	db    *sql.DB
	table string
}

var _ store.Store = (*sqlStore)(nil)

// Get returns the value for key, and whether it was found.
func (s *sqlStore) Get(ctx context.Context, key string) (string, bool, error) {
	query := fmt.Sprintf("SELECT value FROM %s WHERE key = ?", s.table)

	var value string
	if err := s.db.QueryRowContext(ctx, query, key).Scan(&value); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", false, nil
		}

		return "", false, err
	}

	return value, true, nil
}

// Load returns every key/value pair currently held in the table.
func (s *sqlStore) Load(ctx context.Context) (map[string]string, error) {
	query := fmt.Sprintf("SELECT key, value FROM %s", s.table)

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := make(map[string]string)
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}

		out[key] = value
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return out, nil
}

// Set writes value for key immediately (write-through): INSERT OR REPLACE
// so an existing key is overwritten.
func (s *sqlStore) Set(ctx context.Context, key, value string) error {
	query := fmt.Sprintf("INSERT OR REPLACE INTO %s (key, value) VALUES (?, ?)", s.table)

	_, err := s.db.ExecContext(ctx, query, key, value)

	return err
}

// Save is a no-op: Set is write-through (every write already hits the
// database immediately), so there is nothing buffered to flush.
func (s *sqlStore) Save(_ context.Context) error {
	return nil
}

// Delete removes key. Deleting an absent key is not an error.
func (s *sqlStore) Delete(ctx context.Context, key string) error {
	query := fmt.Sprintf("DELETE FROM %s WHERE key = ?", s.table)

	_, err := s.db.ExecContext(ctx, query, key)

	return err
}
