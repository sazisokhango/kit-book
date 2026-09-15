package core

import "testing"

// TestPing is the Sprint Zero hello-world test — it exists so the CI
// pipeline has something real to run before any business-logic tests land
// in Sprint 1.
func TestPing(t *testing.T) {
	got := Ping()
	want := "ok"
	if got != want {
		t.Fatalf("Ping() = %q, want %q", got, want)
	}
}
