package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// These are set via ldflags at build time.
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "granola-mcp %s (commit: %s, built: %s)\n", Version, Commit, Date)
		},
	}
}
