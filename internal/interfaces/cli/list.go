package cli

import (
	"encoding/json"
	"fmt"
	"text/tabwriter"

	meetingapp "github.com/felixgeelhaar/granola-mcp/internal/application/meeting"
	domain "github.com/felixgeelhaar/granola-mcp/internal/domain/meeting"
	"github.com/spf13/cobra"
)

func newListCmd(deps *Dependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List resources",
	}

	cmd.AddCommand(newListMeetingsCmd(deps))
	return cmd
}

func newListMeetingsCmd(deps *Dependencies) *cobra.Command {
	var (
		limit  int
		offset int
		source string
	)

	cmd := &cobra.Command{
		Use:   "meetings",
		Short: "List meetings",
		RunE: func(cmd *cobra.Command, args []string) error {
			input := meetingapp.ListMeetingsInput{
				Limit:  limit,
				Offset: offset,
			}
			if source != "" {
				input.Source = &source
			}

			out, err := deps.ListMeetings.Execute(cmd.Context(), input)
			if err != nil {
				return fmt.Errorf("failed to list meetings: %w", err)
			}

			switch flagFormat {
			case "json":
				return printJSON(deps, out.Meetings)
			default:
				return printMeetingsTable(deps, out.Meetings)
			}
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 20, "Max results")
	cmd.Flags().IntVar(&offset, "offset", 0, "Pagination offset")
	cmd.Flags().StringVar(&source, "source", "", "Filter by source (zoom, google_meet, teams)")

	return cmd
}

func printMeetingsTable(deps *Dependencies, meetings []*domain.Meeting) error {
	w := tabwriter.NewWriter(deps.Out, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "ID\tTITLE\tDATE\tSOURCE")
	for _, m := range meetings {
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			m.ID(), m.Title(), m.Datetime().Format("2006-01-02 15:04"), m.Source())
	}
	return w.Flush()
}

func printJSON(deps *Dependencies, v interface{}) error {
	enc := json.NewEncoder(deps.Out)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
