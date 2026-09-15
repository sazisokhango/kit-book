// Package catalogue implements U2 (05-spec/units/u2-catalogue-seed/spec.md):
// bulk-loading the item catalogue from a CSV file into U1's items table.
package catalogue

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"bitbucket.org/psybergate/kitbook/internal/kitbookerrors"
)

// itemIDPattern matches the catalogue's "<TYPE>-<NN>" id convention, e.g.
// "ROPE-04" (same convention U7's service-due file uses).
var itemIDPattern = regexp.MustCompile(`^[A-Z]+-[0-9]+$`)

// Row is one parsed catalogue entry, ready to insert via ItemInserter.
type Row struct {
	ItemID   string
	ItemType string
}

// ItemInserter is the subset of *store.Store the seed loader needs.
type ItemInserter interface {
	InsertItem(itemID, itemType string) error
}

// SeedResult is the SeedResult DTO (05-spec/units/u2-catalogue-seed/spec.md §3).
type SeedResult struct {
	ItemsLoaded int
}

// Seed reads a CSV file (header "item_id,item_type") and inserts every row
// into store. It validates every item_id against the catalogue's
// "<TYPE>-<NN>" convention before inserting anything — a single malformed
// row rejects the whole file rather than partially loading it.
func Seed(store ItemInserter, filePath string) (SeedResult, error) {
	rows, err := parse(filePath)
	if err != nil {
		return SeedResult{}, err
	}

	for _, r := range rows {
		if err := store.InsertItem(r.ItemID, r.ItemType); err != nil {
			return SeedResult{}, fmt.Errorf("insert item %s: %w", r.ItemID, err)
		}
	}
	return SeedResult{ItemsLoaded: len(rows)}, nil
}

func parse(filePath string) ([]Row, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("open catalogue file: %w", err)
	}
	defer func() { _ = f.Close() }()

	r := csv.NewReader(f)
	header, err := r.Read()
	if err != nil {
		return nil, &kitbookerrors.InvalidCatalogueFormatError{Reason: "missing header row"}
	}
	if len(header) < 2 || strings.TrimSpace(header[0]) != "item_id" || strings.TrimSpace(header[1]) != "item_type" {
		return nil, &kitbookerrors.InvalidCatalogueFormatError{Reason: `header must be "item_id,item_type"`}
	}

	var rows []Row
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, &kitbookerrors.InvalidCatalogueFormatError{Reason: err.Error()}
		}
		if len(record) < 2 {
			return nil, &kitbookerrors.InvalidCatalogueFormatError{Reason: "row has fewer than 2 columns"}
		}
		itemID := strings.TrimSpace(record[0])
		itemType := strings.TrimSpace(record[1])
		if !itemIDPattern.MatchString(itemID) {
			return nil, &kitbookerrors.InvalidCatalogueFormatError{Reason: fmt.Sprintf("item_id %q does not match <TYPE>-<NN>", itemID)}
		}
		rows = append(rows, Row{ItemID: itemID, ItemType: itemType})
	}
	return rows, nil
}
