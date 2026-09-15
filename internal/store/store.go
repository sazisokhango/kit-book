// Package store is kitbook's SQLite-backed persistence layer (U1 in
// unit-registry.md), per ADR-002 (embedded SQLite, pure-Go driver).
package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "modernc.org/sqlite"

	"bitbucket.org/psybergate/kitbook/internal/kitbookerrors"
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

const timeLayout = time.RFC3339

// Item is the ItemRecord DTO (05-spec/units/u1-data-model/spec.md §3).
type Item struct {
	ItemID   string
	ItemType string
}

// Booking is the BookingRecord DTO (05-spec/units/u1-data-model/spec.md §3).
type Booking struct {
	BookingID          string
	ItemID             string
	MemberName         string
	ExpectedReturnDate string
	CheckoutAt         time.Time
	CheckinAt          *time.Time // nil while checked out
}

// Store wraps the SQLite connection.
type Store struct {
	db *sql.DB
}

// Open opens (creating if absent) the SQLite database at path and applies
// the schema.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	if _, err := db.Exec(schema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	return &Store{db: db}, nil
}

// Ping runs a trivial query to prove the connection is live.
func (s *Store) Ping() error {
	var one int
	return s.db.QueryRow("SELECT 1").Scan(&one)
}

// Close closes the underlying database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// InsertItem adds an item to the catalogue (U2 seeds through this).
func (s *Store) InsertItem(itemID, itemType string) error {
	_, err := s.db.Exec(`INSERT INTO items (item_id, item_type) VALUES (?, ?)`, itemID, itemType)
	if err != nil {
		return fmt.Errorf("insert item: %w", err)
	}
	return nil
}

// GetItem looks up an item by id.
func (s *Store) GetItem(itemID string) (Item, error) {
	var it Item
	err := s.db.QueryRow(`SELECT item_id, item_type FROM items WHERE item_id = ?`, itemID).
		Scan(&it.ItemID, &it.ItemType)
	if errors.Is(err, sql.ErrNoRows) {
		return Item{}, &kitbookerrors.ItemNotFoundError{ItemID: itemID}
	}
	if err != nil {
		return Item{}, fmt.Errorf("get item: %w", err)
	}
	return it, nil
}

// GetOpenBooking returns the open (not yet checked in) booking for itemID,
// or nil if the item is currently available.
func (s *Store) GetOpenBooking(itemID string) (*Booking, error) {
	row := s.db.QueryRow(`
		SELECT booking_id, item_id, member_name, expected_return_date, checkout_at
		FROM bookings
		WHERE item_id = ? AND checkin_at IS NULL`, itemID)

	var b Booking
	var checkoutAt string
	err := row.Scan(&b.BookingID, &b.ItemID, &b.MemberName, &b.ExpectedReturnDate, &checkoutAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get open booking: %w", err)
	}
	b.CheckoutAt, err = time.Parse(timeLayout, checkoutAt)
	if err != nil {
		return nil, fmt.Errorf("parse checkout_at: %w", err)
	}
	return &b, nil
}

// GetBooking looks up a booking by id, regardless of open/closed state.
func (s *Store) GetBooking(bookingID string) (*Booking, error) {
	row := s.db.QueryRow(`
		SELECT booking_id, item_id, member_name, expected_return_date, checkout_at, checkin_at
		FROM bookings
		WHERE booking_id = ?`, bookingID)

	var b Booking
	var checkoutAt string
	var checkinAt sql.NullString
	err := row.Scan(&b.BookingID, &b.ItemID, &b.MemberName, &b.ExpectedReturnDate, &checkoutAt, &checkinAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, &kitbookerrors.BookingNotFoundError{BookingID: bookingID}
	}
	if err != nil {
		return nil, fmt.Errorf("get booking: %w", err)
	}
	b.CheckoutAt, err = time.Parse(timeLayout, checkoutAt)
	if err != nil {
		return nil, fmt.Errorf("parse checkout_at: %w", err)
	}
	if checkinAt.Valid {
		t, err := time.Parse(timeLayout, checkinAt.String)
		if err != nil {
			return nil, fmt.Errorf("parse checkin_at: %w", err)
		}
		b.CheckinAt = &t
	}
	return &b, nil
}

// InsertBooking creates a new open booking. Callers (core.Checkout) are
// responsible for the no-double-booking and service-due checks before
// calling this — the store layer only persists.
func (s *Store) InsertBooking(b Booking) error {
	_, err := s.db.Exec(`
		INSERT INTO bookings (booking_id, item_id, member_name, expected_return_date, checkout_at, checkin_at)
		VALUES (?, ?, ?, ?, ?, NULL)`,
		b.BookingID, b.ItemID, b.MemberName, b.ExpectedReturnDate, b.CheckoutAt.Format(timeLayout))
	if err != nil {
		return fmt.Errorf("insert booking: %w", err)
	}
	return nil
}

// CloseBooking sets checkin_at on an open booking. Returns
// BookingAlreadyClosedError if the booking exists but is already checked in.
func (s *Store) CloseBooking(bookingID string, checkinAt time.Time) error {
	b, err := s.GetBooking(bookingID)
	if err != nil {
		return err
	}
	if b.CheckinAt != nil {
		return &kitbookerrors.BookingAlreadyClosedError{BookingID: bookingID}
	}
	_, err = s.db.Exec(`UPDATE bookings SET checkin_at = ? WHERE booking_id = ?`,
		checkinAt.Format(timeLayout), bookingID)
	if err != nil {
		return fmt.Errorf("close booking: %w", err)
	}
	return nil
}

// ListBookingHistory returns every booking for itemID, oldest checkout_at
// first (U6). Returns ItemNotFoundError if the item itself doesn't exist.
func (s *Store) ListBookingHistory(itemID string) ([]Booking, error) {
	if _, err := s.GetItem(itemID); err != nil {
		return nil, err
	}

	rows, err := s.db.Query(`
		SELECT booking_id, item_id, member_name, expected_return_date, checkout_at, checkin_at
		FROM bookings
		WHERE item_id = ?
		ORDER BY checkout_at ASC`, itemID)
	if err != nil {
		return nil, fmt.Errorf("list booking history: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var history []Booking
	for rows.Next() {
		var b Booking
		var checkoutAt string
		var checkinAt sql.NullString
		if err := rows.Scan(&b.BookingID, &b.ItemID, &b.MemberName, &b.ExpectedReturnDate, &checkoutAt, &checkinAt); err != nil {
			return nil, fmt.Errorf("scan booking history: %w", err)
		}
		b.CheckoutAt, err = time.Parse(timeLayout, checkoutAt)
		if err != nil {
			return nil, fmt.Errorf("parse checkout_at: %w", err)
		}
		if checkinAt.Valid {
			t, err := time.Parse(timeLayout, checkinAt.String)
			if err != nil {
				return nil, fmt.Errorf("parse checkin_at: %w", err)
			}
			b.CheckinAt = &t
		}
		history = append(history, b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list booking history: %w", err)
	}
	return history, nil
}

// ItemStatusRow is the StatusRow DTO (05-spec/units/u5-status-board/spec.md §3).
//
// BookingID is an addition beyond the spec's original field list: without
// it, the booking id printed once by `checkout` is unrecoverable if lost,
// making `checkin <booking-id>` (U4) impossible to use in practice. Surfaced
// during manual HITL testing of the running CLI.
type ItemStatusRow struct {
	ItemID    string
	Status    string // AVAILABLE | CHECKED_OUT | OVERDUE
	Holder    string // empty when AVAILABLE
	DueBack   string // empty when AVAILABLE
	BookingID string // empty when AVAILABLE — the id to pass to `checkin`
}

// overdueThreshold is the 48h window from 02-discovery/discovery-log.md
// (meeting notes: "members return kit within 48h of a callout").
const overdueThreshold = 48 * time.Hour

// ListItemStatus returns every item with its current status, computing
// OVERDUE when more than 48 hours have elapsed since checkout with no
// checkin (U5).
func (s *Store) ListItemStatus(now time.Time) ([]ItemStatusRow, error) {
	rows, err := s.db.Query(`
		SELECT i.item_id, b.booking_id, b.member_name, b.expected_return_date, b.checkout_at
		FROM items i
		LEFT JOIN bookings b ON b.item_id = i.item_id AND b.checkin_at IS NULL
		ORDER BY i.item_id`)
	if err != nil {
		return nil, fmt.Errorf("list item status: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var result []ItemStatusRow
	for rows.Next() {
		var itemID string
		var bookingID, memberName, expectedReturn, checkoutAt sql.NullString
		if err := rows.Scan(&itemID, &bookingID, &memberName, &expectedReturn, &checkoutAt); err != nil {
			return nil, fmt.Errorf("scan item status: %w", err)
		}
		row := ItemStatusRow{ItemID: itemID, Status: "AVAILABLE"}
		if bookingID.Valid {
			row.BookingID = bookingID.String
			row.Holder = memberName.String
			row.DueBack = expectedReturn.String
			row.Status = "CHECKED_OUT"
			checkoutTime, err := time.Parse(timeLayout, checkoutAt.String)
			if err != nil {
				return nil, fmt.Errorf("parse checkout_at: %w", err)
			}
			if now.Sub(checkoutTime) > overdueThreshold {
				row.Status = "OVERDUE"
			}
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list item status: %w", err)
	}
	return result, nil
}
