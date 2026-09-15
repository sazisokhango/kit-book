package store

import (
	"path/filepath"
	"testing"
)

// TestOpenAppliesSchemaAndPings is the Sprint Zero hello-world test for the
// storage container: schema applies and the connection is live.
func TestOpenAppliesSchemaAndPings(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "kitbook_test.db")

	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer s.Close()

	if err := s.Ping(); err != nil {
		t.Fatalf("Ping() error = %v", err)
	}
}
