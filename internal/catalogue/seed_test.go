package catalogue

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"bitbucket.org/psybergate/kitbook/internal/kitbookerrors"
)

type fakeInserter struct {
	inserted []Row
}

func (f *fakeInserter) InsertItem(itemID, itemType string) error {
	f.inserted = append(f.inserted, Row{ItemID: itemID, ItemType: itemType})
	return nil
}

func writeCSV(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "catalogue.csv")
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}
	return path
}

// Scenario: Seed a fresh catalogue
func TestSeed_LoadsAllValidRows(t *testing.T) {
	var b strings.Builder
	b.WriteString("item_id,item_type\n")
	for i := 1; i <= 60; i++ {
		fmt.Fprintf(&b, "ROPE-%02d,rope\n", i)
	}
	path := writeCSV(t, b.String())

	ins := &fakeInserter{}
	result, err := Seed(ins, path)
	if err != nil {
		t.Fatalf("Seed() error = %v", err)
	}
	if result.ItemsLoaded != 60 {
		t.Fatalf("Seed().ItemsLoaded = %d, want 60", result.ItemsLoaded)
	}
	if len(ins.inserted) != 60 {
		t.Fatalf("inserted %d rows, want 60", len(ins.inserted))
	}
}

// Scenario: Reject a malformed row
func TestSeed_RejectsMalformedItemID(t *testing.T) {
	path := writeCSV(t, "item_id,item_type\nROPE-04,rope\nnot-an-id,rope\n")

	ins := &fakeInserter{}
	_, err := Seed(ins, path)
	if _, ok := err.(*kitbookerrors.InvalidCatalogueFormatError); !ok {
		t.Fatalf("Seed() error = %v, want *kitbookerrors.InvalidCatalogueFormatError", err)
	}
	if len(ins.inserted) != 0 {
		t.Fatalf("inserted %d rows before rejecting, want 0 (reject before any insert)", len(ins.inserted))
	}
}

func TestSeed_RejectsWrongHeader(t *testing.T) {
	path := writeCSV(t, "id,type\nROPE-04,rope\n")

	ins := &fakeInserter{}
	_, err := Seed(ins, path)
	if _, ok := err.(*kitbookerrors.InvalidCatalogueFormatError); !ok {
		t.Fatalf("Seed() error = %v, want *kitbookerrors.InvalidCatalogueFormatError", err)
	}
}

func TestSeed_MissingFile(t *testing.T) {
	ins := &fakeInserter{}
	_, err := Seed(ins, filepath.Join(t.TempDir(), "missing.csv"))
	if err == nil {
		t.Fatal("Seed() error = nil, want an error for a missing file")
	}
}
