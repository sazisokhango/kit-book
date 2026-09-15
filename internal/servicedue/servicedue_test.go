package servicedue

import (
	"os"
	"path/filepath"
	"testing"

	"bitbucket.org/psybergate/kitbook/internal/kitbookerrors"
)

func writeFile(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "service-due.txt")
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}
	return path
}

// Scenario: Item listed as service-due is blocked
func TestIsServiceDue_ListedItemReturnsTrue(t *testing.T) {
	path := writeFile(t, "RADIO-11\n")

	c, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !c.IsServiceDue("RADIO-11") {
		t.Fatal("IsServiceDue(RADIO-11) = false, want true")
	}
}

// Scenario: Item not listed is not blocked
func TestIsServiceDue_UnlistedItemReturnsFalse(t *testing.T) {
	path := writeFile(t, "RADIO-11\n")

	c, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if c.IsServiceDue("ROPE-04") {
		t.Fatal("IsServiceDue(ROPE-04) = true, want false")
	}
}

// Scenario: Missing file fails closed
func TestLoad_MissingFileFailsClosed(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "does-not-exist.txt"))
	var wantErr *kitbookerrors.ServiceDueFileUnavailableError
	if err == nil {
		t.Fatal("Load() error = nil, want ServiceDueFileUnavailableError")
	}
	if _, ok := err.(*kitbookerrors.ServiceDueFileUnavailableError); !ok {
		t.Fatalf("Load() error = %T, want %T", err, wantErr)
	}
}

// Scenario: Malformed file fails closed
func TestLoad_MalformedFileFailsClosed(t *testing.T) {
	path := writeFile(t, "not a valid item id\n")

	_, err := Load(path)
	if err == nil {
		t.Fatal("Load() error = nil, want ServiceDueFileFormatError")
	}
	if _, ok := err.(*kitbookerrors.ServiceDueFileFormatError); !ok {
		t.Fatalf("Load() error = %T, want *kitbookerrors.ServiceDueFileFormatError", err)
	}
}

func TestLoad_BlankLinesAndCommentsIgnored(t *testing.T) {
	path := writeFile(t, "\n# comment\nRADIO-11\n\n")

	c, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !c.IsServiceDue("RADIO-11") {
		t.Fatal("IsServiceDue(RADIO-11) = false, want true")
	}
}
