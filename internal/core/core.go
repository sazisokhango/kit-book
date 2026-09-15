// Package core holds kitbook's domain logic (checkout, checkin, status,
// history, service-due enforcement). Sprint Zero only wires the module up
// end-to-end; business logic lands in Sprint 1 against the specs in
// 05-spec/units/.
package core

// Ping is the Sprint Zero hello-world for the domain-logic container in the
// C4 diagram (solution-architecture.md). It proves the core package is wired
// into the CLI before any real business logic exists.
func Ping() string {
	return "ok"
}
