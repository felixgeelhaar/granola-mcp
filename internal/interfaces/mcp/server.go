// Package mcp implements the MCP server interface layer.
// It translates between MCP protocol concepts (tools, resources)
// and application use cases, following the Ports & Adapters pattern.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	mcpfw "github.com/felixgeelhaar/mcp-go"

	annotationapp "github.com/felixgeelhaar/granola-mcp/internal/application/annotation"
	embeddingapp "github.com/felixgeelhaar/granola-mcp/internal/application/embedding"
	meetingapp "github.com/felixgeelhaar/granola-mcp/internal/application/meeting"
	workspaceapp "github.com/felixgeelhaar/granola-mcp/internal/application/workspace"
	"github.com/felixgeelhaar/granola-mcp/internal/domain/annotation"
	domain "github.com/felixgeelhaar/granola-mcp/internal/domain/meeting"
	"github.com/felixgeelhaar/granola-mcp/internal/domain/workspace"
)

// ServerOptions groups all use cases passed to NewServer.
type ServerOptions struct {
	ListMeetings      *meetingapp.ListMeetings
	GetMeeting        *meetingapp.GetMeeting
	GetTranscript     *meetingapp.GetTranscript
	SearchTranscripts *meetingapp.SearchTranscripts
	GetActionItems    *meetingapp.GetActionItems
	GetMeetingStats   *meetingapp.GetMeetingStats
	ListWorkspaces    *workspaceapp.ListWorkspaces
	GetWorkspace      *workspaceapp.GetWorkspace

	// Write use cases (Phase 3)
	AddNote            *annotationapp.AddNote
	ListNotes          *annotationapp.ListNotes
	DeleteNote         *annotationapp.DeleteNote
	CompleteActionItem *meetingapp.CompleteActionItem
	UpdateActionItem   *meetingapp.UpdateActionItem

	// Embedding export (Phase 3)
	ExportEmbeddings *embeddingapp.ExportEmbeddings
}

// Server wraps the mcp-go server and exposes Granola meeting data
// as MCP tools and resources.
type Server struct {
	inner *mcpfw.Server

	listMeetings      *meetingapp.ListMeetings
	getMeeting        *meetingapp.GetMeeting
	getTranscript     *meetingapp.GetTranscript
	searchTranscripts *meetingapp.SearchTranscripts
	getActionItems    *meetingapp.GetActionItems
	getMeetingStats   *meetingapp.GetMeetingStats
	listWorkspaces    *workspaceapp.ListWorkspaces
	getWorkspace      *workspaceapp.GetWorkspace

	// Write use cases (Phase 3)
	addNote            *annotationapp.AddNote
	listNotes          *annotationapp.ListNotes
	deleteNote         *annotationapp.DeleteNote
	completeActionItem *meetingapp.CompleteActionItem
	updateActionItem   *meetingapp.UpdateActionItem

	// Embedding export (Phase 3)
	exportEmbeddings *embeddingapp.ExportEmbeddings

	name    string
	version string
}

// NewServer creates a new MCP server wired to application use cases.
func NewServer(name, version string, opts ServerOptions) *Server {
	s := &Server{
		name:               name,
		version:            version,
		listMeetings:       opts.ListMeetings,
		getMeeting:         opts.GetMeeting,
		getTranscript:      opts.GetTranscript,
		searchTranscripts:  opts.SearchTranscripts,
		getActionItems:     opts.GetActionItems,
		getMeetingStats:    opts.GetMeetingStats,
		listWorkspaces:     opts.ListWorkspaces,
		getWorkspace:       opts.GetWorkspace,
		addNote:            opts.AddNote,
		listNotes:          opts.ListNotes,
		deleteNote:         opts.DeleteNote,
		completeActionItem: opts.CompleteActionItem,
		updateActionItem:   opts.UpdateActionItem,
		exportEmbeddings:   opts.ExportEmbeddings,
	}

	srv := mcpfw.NewServer(mcpfw.ServerInfo{
		Name:    name,
		Version: version,
	})

	s.registerTools(srv)
	s.registerResources(srv)

	s.inner = srv
	return s
}

func (s *Server) Name() string    { return s.name }
func (s *Server) Version() string { return s.version }

// Inner returns the underlying mcp-go server for transport integration.
func (s *Server) Inner() *mcpfw.Server { return s.inner }

// ServeStdio starts the MCP server on stdio transport.
func (s *Server) ServeStdio(ctx context.Context) error {
	return mcpfw.ServeStdio(ctx, s.inner)
}

// ServeHTTP starts the MCP server on HTTP+SSE transport.
// extraRoutes allows mounting additional HTTP handlers (e.g., webhook, health).
func (s *Server) ServeHTTP(ctx context.Context, addr string, extraRoutes func(mux *http.ServeMux)) error {
	mux := http.NewServeMux()

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{"status":"ok","server":"%s","version":"%s"}`, s.name, s.version)
	})

	if extraRoutes != nil {
		extraRoutes(mux)
	}

	srv := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	errCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	case err := <-errCh:
		return err
	}
}

// --- Tool registration ---

func (s *Server) registerTools(srv *mcpfw.Server) {
	srv.Tool("list_meetings").
		Description("Search and filter Granola meetings").
		Handler(s.HandleListMeetings)

	srv.Tool("get_meeting").
		Description("Get full details for a specific meeting").
		Handler(s.HandleGetMeeting)

	srv.Tool("get_transcript").
		Description("Get the transcript for a meeting").
		Handler(s.HandleGetTranscript)

	srv.Tool("search_transcripts").
		Description("Full-text search across all meeting transcripts").
		Handler(s.HandleSearchTranscripts)

	srv.Tool("get_action_items").
		Description("Get action items from a meeting").
		Handler(s.HandleGetActionItems)

	srv.Tool("meeting_stats").
		Description("Get aggregated meeting statistics with visual dashboard").
		UIResource("ui://meeting-stats").
		Handler(s.HandleMeetingStats)

	if s.listWorkspaces != nil {
		srv.Tool("list_workspaces").
			Description("List all Granola workspaces").
			Handler(s.HandleListWorkspaces)
	}

	// Write tools (Phase 3)
	if s.addNote != nil {
		srv.Tool("add_note").
			Description("Add an agent note to a meeting").
			Handler(s.HandleAddNote)
	}
	if s.listNotes != nil {
		srv.Tool("list_notes").
			Description("List agent notes for a meeting").
			Handler(s.HandleListNotes)
	}
	if s.deleteNote != nil {
		srv.Tool("delete_note").
			Description("Delete an agent note").
			Handler(s.HandleDeleteNote)
	}
	if s.completeActionItem != nil {
		srv.Tool("complete_action_item").
			Description("Mark an action item as completed").
			Handler(s.HandleCompleteActionItem)
	}
	if s.updateActionItem != nil {
		srv.Tool("update_action_item").
			Description("Update an action item's text").
			Handler(s.HandleUpdateActionItem)
	}
	if s.exportEmbeddings != nil {
		srv.Tool("export_embeddings").
			Description("Export meeting content as chunks for embedding generation (JSONL format)").
			Handler(s.HandleExportEmbeddings)
	}
}

// --- Resource registration ---

func (s *Server) registerResources(srv *mcpfw.Server) {
	srv.Resource("meeting://{id}").
		Name("Meeting").
		Description("A Granola meeting with metadata, participants, and source").
		MimeType("application/json").
		Handler(func(ctx context.Context, uri string, params map[string]string) (*mcpfw.ResourceContent, error) {
			id := params["id"]
			out, err := s.getMeeting.Execute(ctx, meetingapp.GetMeetingInput{
				ID: domain.MeetingID(id),
			})
			if err != nil {
				return nil, err
			}
			result := toMeetingDetailResult(out.Meeting)
			data, _ := json.Marshal(result)
			return &mcpfw.ResourceContent{
				URI:      uri,
				MimeType: "application/json",
				Text:     string(data),
			}, nil
		})

	srv.Resource("ui://meeting-stats").
		Name("Meeting Statistics Dashboard").
		Description("Interactive D3.js dashboard visualizing meeting statistics").
		MimeType("text/html;profile=mcp-app").
		Handler(func(_ context.Context, uri string, _ map[string]string) (*mcpfw.ResourceContent, error) {
			return &mcpfw.ResourceContent{
				URI:      uri,
				MimeType: "text/html;profile=mcp-app",
				Text:     meetingStatsHTML,
			}, nil
		})

	srv.Resource("transcript://{meeting_id}").
		Name("Transcript").
		Description("Ordered transcript utterances for a meeting").
		MimeType("application/json").
		Handler(func(ctx context.Context, uri string, params map[string]string) (*mcpfw.ResourceContent, error) {
			meetingID := params["meeting_id"]
			out, err := s.getTranscript.Execute(ctx, meetingapp.GetTranscriptInput{
				MeetingID: domain.MeetingID(meetingID),
			})
			if err != nil {
				return nil, err
			}
			result := toTranscriptResult(out.Transcript)
			data, _ := json.Marshal(result)
			return &mcpfw.ResourceContent{
				URI:      uri,
				MimeType: "application/json",
				Text:     string(data),
			}, nil
		})

	if s.listNotes != nil {
		srv.Resource("note://{meeting_id}").
			Name("Agent Notes").
			Description("Agent notes for a meeting").
			MimeType("application/json").
			Handler(func(ctx context.Context, uri string, params map[string]string) (*mcpfw.ResourceContent, error) {
				meetingID := params["meeting_id"]
				out, err := s.listNotes.Execute(ctx, annotationapp.ListNotesInput{
					MeetingID: meetingID,
				})
				if err != nil {
					return nil, err
				}
				results := make([]NoteResult, len(out.Notes))
				for i, n := range out.Notes {
					results[i] = toNoteResult(n)
				}
				data, _ := json.Marshal(results)
				return &mcpfw.ResourceContent{
					URI:      uri,
					MimeType: "application/json",
					Text:     string(data),
				}, nil
			})
	}

	if s.getWorkspace != nil {
		srv.Resource("workspace://{id}").
			Name("Workspace").
			Description("A Granola workspace with name and slug").
			MimeType("application/json").
			Handler(func(ctx context.Context, uri string, params map[string]string) (*mcpfw.ResourceContent, error) {
				id := params["id"]
				out, err := s.getWorkspace.Execute(ctx, workspaceapp.GetWorkspaceInput{
					ID: workspace.WorkspaceID(id),
				})
				if err != nil {
					return nil, err
				}
				result := toWorkspaceResult(out.Workspace)
				data, _ := json.Marshal(result)
				return &mcpfw.ResourceContent{
					URI:      uri,
					MimeType: "application/json",
					Text:     string(data),
				}, nil
			})
	}
}

// --- Tool Input Types ---

type ListMeetingsToolInput struct {
	Since       *string `json:"since,omitempty"`
	Until       *string `json:"until,omitempty"`
	Source      *string `json:"source,omitempty"`
	Participant *string `json:"participant,omitempty"`
	Query       *string `json:"query,omitempty"`
	Limit       *int    `json:"limit,omitempty"`
	Offset      *int    `json:"offset,omitempty"`
}

type GetMeetingToolInput struct {
	ID string `json:"id"`
}

type GetTranscriptToolInput struct {
	MeetingID string `json:"meeting_id"`
}

type SearchTranscriptsToolInput struct {
	Query string  `json:"query"`
	Since *string `json:"since,omitempty"`
	Until *string `json:"until,omitempty"`
	Limit *int    `json:"limit,omitempty"`
}

type GetActionItemsToolInput struct {
	MeetingID string `json:"meeting_id"`
}

type MeetingStatsToolInput struct {
	Since *string `json:"since,omitempty"`
	Until *string `json:"until,omitempty"`
}

type ListWorkspacesToolInput struct {
}

// --- Tool Output Types ---

type MeetingResult struct {
	ID           string              `json:"id"`
	Title        string              `json:"title"`
	Datetime     string              `json:"datetime"`
	Source       string              `json:"source"`
	Participants []ParticipantResult `json:"participants"`
}

type ParticipantResult struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Role  string `json:"role"`
}

type MeetingDetailResult struct {
	MeetingResult
	Summary     *SummaryResult     `json:"summary,omitempty"`
	ActionItems []ActionItemResult `json:"action_items,omitempty"`
}

type SummaryResult struct {
	Content string `json:"content"`
	Kind    string `json:"kind"`
}

type TranscriptResult struct {
	MeetingID  string            `json:"meeting_id"`
	Utterances []UtteranceResult `json:"utterances"`
}

type UtteranceResult struct {
	Speaker    string  `json:"speaker"`
	Text       string  `json:"text"`
	Timestamp  string  `json:"timestamp"`
	Confidence float64 `json:"confidence"`
}

type ActionItemResult struct {
	ID        string  `json:"id"`
	Owner     string  `json:"owner"`
	Text      string  `json:"text"`
	DueDate   *string `json:"due_date,omitempty"`
	Completed bool    `json:"completed"`
}

type MeetingStatsResult struct {
	GeneratedAt          string                              `json:"generated_at"`
	TotalMeetings        int                                 `json:"total_meetings"`
	DateRange            meetingapp.DateRange                `json:"date_range"`
	MeetingFrequency     []meetingapp.FrequencyEntry         `json:"meeting_frequency"`
	PlatformDistribution []meetingapp.PlatformEntry          `json:"platform_distribution"`
	TopParticipants      []meetingapp.ParticipantStatsEntry  `json:"top_participants"`
	ActionItems          meetingapp.ActionItemStats          `json:"action_items"`
	DayOfWeekHeatmap     []meetingapp.HeatmapEntry           `json:"day_of_week_heatmap"`
	SpeakerTalkTime      []meetingapp.SpeakerEntry           `json:"speaker_talk_time"`
	SummaryCoverage      meetingapp.SummaryCoverageStats     `json:"summary_coverage"`
}

type WorkspaceResult struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

// --- Tool Handlers ---

func (s *Server) HandleListMeetings(ctx context.Context, input ListMeetingsToolInput) ([]MeetingResult, error) {
	appInput := meetingapp.ListMeetingsInput{
		Source:      input.Source,
		Participant: input.Participant,
		Query:       input.Query,
	}

	if input.Since != nil {
		t, err := time.Parse(time.RFC3339, *input.Since)
		if err != nil {
			return nil, fmt.Errorf("invalid 'since' date: %w", err)
		}
		appInput.Since = &t
	}
	if input.Until != nil {
		t, err := time.Parse(time.RFC3339, *input.Until)
		if err != nil {
			return nil, fmt.Errorf("invalid 'until' date: %w", err)
		}
		appInput.Until = &t
	}
	if input.Limit != nil {
		appInput.Limit = *input.Limit
	} else {
		appInput.Limit = 20
	}
	if input.Offset != nil {
		appInput.Offset = *input.Offset
	}

	out, err := s.listMeetings.Execute(ctx, appInput)
	if err != nil {
		return nil, err
	}

	results := make([]MeetingResult, len(out.Meetings))
	for i, m := range out.Meetings {
		results[i] = toMeetingResult(m)
	}
	return results, nil
}

func (s *Server) HandleGetMeeting(ctx context.Context, input GetMeetingToolInput) (*MeetingDetailResult, error) {
	out, err := s.getMeeting.Execute(ctx, meetingapp.GetMeetingInput{
		ID: domain.MeetingID(input.ID),
	})
	if err != nil {
		return nil, err
	}

	result := toMeetingDetailResult(out.Meeting)
	return &result, nil
}

func (s *Server) HandleGetTranscript(ctx context.Context, input GetTranscriptToolInput) (*TranscriptResult, error) {
	out, err := s.getTranscript.Execute(ctx, meetingapp.GetTranscriptInput{
		MeetingID: domain.MeetingID(input.MeetingID),
	})
	if err != nil {
		return nil, err
	}

	result := toTranscriptResult(out.Transcript)
	return &result, nil
}

func (s *Server) HandleSearchTranscripts(ctx context.Context, input SearchTranscriptsToolInput) ([]MeetingResult, error) {
	appInput := meetingapp.SearchTranscriptsInput{
		Query: input.Query,
	}
	if input.Limit != nil {
		appInput.Limit = *input.Limit
	}
	if input.Since != nil {
		t, err := time.Parse(time.RFC3339, *input.Since)
		if err != nil {
			return nil, fmt.Errorf("invalid 'since' date: %w", err)
		}
		appInput.Since = &t
	}

	out, err := s.searchTranscripts.Execute(ctx, appInput)
	if err != nil {
		return nil, err
	}

	results := make([]MeetingResult, len(out.Meetings))
	for i, m := range out.Meetings {
		results[i] = toMeetingResult(m)
	}
	return results, nil
}

func (s *Server) HandleGetActionItems(ctx context.Context, input GetActionItemsToolInput) ([]ActionItemResult, error) {
	out, err := s.getActionItems.Execute(ctx, meetingapp.GetActionItemsInput{
		MeetingID: domain.MeetingID(input.MeetingID),
	})
	if err != nil {
		return nil, err
	}

	results := make([]ActionItemResult, len(out.Items))
	for i, item := range out.Items {
		results[i] = toActionItemResult(item)
	}
	return results, nil
}

func (s *Server) HandleMeetingStats(ctx context.Context, input MeetingStatsToolInput) (*MeetingStatsResult, error) {
	appInput := meetingapp.GetMeetingStatsInput{}

	if input.Since != nil {
		t, err := time.Parse(time.RFC3339, *input.Since)
		if err != nil {
			return nil, fmt.Errorf("invalid 'since' date: %w", err)
		}
		appInput.Since = &t
	}
	if input.Until != nil {
		t, err := time.Parse(time.RFC3339, *input.Until)
		if err != nil {
			return nil, fmt.Errorf("invalid 'until' date: %w", err)
		}
		appInput.Until = &t
	}

	out, err := s.getMeetingStats.Execute(ctx, appInput)
	if err != nil {
		return nil, err
	}

	return &MeetingStatsResult{
		GeneratedAt:          out.GeneratedAt.Format(time.RFC3339),
		TotalMeetings:        out.TotalMeetings,
		DateRange:            out.DateRange,
		MeetingFrequency:     out.MeetingFrequency,
		PlatformDistribution: out.PlatformDistribution,
		TopParticipants:      out.TopParticipants,
		ActionItems:          out.ActionItems,
		DayOfWeekHeatmap:     out.DayOfWeekHeatmap,
		SpeakerTalkTime:      out.SpeakerTalkTime,
		SummaryCoverage:      out.SummaryCoverage,
	}, nil
}

func (s *Server) HandleListWorkspaces(ctx context.Context, _ ListWorkspacesToolInput) ([]WorkspaceResult, error) {
	out, err := s.listWorkspaces.Execute(ctx, workspaceapp.ListWorkspacesInput{})
	if err != nil {
		return nil, err
	}

	results := make([]WorkspaceResult, len(out.Workspaces))
	for i, ws := range out.Workspaces {
		results[i] = toWorkspaceResult(ws)
	}
	return results, nil
}

// --- Result to JSON helper ---

func (s *Server) HandleToolJSON(ctx context.Context, tool string, rawInput json.RawMessage) (json.RawMessage, error) {
	switch tool {
	case "list_meetings":
		var input ListMeetingsToolInput
		if err := json.Unmarshal(rawInput, &input); err != nil {
			return nil, fmt.Errorf("invalid input: %w", err)
		}
		result, err := s.HandleListMeetings(ctx, input)
		if err != nil {
			return nil, err
		}
		return json.Marshal(result)

	case "get_meeting":
		var input GetMeetingToolInput
		if err := json.Unmarshal(rawInput, &input); err != nil {
			return nil, fmt.Errorf("invalid input: %w", err)
		}
		result, err := s.HandleGetMeeting(ctx, input)
		if err != nil {
			return nil, err
		}
		return json.Marshal(result)

	case "get_transcript":
		var input GetTranscriptToolInput
		if err := json.Unmarshal(rawInput, &input); err != nil {
			return nil, fmt.Errorf("invalid input: %w", err)
		}
		result, err := s.HandleGetTranscript(ctx, input)
		if err != nil {
			return nil, err
		}
		return json.Marshal(result)

	case "search_transcripts":
		var input SearchTranscriptsToolInput
		if err := json.Unmarshal(rawInput, &input); err != nil {
			return nil, fmt.Errorf("invalid input: %w", err)
		}
		result, err := s.HandleSearchTranscripts(ctx, input)
		if err != nil {
			return nil, err
		}
		return json.Marshal(result)

	case "get_action_items":
		var input GetActionItemsToolInput
		if err := json.Unmarshal(rawInput, &input); err != nil {
			return nil, fmt.Errorf("invalid input: %w", err)
		}
		result, err := s.HandleGetActionItems(ctx, input)
		if err != nil {
			return nil, err
		}
		return json.Marshal(result)

	case "meeting_stats":
		var input MeetingStatsToolInput
		if err := json.Unmarshal(rawInput, &input); err != nil {
			return nil, fmt.Errorf("invalid input: %w", err)
		}
		result, err := s.HandleMeetingStats(ctx, input)
		if err != nil {
			return nil, err
		}
		return json.Marshal(result)

	case "list_workspaces":
		var input ListWorkspacesToolInput
		if err := json.Unmarshal(rawInput, &input); err != nil {
			return nil, fmt.Errorf("invalid input: %w", err)
		}
		result, err := s.HandleListWorkspaces(ctx, input)
		if err != nil {
			return nil, err
		}
		return json.Marshal(result)

	case "add_note":
		var input AddNoteToolInput
		if err := json.Unmarshal(rawInput, &input); err != nil {
			return nil, fmt.Errorf("invalid input: %w", err)
		}
		result, err := s.HandleAddNote(ctx, input)
		if err != nil {
			return nil, err
		}
		return json.Marshal(result)

	case "list_notes":
		var input ListNotesToolInput
		if err := json.Unmarshal(rawInput, &input); err != nil {
			return nil, fmt.Errorf("invalid input: %w", err)
		}
		result, err := s.HandleListNotes(ctx, input)
		if err != nil {
			return nil, err
		}
		return json.Marshal(result)

	case "delete_note":
		var input DeleteNoteToolInput
		if err := json.Unmarshal(rawInput, &input); err != nil {
			return nil, fmt.Errorf("invalid input: %w", err)
		}
		result, err := s.HandleDeleteNote(ctx, input)
		if err != nil {
			return nil, err
		}
		return json.Marshal(result)

	case "complete_action_item":
		var input CompleteActionItemToolInput
		if err := json.Unmarshal(rawInput, &input); err != nil {
			return nil, fmt.Errorf("invalid input: %w", err)
		}
		result, err := s.HandleCompleteActionItem(ctx, input)
		if err != nil {
			return nil, err
		}
		return json.Marshal(result)

	case "update_action_item":
		var input UpdateActionItemToolInput
		if err := json.Unmarshal(rawInput, &input); err != nil {
			return nil, fmt.Errorf("invalid input: %w", err)
		}
		result, err := s.HandleUpdateActionItem(ctx, input)
		if err != nil {
			return nil, err
		}
		return json.Marshal(result)

	case "export_embeddings":
		var input ExportEmbeddingsToolInput
		if err := json.Unmarshal(rawInput, &input); err != nil {
			return nil, fmt.Errorf("invalid input: %w", err)
		}
		result, err := s.HandleExportEmbeddings(ctx, input)
		if err != nil {
			return nil, err
		}
		return json.Marshal(result)

	default:
		return nil, fmt.Errorf("unknown tool: %s", tool)
	}
}

// --- Mappers (interface layer → output DTOs) ---

func toMeetingResult(m *domain.Meeting) MeetingResult {
	participants := make([]ParticipantResult, len(m.Participants()))
	for i, p := range m.Participants() {
		participants[i] = ParticipantResult{
			Name:  p.Name(),
			Email: p.Email(),
			Role:  string(p.Role()),
		}
	}
	return MeetingResult{
		ID:           string(m.ID()),
		Title:        m.Title(),
		Datetime:     m.Datetime().Format(time.RFC3339),
		Source:       string(m.Source()),
		Participants: participants,
	}
}

func toMeetingDetailResult(m *domain.Meeting) MeetingDetailResult {
	result := MeetingDetailResult{
		MeetingResult: toMeetingResult(m),
	}

	if m.Summary() != nil {
		result.Summary = &SummaryResult{
			Content: m.Summary().Content(),
			Kind:    string(m.Summary().Kind()),
		}
	}

	items := m.ActionItems()
	result.ActionItems = make([]ActionItemResult, len(items))
	for i, item := range items {
		result.ActionItems[i] = toActionItemResult(item)
	}

	return result
}

func toTranscriptResult(t *domain.Transcript) TranscriptResult {
	utterances := make([]UtteranceResult, len(t.Utterances()))
	for i, u := range t.Utterances() {
		utterances[i] = UtteranceResult{
			Speaker:    u.Speaker(),
			Text:       u.Text(),
			Timestamp:  u.Timestamp().Format(time.RFC3339),
			Confidence: u.Confidence(),
		}
	}
	return TranscriptResult{
		MeetingID:  string(t.MeetingID()),
		Utterances: utterances,
	}
}

func toActionItemResult(item *domain.ActionItem) ActionItemResult {
	r := ActionItemResult{
		ID:        string(item.ID()),
		Owner:     item.Owner(),
		Text:      item.Text(),
		Completed: item.IsCompleted(),
	}
	if item.DueDate() != nil {
		s := item.DueDate().Format(time.RFC3339)
		r.DueDate = &s
	}
	return r
}

func toWorkspaceResult(ws *workspace.Workspace) WorkspaceResult {
	return WorkspaceResult{
		ID:   string(ws.ID()),
		Name: ws.Name(),
		Slug: ws.Slug(),
	}
}

// --- Embedding Export Tool Input Type ---

type ExportEmbeddingsToolInput struct {
	MeetingIDs []string `json:"meeting_ids"`
	Strategy   string   `json:"strategy,omitempty"`
	MaxTokens  int      `json:"max_tokens,omitempty"`
}

type ExportEmbeddingsResult struct {
	Content    string `json:"content"`
	ChunkCount int    `json:"chunk_count"`
	Format     string `json:"format"`
}

// --- Write Tool Input Types (Phase 3) ---

type AddNoteToolInput struct {
	MeetingID string `json:"meeting_id"`
	Author    string `json:"author"`
	Content   string `json:"content"`
}

type ListNotesToolInput struct {
	MeetingID string `json:"meeting_id"`
}

type DeleteNoteToolInput struct {
	NoteID string `json:"note_id"`
}

type CompleteActionItemToolInput struct {
	MeetingID    string `json:"meeting_id"`
	ActionItemID string `json:"action_item_id"`
}

type UpdateActionItemToolInput struct {
	MeetingID    string `json:"meeting_id"`
	ActionItemID string `json:"action_item_id"`
	Text         string `json:"text"`
}

// --- Write Tool Output Types ---

type NoteResult struct {
	ID        string `json:"id"`
	MeetingID string `json:"meeting_id"`
	Author    string `json:"author"`
	Content   string `json:"content"`
	CreatedAt string `json:"created_at"`
}

func toNoteResult(n *annotation.AgentNote) NoteResult {
	return NoteResult{
		ID:        string(n.ID()),
		MeetingID: n.MeetingID(),
		Author:    n.Author(),
		Content:   n.Content(),
		CreatedAt: n.CreatedAt().Format(time.RFC3339),
	}
}

// --- Write Tool Handlers ---

func (s *Server) HandleAddNote(ctx context.Context, input AddNoteToolInput) (*NoteResult, error) {
	out, err := s.addNote.Execute(ctx, annotationapp.AddNoteInput{
		MeetingID: input.MeetingID,
		Author:    input.Author,
		Content:   input.Content,
	})
	if err != nil {
		return nil, err
	}
	result := toNoteResult(out.Note)
	return &result, nil
}

func (s *Server) HandleListNotes(ctx context.Context, input ListNotesToolInput) ([]NoteResult, error) {
	out, err := s.listNotes.Execute(ctx, annotationapp.ListNotesInput{
		MeetingID: input.MeetingID,
	})
	if err != nil {
		return nil, err
	}
	results := make([]NoteResult, len(out.Notes))
	for i, n := range out.Notes {
		results[i] = toNoteResult(n)
	}
	return results, nil
}

func (s *Server) HandleDeleteNote(ctx context.Context, input DeleteNoteToolInput) (*struct{}, error) {
	_, err := s.deleteNote.Execute(ctx, annotationapp.DeleteNoteInput{
		NoteID: input.NoteID,
	})
	if err != nil {
		return nil, err
	}
	return &struct{}{}, nil
}

func (s *Server) HandleCompleteActionItem(ctx context.Context, input CompleteActionItemToolInput) (*ActionItemResult, error) {
	out, err := s.completeActionItem.Execute(ctx, meetingapp.CompleteActionItemInput{
		MeetingID:    domain.MeetingID(input.MeetingID),
		ActionItemID: domain.ActionItemID(input.ActionItemID),
	})
	if err != nil {
		return nil, err
	}
	result := toActionItemResult(out.Item)
	return &result, nil
}

func (s *Server) HandleUpdateActionItem(ctx context.Context, input UpdateActionItemToolInput) (*ActionItemResult, error) {
	out, err := s.updateActionItem.Execute(ctx, meetingapp.UpdateActionItemInput{
		MeetingID:    domain.MeetingID(input.MeetingID),
		ActionItemID: domain.ActionItemID(input.ActionItemID),
		Text:         input.Text,
	})
	if err != nil {
		return nil, err
	}
	result := toActionItemResult(out.Item)
	return &result, nil
}

func (s *Server) HandleExportEmbeddings(ctx context.Context, input ExportEmbeddingsToolInput) (*ExportEmbeddingsResult, error) {
	meetingIDs := make([]domain.MeetingID, len(input.MeetingIDs))
	for i, id := range input.MeetingIDs {
		meetingIDs[i] = domain.MeetingID(id)
	}

	out, err := s.exportEmbeddings.Execute(ctx, embeddingapp.ExportEmbeddingsInput{
		MeetingIDs: meetingIDs,
		Strategy:   input.Strategy,
		MaxTokens:  input.MaxTokens,
		Format:     "jsonl",
	})
	if err != nil {
		return nil, err
	}

	return &ExportEmbeddingsResult{
		Content:    out.Content,
		ChunkCount: out.ChunkCount,
		Format:     "jsonl",
	}, nil
}
