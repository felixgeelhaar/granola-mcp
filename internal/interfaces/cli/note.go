package cli

import (
	"fmt"
	"text/tabwriter"

	annotationapp "github.com/felixgeelhaar/granola-mcp/internal/application/annotation"
	"github.com/spf13/cobra"
)

func newNoteCmd(deps *Dependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "note",
		Short: "Manage agent notes on meetings",
	}

	cmd.AddCommand(
		newNoteAddCmd(deps),
		newNoteListCmd(deps),
		newNoteDeleteCmd(deps),
	)
	return cmd
}

func newNoteAddCmd(deps *Dependencies) *cobra.Command {
	var author string

	cmd := &cobra.Command{
		Use:   "add <meeting_id> <text>",
		Short: "Add an agent note to a meeting",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if deps.AddNote == nil {
				return fmt.Errorf("note functionality not configured")
			}
			out, err := deps.AddNote.Execute(cmd.Context(), annotationapp.AddNoteInput{
				MeetingID: args[0],
				Author:    author,
				Content:   args[1],
			})
			if err != nil {
				return fmt.Errorf("failed to add note: %w", err)
			}
			_, _ = fmt.Fprintf(deps.Out, "Note %s added to meeting %s\n", out.Note.ID(), out.Note.MeetingID())
			return nil
		},
	}

	cmd.Flags().StringVar(&author, "author", "cli", "Note author")
	return cmd
}

func newNoteListCmd(deps *Dependencies) *cobra.Command {
	return &cobra.Command{
		Use:   "list <meeting_id>",
		Short: "List agent notes for a meeting",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if deps.ListNotes == nil {
				return fmt.Errorf("note functionality not configured")
			}
			out, err := deps.ListNotes.Execute(cmd.Context(), annotationapp.ListNotesInput{
				MeetingID: args[0],
			})
			if err != nil {
				return fmt.Errorf("failed to list notes: %w", err)
			}

			switch flagFormat {
			case "json":
				return printJSON(deps, out.Notes)
			default:
				w := tabwriter.NewWriter(deps.Out, 0, 0, 2, ' ', 0)
				_, _ = fmt.Fprintln(w, "ID\tAUTHOR\tCONTENT\tCREATED")
				for _, n := range out.Notes {
					_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
						n.ID(), n.Author(), n.Content(), n.CreatedAt().Format("2006-01-02 15:04"))
				}
				return w.Flush()
			}
		},
	}
}

func newNoteDeleteCmd(deps *Dependencies) *cobra.Command {
	return &cobra.Command{
		Use:   "delete <note_id>",
		Short: "Delete an agent note",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if deps.DeleteNote == nil {
				return fmt.Errorf("note functionality not configured")
			}
			_, err := deps.DeleteNote.Execute(cmd.Context(), annotationapp.DeleteNoteInput{
				NoteID: args[0],
			})
			if err != nil {
				return fmt.Errorf("failed to delete note: %w", err)
			}
			_, _ = fmt.Fprintf(deps.Out, "Note %s deleted\n", args[0])
			return nil
		},
	}
}
