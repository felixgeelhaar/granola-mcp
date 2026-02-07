package cli

import (
	"fmt"
	"time"

	meetingapp "github.com/felixgeelhaar/granola-mcp/internal/application/meeting"
	"github.com/spf13/cobra"
)

func newSyncCmd(deps *Dependencies) *cobra.Command {
	var since string

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync meetings from Granola",
		RunE: func(cmd *cobra.Command, args []string) error {
			input := meetingapp.SyncMeetingsInput{}

			if since != "" {
				t, err := time.Parse(time.RFC3339, since)
				if err != nil {
					t, err = time.Parse("2006-01-02", since)
					if err != nil {
						return fmt.Errorf("invalid --since date: %w", err)
					}
				}
				input.Since = &t
			}

			out, err := deps.SyncMeetings.Execute(cmd.Context(), input)
			if err != nil {
				return fmt.Errorf("sync failed: %w", err)
			}

			if deps.EventDispatcher != nil && len(out.Events) > 0 {
				if dispErr := deps.EventDispatcher.Dispatch(cmd.Context(), out.Events); dispErr != nil {
					_, _ = fmt.Fprintf(deps.Out, "Warning: event dispatch failed: %v\n", dispErr)
				}
			}

			_, _ = fmt.Fprintf(deps.Out, "Synced %d meeting event(s)\n", len(out.Events))
			return nil
		},
	}

	cmd.Flags().StringVar(&since, "since", "", "Sync meetings since date (RFC3339 or YYYY-MM-DD)")

	return cmd
}
