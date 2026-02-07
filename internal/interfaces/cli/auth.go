package cli

import (
	"fmt"

	authapp "github.com/felixgeelhaar/granola-mcp/internal/application/auth"
	domain "github.com/felixgeelhaar/granola-mcp/internal/domain/auth"
	"github.com/spf13/cobra"
)

func newAuthCmd(deps *Dependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage authentication with Granola",
	}

	cmd.AddCommand(newAuthLoginCmd(deps))
	cmd.AddCommand(newAuthStatusCmd(deps))

	return cmd
}

func newAuthLoginCmd(deps *Dependencies) *cobra.Command {
	var method string

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate with Granola",
		RunE: func(cmd *cobra.Command, args []string) error {
			authMethod := domain.AuthOAuth
			if method == "api_token" {
				authMethod = domain.AuthAPIToken
			}

			out, err := deps.Login.Execute(cmd.Context(), authapp.LoginInput{
				Method: authMethod,
			})
			if err != nil {
				return fmt.Errorf("login failed: %w", err)
			}

			_, _ = fmt.Fprintf(deps.Out, "Authenticated successfully (workspace: %s)\n", out.Credential.Workspace())
			return nil
		},
	}

	cmd.Flags().StringVar(&method, "method", "oauth", "Auth method: oauth or api_token")

	return cmd
}

func newAuthStatusCmd(deps *Dependencies) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Check authentication status",
		RunE: func(cmd *cobra.Command, args []string) error {
			out, err := deps.CheckStatus.Execute(cmd.Context())
			if err != nil {
				return err
			}

			if !out.Authenticated {
				_, _ = fmt.Fprintln(deps.Out, "Not authenticated. Run 'granola-mcp auth login' to authenticate.")
				return nil
			}

			_, _ = fmt.Fprintf(deps.Out, "Authenticated (workspace: %s, method: %s)\n",
				out.Credential.Workspace(), out.Credential.Method())
			return nil
		},
	}
}
