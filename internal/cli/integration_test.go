// Integration tests exercising the full CLI command tree end-to-end against
// a real SQLite database and a real service-due file — the same flow
// verified manually by HITL during the P6 demo, now automated (P7 exit
// gate: "Integration tests passing (all unit interactions verified)").
package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// run executes the kitbook CLI with args against dbPath/serviceDuePath and
// returns combined stdout and the error (if any). Each call builds a fresh
// command tree, matching how the real binary starts fresh per invocation.
func run(t *testing.T, dbPath, serviceDuePath string, args ...string) (string, error) {
	t.Helper()
	cmd := newRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	fullArgs := append([]string{"--db", dbPath, "--service-due-file", serviceDuePath}, args...)
	cmd.SetArgs(fullArgs)
	err := cmd.Execute()
	return out.String(), err
}

func writeFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// TestFullLifecycle mirrors the exact manual HITL demo flow: seed -> status
// -> checkout blocked without a service-due file -> checkout succeeds ->
// double-booking blocked -> service-due item blocked -> checkin -> history
// -> status shows AVAILABLE again.
func TestFullLifecycle(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "kitbook.db")
	svcDuePath := filepath.Join(dir, "service-due.txt")
	catalogueCSV := filepath.Join(dir, "catalogue.csv")

	writeFile(t, catalogueCSV, "item_id,item_type\nROPE-04,rope\nRADIO-11,radio\n")

	// Seed
	out, err := run(t, dbPath, svcDuePath, "seed", catalogueCSV)
	if err != nil {
		t.Fatalf("seed: error = %v, out = %s", err, out)
	}
	if !strings.Contains(out, "Loaded 2 items") {
		t.Fatalf("seed output = %q, want it to mention 2 items loaded", out)
	}

	// Status works before the service-due file exists (U5 has no U7 dependency)
	out, err = run(t, dbPath, svcDuePath, "status")
	if err != nil {
		t.Fatalf("status (pre-seed-of-service-due): error = %v, out = %s", err, out)
	}
	if !strings.Contains(out, "ROPE-04\tAVAILABLE") {
		t.Fatalf("status output = %q, want ROPE-04 AVAILABLE", out)
	}

	// Checkout fails closed: no service-due file yet
	_, err = run(t, dbPath, svcDuePath, "checkout", "ROPE-04", "--member", "Tayob", "--return", "2026-09-20")
	if err == nil || err.Error() != "service-due file unavailable — checkout blocked for safety" {
		t.Fatalf("checkout without service-due file: error = %v, want fail-closed message", err)
	}

	// Create the service-due file (nothing due yet), checkout succeeds
	writeFile(t, svcDuePath, "")
	out, err = run(t, dbPath, svcDuePath, "checkout", "ROPE-04", "--member", "Tayob", "--return", "2026-09-20")
	if err != nil {
		t.Fatalf("checkout: error = %v, out = %s", err, out)
	}
	if !strings.Contains(out, "Checked out ROPE-04") || !strings.Contains(out, "KB-") {
		t.Fatalf("checkout output = %q, want confirmation with a KB- booking id", out)
	}

	// status now shows the booking id (the gap HITL found and had fixed)
	out, err = run(t, dbPath, svcDuePath, "status")
	if err != nil {
		t.Fatalf("status: error = %v, out = %s", err, out)
	}
	if !strings.Contains(out, "ROPE-04\tCHECKED_OUT\tTayob\t2026-09-20\tKB-") {
		t.Fatalf("status output = %q, want ROPE-04 CHECKED_OUT row with a KB- booking id", out)
	}
	bookingID := extractBookingID(t, out, "ROPE-04")

	// Double-booking is blocked
	_, err = run(t, dbPath, svcDuePath, "checkout", "ROPE-04", "--member", "Nkosinathi", "--return", "2026-09-21")
	if err == nil || err.Error() != "item already checked out" {
		t.Fatalf("double-booking checkout: error = %v, want \"item already checked out\"", err)
	}

	// Service-due item is blocked
	writeFile(t, svcDuePath, "RADIO-11\n")
	_, err = run(t, dbPath, svcDuePath, "checkout", "RADIO-11", "--member", "Tayob", "--return", "2026-09-20")
	if err == nil || err.Error() != "item is service-due, no exceptions" {
		t.Fatalf("service-due checkout: error = %v, want \"item is service-due, no exceptions\"", err)
	}

	// Checkin frees the item
	out, err = run(t, dbPath, svcDuePath, "checkin", bookingID)
	if err != nil {
		t.Fatalf("checkin: error = %v, out = %s", err, out)
	}

	// History shows the completed booking
	out, err = run(t, dbPath, svcDuePath, "history", "ROPE-04")
	if err != nil {
		t.Fatalf("history: error = %v, out = %s", err, out)
	}
	if !strings.Contains(out, bookingID) || !strings.Contains(out, "Tayob") {
		t.Fatalf("history output = %q, want it to contain %s and Tayob", out, bookingID)
	}

	// Status reflects the item as available again
	out, err = run(t, dbPath, svcDuePath, "status")
	if err != nil {
		t.Fatalf("final status: error = %v, out = %s", err, out)
	}
	if !strings.Contains(out, "ROPE-04\tAVAILABLE") {
		t.Fatalf("final status output = %q, want ROPE-04 AVAILABLE again", out)
	}
}

func TestVersionAndDoctor(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "kitbook.db")
	svcDuePath := filepath.Join(dir, "service-due.txt")

	out, err := run(t, dbPath, svcDuePath, "version")
	if err != nil {
		t.Fatalf("version: error = %v", err)
	}
	if !strings.Contains(out, Version) {
		t.Fatalf("version output = %q, want it to contain %q", out, Version)
	}

	out, err = run(t, dbPath, svcDuePath, "doctor")
	if err != nil {
		t.Fatalf("doctor: error = %v, out = %s", err, out)
	}
	if !strings.Contains(out, "core: ok") || !strings.Contains(out, "store: ok") {
		t.Fatalf("doctor output = %q, want core: ok and store: ok", out)
	}
}

// extractBookingID pulls the KB-* token out of a status line for itemID.
func extractBookingID(t *testing.T, statusOutput, itemID string) string {
	t.Helper()
	for _, line := range strings.Split(statusOutput, "\n") {
		if strings.HasPrefix(line, itemID+"\t") {
			fields := strings.Split(line, "\t")
			last := fields[len(fields)-1]
			if strings.HasPrefix(last, "KB-") {
				return last
			}
		}
	}
	t.Fatalf("no booking id found for %s in status output: %q", itemID, statusOutput)
	return ""
}
