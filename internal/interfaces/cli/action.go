package cli

import (
	"fmt"

	meetingapp "github.com/felixgeelhaar/granola-mcp/internal/application/meeting"
	domain "github.com/felixgeelhaar/granola-mcp/internal/domain/meeting"
	"github.com/spf13/cobra"
)

func newActionCmd(deps *Dependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "action",
		Short: "Manage action items",
	}

	cmd.AddCommand(
		newActionCompleteCmd(deps),
		newActionUpdateCmd(deps),
	)
	return cmd
}

func newActionCompleteCmd(deps *Dependencies) *cobra.Command {
	return &cobra.Command{
		Use:   "complete <meeting_id> <action_item_id>",
		Short: "Mark an action item as completed",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if deps.CompleteActionItem == nil {
				return fmt.Errorf("action item functionality not configured")
			}
			out, err := deps.CompleteActionItem.Execute(cmd.Context(), meetingapp.CompleteActionItemInput{
				MeetingID:    domain.MeetingID(args[0]),
				ActionItemID: domain.ActionItemID(args[1]),
			})
			if err != nil {
				return fmt.Errorf("failed to complete action item: %w", err)
			}
			_, _ = fmt.Fprintf(deps.Out, "Action item %s completed (text: %s)\n", out.Item.ID(), out.Item.Text())
			return nil
		},
	}
}

func newActionUpdateCmd(deps *Dependencies) *cobra.Command {
	return &cobra.Command{
		Use:   "update <meeting_id> <action_item_id> <text>",
		Short: "Update an action item's text",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			if deps.UpdateActionItem == nil {
				return fmt.Errorf("action item functionality not configured")
			}
			out, err := deps.UpdateActionItem.Execute(cmd.Context(), meetingapp.UpdateActionItemInput{
				MeetingID:    domain.MeetingID(args[0]),
				ActionItemID: domain.ActionItemID(args[1]),
				Text:         args[2],
			})
			if err != nil {
				return fmt.Errorf("failed to update action item: %w", err)
			}
			_, _ = fmt.Fprintf(deps.Out, "Action item %s updated (text: %s)\n", out.Item.ID(), out.Item.Text())
			return nil
		},
	}
}
