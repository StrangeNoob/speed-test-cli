package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// newUpdateCmd builds the `speed-test update` subcommand. It always checks
// GitHub (ignoring the 24h throttle and --no-update-check) and self-updates.
func newUpdateCmd(versionRaw string) *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "Update speed-test to the latest release",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			fmt.Fprintln(os.Stderr, "Checking for updates…")
			return runUpdate(versionRaw)
		},
	}
}
