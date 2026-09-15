// Package cli wires kitbook's cobra command tree (ADR-003): version, doctor
// (Sprint Zero), and checkout/checkin/status (Sprint 1, against
// 05-spec/units/{u3,u4,u5}-*/spec.md).
package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"bitbucket.org/psybergate/kitbook/internal/catalogue"
	"bitbucket.org/psybergate/kitbook/internal/core"
	"bitbucket.org/psybergate/kitbook/internal/store"
)

// Version is the kitbook build version. Sprint Zero hard-codes it; a real
// version-injection mechanism (ldflags) can follow once there's a release
// pipeline to inject it in P8/P9.
const Version = "0.0.0-sprint1"

// defaultDBPath and defaultServiceDuePath are the default file locations on
// the operator's machine. RESOLVE-IN-PLAN: confirm the final path
// conventions with Chris (see 05-spec/units/u1-data-model/spec.md and
// u7-service-due-integration/spec.md) — both are overridable via flags in
// the meantime.
const (
	defaultDBPath         = "kitbook.db"
	defaultServiceDuePath = "service-due.txt"
)

func newRootCmd() *cobra.Command {
	var dbPath, serviceDuePath string

	root := &cobra.Command{
		Use:           "kitbook",
		Short:         "kitbook manages equipment checkout/checkin for Ridgeline Mountain Rescue",
		SilenceUsage:  true,
		SilenceErrors: false,
	}
	root.PersistentFlags().StringVar(&dbPath, "db", defaultDBPath, "path to the kitbook SQLite database file")
	root.PersistentFlags().StringVar(&serviceDuePath, "service-due-file", defaultServiceDuePath, "path to the service-due data file")

	openService := func() (*core.Service, *store.Store, error) {
		st, err := store.Open(dbPath)
		if err != nil {
			return nil, nil, fmt.Errorf("store: %w", err)
		}
		return core.NewService(st, serviceDuePath), st, nil
	}

	openStore := func() (*store.Store, error) {
		return store.Open(dbPath)
	}

	root.AddCommand(newVersionCmd())
	root.AddCommand(newDoctorCmd(&dbPath))
	root.AddCommand(newCheckoutCmd(openService))
	root.AddCommand(newCheckinCmd(openService))
	root.AddCommand(newStatusCmd(openService))
	root.AddCommand(newSeedCmd(openStore))
	root.AddCommand(newHistoryCmd(openStore))
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

// newDoctorCmd is kitbook's CLI equivalent of a "/health" endpoint: it
// exercises the domain-logic container (core.Ping) and the storage
// container (store.Open + Ping) end-to-end and reports a deterministic
// result.
func newDoctorCmd(dbPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check that kitbook's storage and core wiring are healthy",
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()

			fmt.Fprintf(out, "core: %s\n", core.Ping())

			s, err := store.Open(*dbPath)
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

// serviceOpener opens the wired-together Store + Service for a command
// invocation. Returning the raw *store.Store too lets callers defer Close.
type serviceOpener func() (*core.Service, *store.Store, error)

// newCheckoutCmd implements U3 (05-spec/units/u3-checkout/spec.md).
func newCheckoutCmd(open serviceOpener) *cobra.Command {
	var member, returnDate string

	cmd := &cobra.Command{
		Use:   "checkout <item-id>",
		Short: "Check out an item to a member",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, st, err := open()
			if err != nil {
				return err
			}
			defer st.Close()

			bookingID, err := svc.Checkout(args[0], member, returnDate)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Checked out %s -- booking %s\n", args[0], bookingID)
			return nil
		},
	}
	cmd.Flags().StringVar(&member, "member", "", "name of the member checking out the item")
	cmd.Flags().StringVar(&returnDate, "return", "", "expected return date (YYYY-MM-DD)")
	cmd.MarkFlagRequired("member")
	cmd.MarkFlagRequired("return")
	return cmd
}

// newCheckinCmd implements U4 (05-spec/units/u4-checkin/spec.md).
func newCheckinCmd(open serviceOpener) *cobra.Command {
	return &cobra.Command{
		Use:   "checkin <booking-id>",
		Short: "Check in an item by booking id",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, st, err := open()
			if err != nil {
				return err
			}
			defer st.Close()

			if err := svc.Checkin(args[0]); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Checked in booking %s\n", args[0])
			return nil
		},
	}
}

// newStatusCmd implements U5 (05-spec/units/u5-status-board/spec.md).
func newStatusCmd(open serviceOpener) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show every item's status, holder, and due-back date",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, st, err := open()
			if err != nil {
				return err
			}
			defer st.Close()

			rows, err := svc.Status()
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			fmt.Fprintln(out, "ITEM\tSTATUS\tHOLDER\tDUE BACK\tBOOKING ID")
			for _, r := range rows {
				fmt.Fprintf(out, "%s\t%s\t%s\t%s\t%s\n", r.ItemID, r.Status, r.Holder, r.DueBack, r.BookingID)
			}
			return nil
		},
	}
}

// storeOpener opens just the Store, for commands that don't need the
// service-due checker (seed, history).
type storeOpener func() (*store.Store, error)

// newSeedCmd implements U2 (05-spec/units/u2-catalogue-seed/spec.md).
func newSeedCmd(open storeOpener) *cobra.Command {
	return &cobra.Command{
		Use:   "seed <catalogue-csv>",
		Short: "Bulk-load the item catalogue from a CSV file (header: item_id,item_type)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := open()
			if err != nil {
				return fmt.Errorf("store: %w", err)
			}
			defer st.Close()

			result, err := catalogue.Seed(st, args[0])
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Loaded %d items\n", result.ItemsLoaded)
			return nil
		},
	}
}

// newHistoryCmd implements U6 (05-spec/units/u6-history/spec.md).
func newHistoryCmd(open storeOpener) *cobra.Command {
	return &cobra.Command{
		Use:   "history <item-id>",
		Short: "Show an item's booking history, oldest first",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := open()
			if err != nil {
				return fmt.Errorf("store: %w", err)
			}
			defer st.Close()

			history, err := st.ListBookingHistory(args[0])
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			for _, b := range history {
				checkinAt := ""
				if b.CheckinAt != nil {
					checkinAt = b.CheckinAt.Format("2006-01-02T15:04:05Z07:00")
				}
				fmt.Fprintf(out, "%s\t%s\t%s\t%s\n", b.BookingID, b.MemberName, b.CheckoutAt.Format("2006-01-02T15:04:05Z07:00"), checkinAt)
			}
			return nil
		},
	}
}

// Execute runs the kitbook root command.
func Execute() error {
	return newRootCmd().Execute()
}
