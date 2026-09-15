// Package store is kitbook's SQLite-backed persistence layer (U1 in
// unit-registry.md), per ADR-002 (embedded SQLite, pure-Go driver).
// Sprint Zero only establishes schema creation and a connectivity check;
// the item/booking CRUD primitives land in Sprint 1 (05-spec/units/u1-data-model).
package store

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS items (
	item_id   TEXT PRIMARY KEY,
	item_type TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS bookings (
	booking_id            TEXT PRIMARY KEY,
	item_id               TEXT NOT NULL REFERENCES items(item_id),
	member_name           TEXT NOT NULL,
	expected_return_date  TEXT NOT NULL,
	checkout_at           TEXT NOT NULL,
	checkin_at            TEXT
);
`

// Store wraps the SQLite connection.
type Store struct {
	db *sql.DB
}

// Open opens (creating if absent) the SQLite database at path and applies
// the schema. This is the Sprint Zero "database schema applied" and
// "connection works" criteria from sprint-zero-checklist.md.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	return &Store{db: db}, nil
}

// Ping runs a trivial query to prove the connection is live — the Sprint
// Zero "SELECT 1" criterion.
func (s *Store) Ping() error {
	var one int
	return s.db.QueryRow("SELECT 1").Scan(&one)
}

// Close closes the underlying database connection.
func (s *Store) Close() error {
	return s.db.Close()
}
