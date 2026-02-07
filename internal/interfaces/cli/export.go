package cli

import (
	"fmt"
	"strings"

	embeddingapp "github.com/felixgeelhaar/granola-mcp/internal/application/embedding"
	exportapp "github.com/felixgeelhaar/granola-mcp/internal/application/export"
	domain "github.com/felixgeelhaar/granola-mcp/internal/domain/meeting"
	"github.com/spf13/cobra"
)

func newExportCmd(deps *Dependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export meeting data",
	}

	cmd.AddCommand(
		newExportMeetingCmd(deps),
		newExportEmbeddingsCmd(deps),
	)
	return cmd
}

func newExportEmbeddingsCmd(deps *Dependencies) *cobra.Command {
	var (
		meetings  string
		strategy  string
		maxTokens int
	)

	cmd := &cobra.Command{
		Use:   "embeddings",
		Short: "Export meeting content as chunks for embedding generation",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if deps.ExportEmbeddings == nil {
				return fmt.Errorf("embedding export functionality not configured")
			}
			if meetings == "" {
				return fmt.Errorf("--meetings is required")
			}

			ids := strings.Split(meetings, ",")
			meetingIDs := make([]domain.MeetingID, len(ids))
			for i, id := range ids {
				meetingIDs[i] = domain.MeetingID(strings.TrimSpace(id))
			}

			out, err := deps.ExportEmbeddings.Execute(cmd.Context(), embeddingapp.ExportEmbeddingsInput{
				MeetingIDs: meetingIDs,
				Strategy:   strategy,
				MaxTokens:  maxTokens,
				Format:     "jsonl",
			})
			if err != nil {
				return fmt.Errorf("export failed: %w", err)
			}

			_, _ = fmt.Fprintln(deps.Out, out.Content)
			_, _ = fmt.Fprintf(deps.Out, "# %d chunks exported\n", out.ChunkCount)
			return nil
		},
	}

	cmd.Flags().StringVar(&meetings, "meetings", "", "Comma-separated meeting IDs")
	cmd.Flags().StringVar(&strategy, "strategy", "speaker_turn", "Chunking strategy: speaker_turn, time_window, token_limit")
	cmd.Flags().IntVar(&maxTokens, "max-tokens", 256, "Max tokens per chunk (for token_limit strategy)")
	return cmd
}

func newExportMeetingCmd(deps *Dependencies) *cobra.Command {
	return &cobra.Command{
		Use:   "meeting [id]",
		Short: "Export a meeting",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			format := exportapp.Format(flagFormat)
			if flagFormat == "table" {
				format = exportapp.FormatMarkdown
			}

			out, err := deps.ExportMeeting.Execute(cmd.Context(), exportapp.ExportMeetingInput{
				MeetingID: domain.MeetingID(args[0]),
				Format:    format,
			})
			if err != nil {
				return fmt.Errorf("export failed: %w", err)
			}

			_, _ = fmt.Fprint(deps.Out, out.Content)
			return nil
		},
	}
}
