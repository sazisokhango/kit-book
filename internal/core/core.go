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
	ListBookingHistory(itemID string) ([]store.Booking, error)
}

// ServiceDueChecker is the subset of *servicedue.Checker core depends on.
type ServiceDueChecker interface {
	IsServiceDue(itemID string) bool
}

// Service wires the storage layer together with U3/U4/U5/U6's business
// rules. The service-due checker (U7) is deliberately NOT loaded here —
// only Checkout depends on it (per u5/u6's specs, which declare "Depends
// On: U1" only), so loading it eagerly for every command would make
// `status`/`history` fail on a missing service-due file they never read.
// See Checkout, which loads it lazily and caches the result.
type Service struct {
	Store          Store
	ServiceDue     ServiceDueChecker // set once Checkout has loaded it; nil until then
	ServiceDuePath string
	Now            func() time.Time // overridable in tests
}

// NewService constructs a Service against an already-open store. The
// service-due file at servicePath is not read until the first Checkout call.
func NewService(st *store.Store, servicePath string) *Service {
	return &Service{Store: st, ServiceDuePath: servicePath, Now: time.Now}
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
// available and not service-due, or fails explicitly with the reason. The
// service-due file (U7) is loaded on first use, not at Service construction
// — see the Service doc comment.
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

	if s.ServiceDue == nil {
		checker, err := servicedue.Load(s.ServiceDuePath)
		if err != nil {
			return "", err
		}
		s.ServiceDue = checker
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

// History implements U6: lists every booking for itemID, oldest first.
func (s *Service) History(itemID string) ([]store.Booking, error) {
	return s.Store.ListBookingHistory(itemID)
}

// StatusRow is the StatusRow DTO (05-spec/units/u5-status-board/spec.md §3).
type StatusRow = store.ItemStatusRow

// Status implements U5: lists every item with its current status, computing
// OVERDUE when a checkout has passed 48 hours.
func (s *Service) Status() ([]StatusRow, error) {
	return s.Store.ListItemStatus(s.now())
}
