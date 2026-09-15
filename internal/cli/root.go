// Package cli wires kitbook's cobra command tree (ADR-003). Sprint Zero adds
// only the hello-world commands (version, doctor); checkout/checkin/status/
// history land in Sprint 1 against 05-spec/units/.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"bitbucket.org/psybergate/kitbook/internal/core"
	"bitbucket.org/psybergate/kitbook/internal/store"
)

// Version is the kitbook build version. Sprint Zero hard-codes it; a real
// version-injection mechanism (ldflags) can follow once there's a release
// pipeline to inject it in P8/P9.
const Version = "0.0.0-sprintzero"

// defaultDBPath is the default location of the SQLite database file on the
// operator's machine. RESOLVE-IN-PLAN: confirm the final path convention
// with Chris before Sprint 1 ships (see 05-spec/units/u1-data-model/spec.md).
const defaultDBPath = "kitbook.db"

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "kitbook",
		Short: "kitbook manages equipment checkout/checkin for Ridgeline Mountain Rescue",
	}
	root.AddCommand(newVersionCmd())
	root.AddCommand(newDoctorCmd())
	return root
}

// newVersionCmd is the Sprint Zero hello-world entry point for the CLI
// container in the C4 diagram: a deterministic, known response.
func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the kitbook version",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.OutOrStdout(), Version)
			return nil
		},
	}
}

// newDoctorCmd is kitbook's CLI equivalent of a "/health" endpoint
// (sprint-zero-checklist.md has no network service to expose one on): it
// exercises the domain-logic container (core.Ping) and the storage
// container (store.Open + Ping) end-to-end and reports a deterministic
// result.
func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check that kitbook's storage and core wiring are healthy",
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()

			fmt.Fprintf(out, "core: %s\n", core.Ping())

			s, err := store.Open(defaultDBPath)
			if err != nil {
				return fmt.Errorf("store: %w", err)
			}
			defer s.Close()

			if err := s.Ping(); err != nil {
				return fmt.Errorf("store ping: %w", err)
			}
			fmt.Fprintln(out, "store: ok")
			return nil
		},
	}
}

// Execute runs the kitbook root command.
func Execute() error {
	return newRootCmd().Execute()
}
