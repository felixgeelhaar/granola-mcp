package cli

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
)

func newServeCmd(deps *Dependencies) *cobra.Command {
	var transport string
	var port int

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the MCP server",
		Long:  "Start the Granola MCP server. By default serves over stdio for use with Claude Code and other MCP clients.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if deps.MCPServer == nil {
				return fmt.Errorf("MCP server not configured")
			}

			ctx, cancel := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
			defer cancel()

			switch transport {
			case "http":
				addr := fmt.Sprintf(":%d", port)
				_, _ = fmt.Fprintf(deps.Out, "Starting %s v%s MCP server (http on %s)...\n",
					deps.MCPServer.Name(), deps.MCPServer.Version(), addr)

				err := deps.MCPServer.ServeHTTP(ctx, addr, func(mux *http.ServeMux) {
					if deps.WebhookHandler != nil {
						mux.Handle("/webhook/granola", deps.WebhookHandler)
					}
				})
				if err != nil {
					if ctx.Err() != nil {
						_, _ = fmt.Fprintln(os.Stderr, "MCP server stopped.")
						return nil
					}
					return fmt.Errorf("MCP server error: %w", err)
				}
				return nil

			default: // stdio
				_, _ = fmt.Fprintf(deps.Out, "Starting %s v%s MCP server (stdio)...\n",
					deps.MCPServer.Name(), deps.MCPServer.Version())

				if err := deps.MCPServer.ServeStdio(ctx); err != nil {
					if ctx.Err() != nil {
						_, _ = fmt.Fprintln(os.Stderr, "MCP server stopped.")
						return nil
					}
					return fmt.Errorf("MCP server error: %w", err)
				}
				return nil
			}
		},
	}

	cmd.Flags().StringVar(&transport, "transport", "stdio", "Transport: stdio or http")
	cmd.Flags().IntVar(&port, "port", 8080, "HTTP port (when transport=http)")

	return cmd
}
