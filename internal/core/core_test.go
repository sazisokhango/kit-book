package core

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"bitbucket.org/psybergate/kitbook/internal/kitbookerrors"
	"bitbucket.org/psybergate/kitbook/internal/store"
)

func TestPing(t *testing.T) {
	got := Ping()
	want := "ok"
	if got != want {
		t.Fatalf("Ping() = %q, want %q", got, want)
	}
}

// fakeStore is an in-memory Store double, isolating core's business rules
// (no-double-booking, service-due, OVERDUE) from the real SQLite layer,
// which is covered separately in internal/store.
type fakeStore struct {
	items    map[string]store.Item
	bookings map[string]store.Booking
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		items:    map[string]store.Item{},
		bookings: map[string]store.Booking{},
	}
}

func (f *fakeStore) GetItem(itemID string) (store.Item, error) {
	it, ok := f.items[itemID]
	if !ok {
		return store.Item{}, &kitbookerrors.ItemNotFoundError{ItemID: itemID}
	}
	return it, nil
}

func (f *fakeStore) GetOpenBooking(itemID string) (*store.Booking, error) {
	for _, b := range f.bookings {
		if b.ItemID == itemID && b.CheckinAt == nil {
			bCopy := b
			return &bCopy, nil
		}
	}
	return nil, nil
}

func (f *fakeStore) InsertBooking(b store.Booking) error {
	f.bookings[b.BookingID] = b
	return nil
}

func (f *fakeStore) CloseBooking(bookingID string, checkinAt time.Time) error {
	b, ok := f.bookings[bookingID]
	if !ok {
		return &kitbookerrors.BookingNotFoundError{BookingID: bookingID}
	}
	if b.CheckinAt != nil {
		return &kitbookerrors.BookingAlreadyClosedError{BookingID: bookingID}
	}
	b.CheckinAt = &checkinAt
	f.bookings[bookingID] = b
	return nil
}

func (f *fakeStore) ListBookingHistory(itemID string) ([]store.Booking, error) {
	if _, ok := f.items[itemID]; !ok {
		return nil, &kitbookerrors.ItemNotFoundError{ItemID: itemID}
	}
	var history []store.Booking
	for _, b := range f.bookings {
		if b.ItemID == itemID {
			history = append(history, b)
		}
	}
	return history, nil
}

func (f *fakeStore) ListItemStatus(now time.Time) ([]store.ItemStatusRow, error) {
	var rows []store.ItemStatusRow
	for id := range f.items {
		row := store.ItemStatusRow{ItemID: id, Status: "AVAILABLE"}
		for _, b := range f.bookings {
			if b.ItemID == id && b.CheckinAt == nil {
				row.Status = "CHECKED_OUT"
				row.Holder = b.MemberName
				row.DueBack = b.ExpectedReturnDate
				if now.Sub(b.CheckoutAt) > 48*time.Hour {
					row.Status = "OVERDUE"
				}
			}
		}
		rows = append(rows, row)
	}
	return rows, nil
}

type fakeServiceDue struct {
	due map[string]bool
}

func (f *fakeServiceDue) IsServiceDue(itemID string) bool {
	return f.due[itemID]
}

func newTestService(fs *fakeStore, sd *fakeServiceDue) *Service {
	return &Service{Store: fs, ServiceDue: sd, Now: time.Now}
}

// Scenario: Happy path checkout
func TestCheckout_HappyPath(t *testing.T) {
	fs := newFakeStore()
	fs.items["ROPE-04"] = store.Item{ItemID: "ROPE-04", ItemType: "rope"}
	svc := newTestService(fs, &fakeServiceDue{})

	bookingID, err := svc.Checkout("ROPE-04", "Tayob", "2026-09-20")
	if err != nil {
		t.Fatalf("Checkout() error = %v", err)
	}
	if len(bookingID) < 4 || bookingID[:3] != "KB-" {
		t.Fatalf("Checkout() bookingID = %q, want KB-* pattern", bookingID)
	}
}

// Scenario: Reject an already checked-out item
func TestCheckout_AlreadyCheckedOut(t *testing.T) {
	fs := newFakeStore()
	fs.items["ROPE-04"] = store.Item{ItemID: "ROPE-04", ItemType: "rope"}
	svc := newTestService(fs, &fakeServiceDue{})

	if _, err := svc.Checkout("ROPE-04", "Tayob", "2026-09-20"); err != nil {
		t.Fatalf("first Checkout() error = %v", err)
	}

	_, err := svc.Checkout("ROPE-04", "Nkosinathi", "2026-09-21")
	wantMsg := "item already checked out"
	if err == nil || err.Error() != wantMsg {
		t.Fatalf("Checkout() error = %v, want %q", err, wantMsg)
	}
}

// Scenario: Reject a service-due item
func TestCheckout_ServiceDue(t *testing.T) {
	fs := newFakeStore()
	fs.items["RADIO-11"] = store.Item{ItemID: "RADIO-11", ItemType: "radio"}
	svc := newTestService(fs, &fakeServiceDue{due: map[string]bool{"RADIO-11": true}})

	_, err := svc.Checkout("RADIO-11", "Tayob", "2026-09-20")
	wantMsg := "item is service-due, no exceptions"
	if err == nil || err.Error() != wantMsg {
		t.Fatalf("Checkout() error = %v, want %q", err, wantMsg)
	}
}

func TestCheckout_ItemNotFound(t *testing.T) {
	fs := newFakeStore()
	svc := newTestService(fs, &fakeServiceDue{})

	_, err := svc.Checkout("ROPE-99", "Tayob", "2026-09-20")
	if _, ok := err.(*kitbookerrors.ItemNotFoundError); !ok {
		t.Fatalf("Checkout() error = %v, want *kitbookerrors.ItemNotFoundError", err)
	}
}

// Scenario: Check in an open booking
func TestCheckin_HappyPath(t *testing.T) {
	fs := newFakeStore()
	fs.items["ROPE-04"] = store.Item{ItemID: "ROPE-04", ItemType: "rope"}
	svc := newTestService(fs, &fakeServiceDue{})

	bookingID, err := svc.Checkout("ROPE-04", "Tayob", "2026-09-20")
	if err != nil {
		t.Fatalf("Checkout() error = %v", err)
	}

	if err := svc.Checkin(bookingID); err != nil {
		t.Fatalf("Checkin() error = %v", err)
	}

	b := fs.bookings[bookingID]
	if b.CheckinAt == nil {
		t.Fatal("Checkin() did not set CheckinAt")
	}
}

// Scenario: Reject an unknown booking id
func TestCheckin_BookingNotFound(t *testing.T) {
	fs := newFakeStore()
	svc := newTestService(fs, &fakeServiceDue{})

	err := svc.Checkin("KB-9999")
	wantMsg := "booking not found"
	if err == nil || err.Error() != wantMsg {
		t.Fatalf("Checkin() error = %v, want %q", err, wantMsg)
	}
}

// Scenario: Overdue checkout is flagged
func TestStatus_OverdueFlagged(t *testing.T) {
	fs := newFakeStore()
	fs.items["ROPE-04"] = store.Item{ItemID: "ROPE-04", ItemType: "rope"}
	svc := newTestService(fs, &fakeServiceDue{})

	fixedNow := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)
	svc.Now = func() time.Time { return fixedNow }

	checkoutAt := fixedNow.Add(-50 * time.Hour)
	fs.bookings["KB-1"] = store.Booking{
		BookingID: "KB-1", ItemID: "ROPE-04", MemberName: "Tayob",
		ExpectedReturnDate: "2026-09-16", CheckoutAt: checkoutAt,
	}

	rows, err := svc.Status()
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if len(rows) != 1 || rows[0].Status != "OVERDUE" {
		t.Fatalf("Status() = %+v, want one row with status OVERDUE", rows)
	}
}

// TestNewService_Integration exercises real wiring (SQLite store +
// service-due file), not the fakes above, so NewService itself is covered
// end-to-end rather than only through direct Service{} construction.
func TestNewService_Integration(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "kitbook.db")
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("store.Open() error = %v", err)
	}
	t.Cleanup(func() { st.Close() })

	if err := st.InsertItem("ROPE-04", "rope"); err != nil {
		t.Fatalf("InsertItem() error = %v", err)
	}

	svcDuePath := filepath.Join(t.TempDir(), "service-due.txt")
	if err := os.WriteFile(svcDuePath, []byte("RADIO-11\n"), 0o644); err != nil {
		t.Fatalf("write service-due file: %v", err)
	}

	svc := NewService(st, svcDuePath)

	bookingID, err := svc.Checkout("ROPE-04", "Tayob", "2026-09-20")
	if err != nil {
		t.Fatalf("Checkout() error = %v", err)
	}
	if err := svc.Checkin(bookingID); err != nil {
		t.Fatalf("Checkin() error = %v", err)
	}
}

// Checkout genuinely needs the service-due file and fails closed without
// one; Status/History have no such dependency (per their specs' own
// "Depends On: U1" line) and must keep working when the file is missing.
func TestNewService_StatusAndHistoryWorkWithoutServiceDueFile(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "kitbook.db")
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("store.Open() error = %v", err)
	}
	t.Cleanup(func() { st.Close() })
	if err := st.InsertItem("ROPE-04", "rope"); err != nil {
		t.Fatalf("InsertItem() error = %v", err)
	}

	svc := NewService(st, filepath.Join(t.TempDir(), "missing.txt"))

	if _, err := svc.Status(); err != nil {
		t.Fatalf("Status() error = %v, want nil (no service-due dependency)", err)
	}
	if _, err := svc.History("ROPE-04"); err != nil {
		t.Fatalf("History() error = %v, want nil (no service-due dependency)", err)
	}

	_, err = svc.Checkout("ROPE-04", "Tayob", "2026-09-20")
	if _, ok := err.(*kitbookerrors.ServiceDueFileUnavailableError); !ok {
		t.Fatalf("Checkout() error = %v, want *kitbookerrors.ServiceDueFileUnavailableError", err)
	}
}

// now() falls back to time.Now when Service.Now is left nil (e.g. a
// hand-built Service outside NewService).
func TestService_NowDefaultsWhenNil(t *testing.T) {
	fs := newFakeStore()
	fs.items["ROPE-04"] = store.Item{ItemID: "ROPE-04", ItemType: "rope"}
	svc := &Service{Store: fs, ServiceDue: &fakeServiceDue{}}

	before := time.Now()
	if _, err := svc.Checkout("ROPE-04", "Tayob", "2026-09-20"); err != nil {
		t.Fatalf("Checkout() error = %v", err)
	}
	after := time.Now()

	var booking store.Booking
	for _, b := range fs.bookings {
		booking = b
	}
	if booking.CheckoutAt.Before(before) || booking.CheckoutAt.After(after) {
		t.Fatalf("CheckoutAt = %v, want between %v and %v", booking.CheckoutAt, before, after)
	}
}

func TestHistory_ItemNotFound(t *testing.T) {
	fs := newFakeStore()
	svc := newTestService(fs, &fakeServiceDue{})

	_, err := svc.History("ROPE-99")
	if _, ok := err.(*kitbookerrors.ItemNotFoundError); !ok {
		t.Fatalf("History() error = %v, want *kitbookerrors.ItemNotFoundError", err)
	}
}

func TestHistory_ReturnsBookings(t *testing.T) {
	fs := newFakeStore()
	fs.items["ROPE-04"] = store.Item{ItemID: "ROPE-04", ItemType: "rope"}
	fs.bookings["KB-1"] = store.Booking{BookingID: "KB-1", ItemID: "ROPE-04", MemberName: "Tayob"}
	svc := newTestService(fs, &fakeServiceDue{})

	history, err := svc.History("ROPE-04")
	if err != nil {
		t.Fatalf("History() error = %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("History() returned %d rows, want 1", len(history))
	}
}

// Scenario: Available item is not flagged
func TestStatus_AvailableItem(t *testing.T) {
	fs := newFakeStore()
	fs.items["RADIO-11"] = store.Item{ItemID: "RADIO-11", ItemType: "radio"}
	svc := newTestService(fs, &fakeServiceDue{})

	rows, err := svc.Status()
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if len(rows) != 1 || rows[0].Status != "AVAILABLE" {
		t.Fatalf("Status() = %+v, want one row with status AVAILABLE", rows)
	}
}
