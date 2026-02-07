package cli

import (
	"fmt"

	workspaceapp "github.com/felixgeelhaar/granola-mcp/internal/application/workspace"
	"github.com/spf13/cobra"
)

func newWorkspaceCmd(deps *Dependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workspace",
		Short: "Manage Granola workspaces",
	}

	cmd.AddCommand(newWorkspaceListCmd(deps))
	return cmd
}

func newWorkspaceListCmd(deps *Dependencies) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all workspaces",
		RunE: func(cmd *cobra.Command, args []string) error {
			if deps.ListWorkspaces == nil {
				return fmt.Errorf("workspace support not configured")
			}

			out, err := deps.ListWorkspaces.Execute(cmd.Context(), workspaceapp.ListWorkspacesInput{})
			if err != nil {
				return fmt.Errorf("list workspaces failed: %w", err)
			}

			if len(out.Workspaces) == 0 {
				_, _ = fmt.Fprintln(deps.Out, "No workspaces found.")
				return nil
			}

			_, _ = fmt.Fprintf(deps.Out, "%-20s %-30s %s\n", "ID", "NAME", "SLUG")
			for _, ws := range out.Workspaces {
				_, _ = fmt.Fprintf(deps.Out, "%-20s %-30s %s\n", ws.ID(), ws.Name(), ws.Slug())
			}
			return nil
		},
	}
}
