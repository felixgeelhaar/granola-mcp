package export

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	domain "github.com/felixgeelhaar/granola-mcp/internal/domain/meeting"
)

var ErrUnsupportedFormat = errors.New("unsupported export format")

type Format string

const (
	FormatJSON     Format = "json"
	FormatMarkdown Format = "md"
	FormatText     Format = "txt"
)

type ExportMeetingInput struct {
	MeetingID domain.MeetingID
	Format    Format
}

type ExportMeetingOutput struct {
	Content string
	Format  Format
}

type ExportMeeting struct {
	repo domain.Repository
}

func NewExportMeeting(repo domain.Repository) *ExportMeeting {
	return &ExportMeeting{repo: repo}
}

func (uc *ExportMeeting) Execute(ctx context.Context, input ExportMeetingInput) (*ExportMeetingOutput, error) {
	if input.MeetingID == "" {
		return nil, domain.ErrInvalidMeetingID
	}

	mtg, err := uc.repo.FindByID(ctx, input.MeetingID)
	if err != nil {
		return nil, err
	}

	var content string
	switch input.Format {
	case FormatMarkdown:
		content = formatMarkdown(mtg)
	case FormatText:
		content = formatText(mtg)
	case FormatJSON, "":
		content = formatJSON(mtg)
	default:
		return nil, ErrUnsupportedFormat
	}

	f := input.Format
	if f == "" {
		f = FormatJSON
	}

	return &ExportMeetingOutput{
		Content: content,
		Format:  f,
	}, nil
}

func formatMarkdown(m *domain.Meeting) string {
	var b strings.Builder
	_, _ = fmt.Fprintf(&b, "# %s\n\n", m.Title())
	_, _ = fmt.Fprintf(&b, "**Date:** %s\n", m.Datetime().Format(time.RFC3339))
	_, _ = fmt.Fprintf(&b, "**Source:** %s\n\n", m.Source())

	if len(m.Participants()) > 0 {
		b.WriteString("## Participants\n\n")
		for _, p := range m.Participants() {
			_, _ = fmt.Fprintf(&b, "- %s (%s)\n", p.Name(), p.Email())
		}
		b.WriteString("\n")
	}

	if m.Summary() != nil {
		b.WriteString("## Summary\n\n")
		b.WriteString(m.Summary().Content())
		b.WriteString("\n\n")
	}

	if len(m.ActionItems()) > 0 {
		b.WriteString("## Action Items\n\n")
		for _, item := range m.ActionItems() {
			status := "[ ]"
			if item.IsCompleted() {
				status = "[x]"
			}
			_, _ = fmt.Fprintf(&b, "- %s %s (Owner: %s)\n", status, item.Text(), item.Owner())
		}
		b.WriteString("\n")
	}

	return b.String()
}

func formatText(m *domain.Meeting) string {
	var b strings.Builder
	_, _ = fmt.Fprintf(&b, "%s\n", m.Title())
	_, _ = fmt.Fprintf(&b, "Date: %s\n", m.Datetime().Format(time.RFC3339))
	_, _ = fmt.Fprintf(&b, "Source: %s\n", m.Source())

	if m.Summary() != nil {
		_, _ = fmt.Fprintf(&b, "\nSummary:\n%s\n", m.Summary().Content())
	}

	return b.String()
}

func formatJSON(m *domain.Meeting) string {
	// Minimal JSON without encoding/json to avoid domain layer import concerns.
	// The interfaces layer handles proper JSON serialization.
	var b strings.Builder
	_, _ = fmt.Fprintf(&b, `{"id":"%s","title":"%s","datetime":"%s","source":"%s"}`,
		m.ID(), m.Title(), m.Datetime().Format(time.RFC3339), m.Source())
	return b.String()
}
