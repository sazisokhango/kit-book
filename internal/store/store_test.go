package store

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"bitbucket.org/psybergate/kitbook/internal/kitbookerrors"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "kitbook_test.db")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestOpenAppliesSchemaAndPings(t *testing.T) {
	s := openTestStore(t)
	if err := s.Ping(); err != nil {
		t.Fatalf("Ping() error = %v", err)
	}
}

// Scenario: Item lookup by id
func TestInsertItemAndGetItem(t *testing.T) {
	s := openTestStore(t)

	if err := s.InsertItem("ROPE-04", "rope"); err != nil {
		t.Fatalf("InsertItem() error = %v", err)
	}

	got, err := s.GetItem("ROPE-04")
	if err != nil {
		t.Fatalf("GetItem() error = %v", err)
	}
	if got.ItemType != "rope" {
		t.Fatalf("GetItem().ItemType = %q, want %q", got.ItemType, "rope")
	}
}

func TestGetItem_NotFound(t *testing.T) {
	s := openTestStore(t)

	_, err := s.GetItem("ROPE-99")
	if _, ok := err.(*kitbookerrors.ItemNotFoundError); !ok {
		t.Fatalf("GetItem() error = %v, want *kitbookerrors.ItemNotFoundError", err)
	}
}

func TestGetOpenBooking_NoneReturnsNil(t *testing.T) {
	s := openTestStore(t)
	if err := s.InsertItem("ROPE-04", "rope"); err != nil {
		t.Fatalf("InsertItem() error = %v", err)
	}

	b, err := s.GetOpenBooking("ROPE-04")
	if err != nil {
		t.Fatalf("GetOpenBooking() error = %v", err)
	}
	if b != nil {
		t.Fatalf("GetOpenBooking() = %+v, want nil", b)
	}
}

func TestInsertBookingAndGetOpenBooking(t *testing.T) {
	s := openTestStore(t)
	if err := s.InsertItem("ROPE-04", "rope"); err != nil {
		t.Fatalf("InsertItem() error = %v", err)
	}

	checkoutAt := time.Now()
	err := s.InsertBooking(Booking{
		BookingID:          "KB-1",
		ItemID:             "ROPE-04",
		MemberName:         "Tayob",
		ExpectedReturnDate: "2026-09-20",
		CheckoutAt:         checkoutAt,
	})
	if err != nil {
		t.Fatalf("InsertBooking() error = %v", err)
	}

	got, err := s.GetOpenBooking("ROPE-04")
	if err != nil {
		t.Fatalf("GetOpenBooking() error = %v", err)
	}
	if got == nil {
		t.Fatal("GetOpenBooking() = nil, want an open booking")
	}
	if got.BookingID != "KB-1" || got.MemberName != "Tayob" {
		t.Fatalf("GetOpenBooking() = %+v, want booking_id=KB-1 member=Tayob", got)
	}
}

// Scenario: Check in an open booking
func TestCloseBooking(t *testing.T) {
	s := openTestStore(t)
	if err := s.InsertItem("ROPE-04", "rope"); err != nil {
		t.Fatalf("InsertItem() error = %v", err)
	}
	if err := s.InsertBooking(Booking{
		BookingID: "KB-1001", ItemID: "ROPE-04", MemberName: "Tayob",
		ExpectedReturnDate: "2026-09-20", CheckoutAt: time.Now(),
	}); err != nil {
		t.Fatalf("InsertBooking() error = %v", err)
	}

	if err := s.CloseBooking("KB-1001", time.Now()); err != nil {
		t.Fatalf("CloseBooking() error = %v", err)
	}

	got, err := s.GetBooking("KB-1001")
	if err != nil {
		t.Fatalf("GetBooking() error = %v", err)
	}
	if got.CheckinAt == nil {
		t.Fatal("GetBooking().CheckinAt = nil, want non-nil")
	}

	if b, err := s.GetOpenBooking("ROPE-04"); err != nil || b != nil {
		t.Fatalf("GetOpenBooking() after checkin = (%+v, %v), want (nil, nil)", b, err)
	}
}

// Scenario: Reject an unknown booking id
func TestCloseBooking_NotFound(t *testing.T) {
	s := openTestStore(t)

	err := s.CloseBooking("KB-9999", time.Now())
	if _, ok := err.(*kitbookerrors.BookingNotFoundError); !ok {
		t.Fatalf("CloseBooking() error = %v, want *kitbookerrors.BookingNotFoundError", err)
	}
}

func TestCloseBooking_AlreadyClosed(t *testing.T) {
	s := openTestStore(t)
	if err := s.InsertItem("ROPE-04", "rope"); err != nil {
		t.Fatalf("InsertItem() error = %v", err)
	}
	if err := s.InsertBooking(Booking{
		BookingID: "KB-1001", ItemID: "ROPE-04", MemberName: "Tayob",
		ExpectedReturnDate: "2026-09-20", CheckoutAt: time.Now(),
	}); err != nil {
		t.Fatalf("InsertBooking() error = %v", err)
	}
	if err := s.CloseBooking("KB-1001", time.Now()); err != nil {
		t.Fatalf("first CloseBooking() error = %v", err)
	}

	err := s.CloseBooking("KB-1001", time.Now())
	if _, ok := err.(*kitbookerrors.BookingAlreadyClosedError); !ok {
		t.Fatalf("CloseBooking() error = %v, want *kitbookerrors.BookingAlreadyClosedError", err)
	}
}

// Scenario: List history oldest-first
func TestListBookingHistory_OldestFirst(t *testing.T) {
	s := openTestStore(t)
	if err := s.InsertItem("ROPE-04", "rope"); err != nil {
		t.Fatalf("InsertItem() error = %v", err)
	}

	base := time.Now().Add(-72 * time.Hour)
	for i, member := range []string{"Tayob", "Nkosinathi", "Chris"} {
		checkoutAt := base.Add(time.Duration(i) * time.Hour)
		if err := s.InsertBooking(Booking{
			BookingID: fmt.Sprintf("KB-%d", i), ItemID: "ROPE-04", MemberName: member,
			ExpectedReturnDate: "2026-09-20", CheckoutAt: checkoutAt,
		}); err != nil {
			t.Fatalf("InsertBooking() error = %v", err)
		}
		if err := s.CloseBooking(fmt.Sprintf("KB-%d", i), checkoutAt.Add(time.Hour)); err != nil {
			t.Fatalf("CloseBooking() error = %v", err)
		}
	}

	history, err := s.ListBookingHistory("ROPE-04")
	if err != nil {
		t.Fatalf("ListBookingHistory() error = %v", err)
	}
	if len(history) != 3 {
		t.Fatalf("ListBookingHistory() returned %d rows, want 3", len(history))
	}
	if history[0].MemberName != "Tayob" || history[2].MemberName != "Chris" {
		t.Fatalf("ListBookingHistory() not oldest-first: %+v", history)
	}
}

func TestListBookingHistory_ItemNotFound(t *testing.T) {
	s := openTestStore(t)

	_, err := s.ListBookingHistory("ROPE-99")
	if _, ok := err.(*kitbookerrors.ItemNotFoundError); !ok {
		t.Fatalf("ListBookingHistory() error = %v, want *kitbookerrors.ItemNotFoundError", err)
	}
}

// Scenario: Overdue checkout is flagged / Available item is not flagged
func TestListItemStatus(t *testing.T) {
	s := openTestStore(t)
	if err := s.InsertItem("ROPE-04", "rope"); err != nil {
		t.Fatalf("InsertItem(ROPE-04) error = %v", err)
	}
	if err := s.InsertItem("RADIO-11", "radio"); err != nil {
		t.Fatalf("InsertItem(RADIO-11) error = %v", err)
	}

	now := time.Now()
	overdueCheckout := now.Add(-50 * time.Hour)
	if err := s.InsertBooking(Booking{
		BookingID: "KB-1", ItemID: "ROPE-04", MemberName: "Tayob",
		ExpectedReturnDate: "2026-09-20", CheckoutAt: overdueCheckout,
	}); err != nil {
		t.Fatalf("InsertBooking() error = %v", err)
	}

	rows, err := s.ListItemStatus(now)
	if err != nil {
		t.Fatalf("ListItemStatus() error = %v", err)
	}

	byID := map[string]ItemStatusRow{}
	for _, r := range rows {
		byID[r.ItemID] = r
	}

	if got := byID["ROPE-04"].Status; got != "OVERDUE" {
		t.Fatalf(`ROPE-04 status = %q, want "OVERDUE"`, got)
	}
	// Without the booking id in the status row, a lost checkout receipt
	// makes `checkin <booking-id>` impossible to use.
	if got := byID["ROPE-04"].BookingID; got != "KB-1" {
		t.Fatalf(`ROPE-04 BookingID = %q, want "KB-1"`, got)
	}
	if got := byID["RADIO-11"].BookingID; got != "" {
		t.Fatalf(`RADIO-11 BookingID = %q, want "" (available item)`, got)
	}
	if got := byID["RADIO-11"].Status; got != "AVAILABLE" {
		t.Fatalf(`RADIO-11 status = %q, want "AVAILABLE"`, got)
	}
}
