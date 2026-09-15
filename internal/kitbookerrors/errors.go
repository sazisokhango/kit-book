// Package kitbookerrors holds kitbook's named business exceptions (one per
// exception listed across the 05-spec/units/ specs' Interface Contract
// sections). Each error's Error() text matches the exact message its spec's
// Gherkin scenario asserts on — these are user-facing strings, not just
// debug text.
package kitbookerrors

import "fmt"

// ItemNotFoundError — U1: item_id not found in the items table.
type ItemNotFoundError struct{ ItemID string }

func (e *ItemNotFoundError) Error() string {
	return fmt.Sprintf("item not found: %s", e.ItemID)
}

// ItemAlreadyCheckedOutError — U3: an open booking already exists for item_id.
type ItemAlreadyCheckedOutError struct{ ItemID string }

func (e *ItemAlreadyCheckedOutError) Error() string {
	return "item already checked out"
}

// ItemServiceDueError — U3/U7: the item is flagged service-due.
type ItemServiceDueError struct{ ItemID string }

func (e *ItemServiceDueError) Error() string {
	return "item is service-due, no exceptions"
}

// BookingNotFoundError — U4: booking_id does not exist.
type BookingNotFoundError struct{ BookingID string }

func (e *BookingNotFoundError) Error() string {
	return "booking not found"
}

// BookingAlreadyClosedError — U4: booking_id exists but is already checked in.
type BookingAlreadyClosedError struct{ BookingID string }

func (e *BookingAlreadyClosedError) Error() string {
	return "booking already checked in"
}

// ServiceDueFileUnavailableError — U7: the configured file path does not
// exist or is not readable. Fail-closed: never treat "can't verify" as
// "safe to checkout".
type ServiceDueFileUnavailableError struct{}

func (e *ServiceDueFileUnavailableError) Error() string {
	return "service-due file unavailable — checkout blocked for safety"
}

// ServiceDueFileFormatError — U7: the file exists but fails to parse.
type ServiceDueFileFormatError struct{}

func (e *ServiceDueFileFormatError) Error() string {
	return "service-due file unreadable — checkout blocked for safety"
}

// InvalidCatalogueFormatError — U2: a seed file row's item_id does not match
// the "<TYPE>-<NN>" pattern, or the file is otherwise malformed.
type InvalidCatalogueFormatError struct{ Reason string }

func (e *InvalidCatalogueFormatError) Error() string {
	return fmt.Sprintf("invalid catalogue file: %s", e.Reason)
}
