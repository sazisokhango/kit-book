// Package core holds kitbook's domain logic: checkout (U3), checkin (U4),
// and status board (U5), against the specs in 05-spec/units/.
package core

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"bitbucket.org/psybergate/kitbook/internal/kitbookerrors"
	"bitbucket.org/psybergate/kitbook/internal/servicedue"
	"bitbucket.org/psybergate/kitbook/internal/store"
)

// Ping is the Sprint Zero hello-world for the domain-logic container.
func Ping() string {
	return "ok"
}

// Store is the subset of *store.Store that core depends on — narrowed for
// testability (see core_test.go, which can wrap a real *store.Store or a
// fake without changing this interface).
type Store interface {
	GetItem(itemID string) (store.Item, error)
	GetOpenBooking(itemID string) (*store.Booking, error)
	InsertBooking(b store.Booking) error
	CloseBooking(bookingID string, checkinAt time.Time) error
	ListItemStatus(now time.Time) ([]store.ItemStatusRow, error)
}

// ServiceDueChecker is the subset of *servicedue.Checker core depends on.
type ServiceDueChecker interface {
	IsServiceDue(itemID string) bool
}

// Service wires the storage layer and the service-due checker together to
// implement U3/U4/U5's business rules.
type Service struct {
	Store       Store
	ServiceDue  ServiceDueChecker
	Now         func() time.Time // overridable in tests
}

// NewService constructs a Service. servicePath is the path to the
// service-due data file (U7); it is loaded fresh on construction so a
// changed file is picked up on the next command invocation (kitbook is a
// short-lived CLI process, not a long-running daemon).
func NewService(st *store.Store, servicePath string) (*Service, error) {
	checker, err := servicedue.Load(servicePath)
	if err != nil {
		return nil, err
	}
	return &Service{Store: st, ServiceDue: checker, Now: time.Now}, nil
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

// newBookingID generates a "KB-" prefixed booking id (05-spec/units/u1-data-model/spec.md §3).
func newBookingID() (string, error) {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate booking id: %w", err)
	}
	return "KB-" + hex.EncodeToString(b), nil
}

// Checkout implements U3: records a checkout after verifying the item is
// available and not service-due, or fails explicitly with the reason.
func (s *Service) Checkout(itemID, memberName, expectedReturnDate string) (string, error) {
	if _, err := s.Store.GetItem(itemID); err != nil {
		return "", err
	}

	existing, err := s.Store.GetOpenBooking(itemID)
	if err != nil {
		return "", err
	}
	if existing != nil {
		return "", &kitbookerrors.ItemAlreadyCheckedOutError{ItemID: itemID}
	}

	if s.ServiceDue.IsServiceDue(itemID) {
		return "", &kitbookerrors.ItemServiceDueError{ItemID: itemID}
	}

	bookingID, err := newBookingID()
	if err != nil {
		return "", err
	}

	err = s.Store.InsertBooking(store.Booking{
		BookingID:          bookingID,
		ItemID:             itemID,
		MemberName:         memberName,
		ExpectedReturnDate: expectedReturnDate,
		CheckoutAt:         s.now(),
	})
	if err != nil {
		return "", err
	}
	return bookingID, nil
}

// Checkin implements U4: closes an open booking by booking id.
func (s *Service) Checkin(bookingID string) error {
	return s.Store.CloseBooking(bookingID, s.now())
}

// StatusRow is the StatusRow DTO (05-spec/units/u5-status-board/spec.md §3).
type StatusRow = store.ItemStatusRow

// Status implements U5: lists every item with its current status, computing
// OVERDUE when a checkout has passed 48 hours.
func (s *Service) Status() ([]StatusRow, error) {
	return s.Store.ListItemStatus(s.now())
}
