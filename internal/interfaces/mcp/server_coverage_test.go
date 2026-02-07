package mcp_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	domain "github.com/felixgeelhaar/granola-mcp/internal/domain/meeting"
	"github.com/felixgeelhaar/granola-mcp/internal/domain/workspace"
	mcpiface "github.com/felixgeelhaar/granola-mcp/internal/interfaces/mcp"
)

func TestServer_NameAndVersion(t *testing.T) {
	repo := newMockRepo()
	srv := newTestServer(repo)

	if srv.Name() != "granola-mcp" {
		t.Errorf("got name %q", srv.Name())
	}
	if srv.Version() != "test" {
		t.Errorf("got version %q", srv.Version())
	}
}

func TestServer_HandleSearchTranscripts(t *testing.T) {
	repo := newMockRepo()
	repo.addMeeting(mustMeeting(t, "m-1", "Sprint Planning"))

	srv := newTestServer(repo)

	results, err := srv.HandleSearchTranscripts(context.Background(), mcpiface.SearchTranscriptsToolInput{
		Query: "sprint",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("got %d results", len(results))
	}
}

func TestServer_HandleListMeetings_WithFilters(t *testing.T) {
	repo := newMockRepo()
	repo.addMeeting(mustMeeting(t, "m-1", "Meeting"))

	srv := newTestServer(repo)
	since := time.Now().Add(-24 * time.Hour).UTC().Format(time.RFC3339)
	until := time.Now().UTC().Format(time.RFC3339)
	limit := 5
	offset := 0
	source := "zoom"

	results, err := srv.HandleListMeetings(context.Background(), mcpiface.ListMeetingsToolInput{
		Since:  &since,
		Until:  &until,
		Limit:  &limit,
		Offset: &offset,
		Source: &source,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("got %d results", len(results))
	}
}

func TestServer_HandleListMeetings_InvalidSinceDate(t *testing.T) {
	repo := newMockRepo()
	srv := newTestServer(repo)

	bad := "not-a-date"
	_, err := srv.HandleListMeetings(context.Background(), mcpiface.ListMeetingsToolInput{
		Since: &bad,
	})
	if err == nil {
		t.Fatal("expected error for invalid since date")
	}
}

func TestServer_HandleListMeetings_InvalidUntilDate(t *testing.T) {
	repo := newMockRepo()
	srv := newTestServer(repo)

	bad := "not-a-date"
	_, err := srv.HandleListMeetings(context.Background(), mcpiface.ListMeetingsToolInput{
		Until: &bad,
	})
	if err == nil {
		t.Fatal("expected error for invalid until date")
	}
}

func TestServer_HandleGetActionItems_WithDueDate(t *testing.T) {
	repo := newMockRepo()
	due := time.Now().Add(48 * time.Hour).UTC()
	item, _ := domain.NewActionItem("ai-1", "m-1", "Alice", "Write report", &due)
	repo.addActionItems("m-1", []*domain.ActionItem{item})

	srv := newTestServer(repo)
	results, err := srv.HandleGetActionItems(context.Background(), mcpiface.GetActionItemsToolInput{
		MeetingID: "m-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if results[0].DueDate == nil {
		t.Error("expected due date")
	}
}

func TestServer_HandleToolJSON_GetMeeting(t *testing.T) {
	repo := newMockRepo()
	repo.addMeeting(mustMeeting(t, "m-1", "Test"))

	srv := newTestServer(repo)

	raw, err := srv.HandleToolJSON(context.Background(), "get_meeting", json.RawMessage(`{"id":"m-1"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result mcpiface.MeetingDetailResult
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if result.ID != "m-1" {
		t.Errorf("got id %q", result.ID)
	}
}

func TestServer_HandleToolJSON_GetTranscript(t *testing.T) {
	repo := newMockRepo()
	transcript := domain.NewTranscript("m-1", []domain.Utterance{
		domain.NewUtterance("Alice", "Hello", time.Now().UTC(), 0.9),
	})
	repo.addTranscript("m-1", &transcript)

	srv := newTestServer(repo)

	raw, err := srv.HandleToolJSON(context.Background(), "get_transcript", json.RawMessage(`{"meeting_id":"m-1"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result mcpiface.TranscriptResult
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if len(result.Utterances) != 1 {
		t.Errorf("got %d utterances", len(result.Utterances))
	}
}

func TestServer_HandleToolJSON_SearchTranscripts(t *testing.T) {
	repo := newMockRepo()
	repo.addMeeting(mustMeeting(t, "m-1", "Meeting"))

	srv := newTestServer(repo)

	raw, err := srv.HandleToolJSON(context.Background(), "search_transcripts", json.RawMessage(`{"query":"meeting"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var results []mcpiface.MeetingResult
	if err := json.Unmarshal(raw, &results); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
}

func TestServer_HandleToolJSON_GetActionItems(t *testing.T) {
	repo := newMockRepo()
	item, _ := domain.NewActionItem("ai-1", "m-1", "Alice", "Write", nil)
	repo.addActionItems("m-1", []*domain.ActionItem{item})

	srv := newTestServer(repo)

	raw, err := srv.HandleToolJSON(context.Background(), "get_action_items", json.RawMessage(`{"meeting_id":"m-1"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var results []mcpiface.ActionItemResult
	if err := json.Unmarshal(raw, &results); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("got %d results", len(results))
	}
}

func TestServer_HandleToolJSON_InvalidInput(t *testing.T) {
	repo := newMockRepo()
	srv := newTestServer(repo)

	_, err := srv.HandleToolJSON(context.Background(), "get_meeting", json.RawMessage(`{invalid`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestServer_Inner(t *testing.T) {
	repo := newMockRepo()
	srv := newTestServer(repo)

	if srv.Inner() == nil {
		t.Error("Inner() should not return nil")
	}
}

func TestServer_HandleSearchTranscripts_WithSince(t *testing.T) {
	repo := newMockRepo()
	repo.addMeeting(mustMeeting(t, "m-1", "Planning"))

	srv := newTestServer(repo)
	since := time.Now().Add(-24 * time.Hour).UTC().Format(time.RFC3339)
	limit := 10

	results, err := srv.HandleSearchTranscripts(context.Background(), mcpiface.SearchTranscriptsToolInput{
		Query: "planning",
		Since: &since,
		Limit: &limit,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("got %d results", len(results))
	}
}

func TestServer_HandleSearchTranscripts_InvalidSince(t *testing.T) {
	repo := newMockRepo()
	srv := newTestServer(repo)

	bad := "not-a-date"
	_, err := srv.HandleSearchTranscripts(context.Background(), mcpiface.SearchTranscriptsToolInput{
		Query: "test",
		Since: &bad,
	})
	if err == nil {
		t.Fatal("expected error for invalid since date")
	}
}

func TestServer_HandleGetTranscript_NotFound(t *testing.T) {
	repo := newMockRepo()
	srv := newTestServer(repo)

	_, err := srv.HandleGetTranscript(context.Background(), mcpiface.GetTranscriptToolInput{MeetingID: "nonexistent"})
	if err == nil {
		t.Fatal("expected error for missing transcript")
	}
}

func TestServer_HandleToolJSON_InvalidJSON_AllTools(t *testing.T) {
	repo := newMockRepo()
	srv := newTestServer(repo)

	tools := []string{"list_meetings", "get_meeting", "get_transcript", "search_transcripts", "get_action_items", "meeting_stats", "list_workspaces", "add_note", "list_notes", "delete_note", "complete_action_item", "update_action_item", "export_embeddings"}
	for _, tool := range tools {
		_, err := srv.HandleToolJSON(context.Background(), tool, json.RawMessage(`{invalid`))
		if err == nil {
			t.Errorf("expected error for invalid JSON on tool %q", tool)
		}
	}
}

func TestServer_HandleListWorkspaces(t *testing.T) {
	repo := newMockRepo()
	ws1, _ := workspace.New("ws-1", "Engineering", "engineering")
	ws2, _ := workspace.New("ws-2", "Design", "design")
	wsRepo := &mockWorkspaceRepo{workspaces: []*workspace.Workspace{ws1, ws2}}
	srv := newTestServerWithWorkspaces(repo, wsRepo)

	results, err := srv.HandleListWorkspaces(context.Background(), mcpiface.ListWorkspacesToolInput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 workspaces, got %d", len(results))
	}
	if results[0].ID != "ws-1" {
		t.Errorf("expected ws-1, got %s", results[0].ID)
	}
}

func TestServer_HandleListWorkspaces_Empty(t *testing.T) {
	repo := newMockRepo()
	wsRepo := &mockWorkspaceRepo{workspaces: []*workspace.Workspace{}}
	srv := newTestServerWithWorkspaces(repo, wsRepo)

	results, err := srv.HandleListWorkspaces(context.Background(), mcpiface.ListWorkspacesToolInput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0, got %d", len(results))
	}
}

func TestServer_HandleToolJSON_ListWorkspaces(t *testing.T) {
	repo := newMockRepo()
	ws1, _ := workspace.New("ws-1", "Engineering", "engineering")
	wsRepo := &mockWorkspaceRepo{workspaces: []*workspace.Workspace{ws1}}
	srv := newTestServerWithWorkspaces(repo, wsRepo)

	raw, err := srv.HandleToolJSON(context.Background(), "list_workspaces", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var results []mcpiface.WorkspaceResult
	if err := json.Unmarshal(raw, &results); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 workspace, got %d", len(results))
	}
}

func TestServer_ServeHTTP_StartsAndStops(t *testing.T) {
	repo := newMockRepo()
	srv := newTestServer(repo)

	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ServeHTTP(ctx, ":0", nil)
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("server did not stop in time")
	}
}

func TestServer_ServeHTTP_HealthEndpoint(t *testing.T) {
	repo := newMockRepo()
	srv := newTestServer(repo)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Use port 0 to get a random available port, but we need a known port for the test.
	// Use a specific port instead.
	port := 18923
	addr := fmt.Sprintf(":%d", port)

	go func() {
		_ = srv.ServeHTTP(ctx, addr, nil)
	}()

	// Wait for server to start
	time.Sleep(200 * time.Millisecond)

	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/health", port))
	if err != nil {
		t.Fatalf("health request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if len(body) == 0 {
		t.Error("expected non-empty health response")
	}
}

func TestServer_ServeHTTP_WithWebhookRoute(t *testing.T) {
	repo := newMockRepo()
	srv := newTestServer(repo)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	port := 18924
	addr := fmt.Sprintf(":%d", port)

	webhookCalled := false
	go func() {
		_ = srv.ServeHTTP(ctx, addr, func(mux *http.ServeMux) {
			mux.HandleFunc("/webhook/granola", func(w http.ResponseWriter, _ *http.Request) {
				webhookCalled = true
				w.WriteHeader(http.StatusOK)
			})
		})
	}()

	time.Sleep(200 * time.Millisecond)

	resp, err := http.Post(fmt.Sprintf("http://localhost:%d/webhook/granola", port), "application/json", nil)
	if err != nil {
		t.Fatalf("webhook request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	if !webhookCalled {
		t.Error("webhook handler was not called")
	}
}

// --- Write Tool Tests (Phase 3) ---

func TestServer_HandleAddNote(t *testing.T) {
	repo := newMockRepo()
	repo.addMeeting(mustMeeting(t, "m-1", "Sprint Planning"))

	srv := newTestServer(repo)

	result, err := srv.HandleAddNote(context.Background(), mcpiface.AddNoteToolInput{
		MeetingID: "m-1",
		Author:    "claude",
		Content:   "Great discussion",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.MeetingID != "m-1" {
		t.Errorf("got meeting id %q", result.MeetingID)
	}
	if result.Author != "claude" {
		t.Errorf("got author %q", result.Author)
	}
	if result.Content != "Great discussion" {
		t.Errorf("got content %q", result.Content)
	}
}

func TestServer_HandleAddNote_MeetingNotFound(t *testing.T) {
	repo := newMockRepo()
	srv := newTestServer(repo)

	_, err := srv.HandleAddNote(context.Background(), mcpiface.AddNoteToolInput{
		MeetingID: "nonexistent",
		Author:    "claude",
		Content:   "Note",
	})
	if err != domain.ErrMeetingNotFound {
		t.Errorf("got error %v, want %v", err, domain.ErrMeetingNotFound)
	}
}

func TestServer_HandleListNotes(t *testing.T) {
	repo := newMockRepo()
	repo.addMeeting(mustMeeting(t, "m-1", "Meeting"))

	srv := newTestServer(repo)

	// Add a note first
	_, err := srv.HandleAddNote(context.Background(), mcpiface.AddNoteToolInput{
		MeetingID: "m-1",
		Author:    "claude",
		Content:   "observation",
	})
	if err != nil {
		t.Fatalf("add note: %v", err)
	}

	results, err := srv.HandleListNotes(context.Background(), mcpiface.ListNotesToolInput{
		MeetingID: "m-1",
	})
	if err != nil {
		t.Fatalf("list notes: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("got %d notes, want 1", len(results))
	}
}

func TestServer_HandleListNotes_Empty(t *testing.T) {
	repo := newMockRepo()
	srv := newTestServer(repo)

	results, err := srv.HandleListNotes(context.Background(), mcpiface.ListNotesToolInput{
		MeetingID: "m-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("got %d notes, want 0", len(results))
	}
}

func TestServer_HandleDeleteNote(t *testing.T) {
	repo := newMockRepo()
	repo.addMeeting(mustMeeting(t, "m-1", "Meeting"))
	srv := newTestServer(repo)

	// Add then delete
	added, _ := srv.HandleAddNote(context.Background(), mcpiface.AddNoteToolInput{
		MeetingID: "m-1",
		Author:    "claude",
		Content:   "to delete",
	})

	_, err := srv.HandleDeleteNote(context.Background(), mcpiface.DeleteNoteToolInput{
		NoteID: added.ID,
	})
	if err != nil {
		t.Fatalf("delete note: %v", err)
	}

	// Verify deleted
	notes, _ := srv.HandleListNotes(context.Background(), mcpiface.ListNotesToolInput{
		MeetingID: "m-1",
	})
	if len(notes) != 0 {
		t.Errorf("note should be deleted, got %d", len(notes))
	}
}

func TestServer_HandleCompleteActionItem(t *testing.T) {
	repo := newMockRepo()
	item, _ := domain.NewActionItem("ai-1", "m-1", "Alice", "Write report", nil)
	repo.addActionItems("m-1", []*domain.ActionItem{item})

	srv := newTestServer(repo)

	result, err := srv.HandleCompleteActionItem(context.Background(), mcpiface.CompleteActionItemToolInput{
		MeetingID:    "m-1",
		ActionItemID: "ai-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Completed {
		t.Error("action item should be completed")
	}
}

func TestServer_HandleUpdateActionItem(t *testing.T) {
	repo := newMockRepo()
	item, _ := domain.NewActionItem("ai-1", "m-1", "Alice", "Original", nil)
	repo.addActionItems("m-1", []*domain.ActionItem{item})

	srv := newTestServer(repo)

	result, err := srv.HandleUpdateActionItem(context.Background(), mcpiface.UpdateActionItemToolInput{
		MeetingID:    "m-1",
		ActionItemID: "ai-1",
		Text:         "Updated text",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Text != "Updated text" {
		t.Errorf("got text %q", result.Text)
	}
}

func TestServer_HandleToolJSON_WriteTools(t *testing.T) {
	repo := newMockRepo()
	repo.addMeeting(mustMeeting(t, "m-1", "Meeting"))
	item, _ := domain.NewActionItem("ai-1", "m-1", "Alice", "Write", nil)
	repo.addActionItems("m-1", []*domain.ActionItem{item})

	srv := newTestServer(repo)

	// add_note via JSON
	raw, err := srv.HandleToolJSON(context.Background(), "add_note", json.RawMessage(`{"meeting_id":"m-1","author":"claude","content":"note"}`))
	if err != nil {
		t.Fatalf("add_note: %v", err)
	}
	var noteResult mcpiface.NoteResult
	if err := json.Unmarshal(raw, &noteResult); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// list_notes via JSON
	_, err = srv.HandleToolJSON(context.Background(), "list_notes", json.RawMessage(`{"meeting_id":"m-1"}`))
	if err != nil {
		t.Fatalf("list_notes: %v", err)
	}

	// delete_note via JSON
	input := fmt.Sprintf(`{"note_id":"%s"}`, noteResult.ID)
	_, err = srv.HandleToolJSON(context.Background(), "delete_note", json.RawMessage(input))
	if err != nil {
		t.Fatalf("delete_note: %v", err)
	}

	// complete_action_item via JSON
	_, err = srv.HandleToolJSON(context.Background(), "complete_action_item", json.RawMessage(`{"meeting_id":"m-1","action_item_id":"ai-1"}`))
	if err != nil {
		t.Fatalf("complete_action_item: %v", err)
	}

	// update_action_item via JSON
	raw, err = srv.HandleToolJSON(context.Background(), "update_action_item", json.RawMessage(`{"meeting_id":"m-1","action_item_id":"ai-1","text":"new"}`))
	if err != nil {
		t.Fatalf("update_action_item: %v", err)
	}
	_ = raw
}

func TestServer_HandleExportEmbeddings(t *testing.T) {
	repo := newMockRepo()
	repo.addMeeting(mustMeeting(t, "m-1", "Sprint Planning"))
	transcript := domain.NewTranscript("m-1", []domain.Utterance{
		domain.NewUtterance("Alice", "Hello world", time.Now().UTC(), 0.9),
	})
	repo.addTranscript("m-1", &transcript)

	srv := newTestServer(repo)

	result, err := srv.HandleExportEmbeddings(context.Background(), mcpiface.ExportEmbeddingsToolInput{
		MeetingIDs: []string{"m-1"},
		Strategy:   "speaker_turn",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ChunkCount < 1 {
		t.Errorf("expected at least 1 chunk, got %d", result.ChunkCount)
	}
	if result.Format != "jsonl" {
		t.Errorf("expected format 'jsonl', got %q", result.Format)
	}
}

func TestServer_HandleToolJSON_ExportEmbeddings(t *testing.T) {
	repo := newMockRepo()
	repo.addMeeting(mustMeeting(t, "m-1", "Meeting"))
	transcript := domain.NewTranscript("m-1", []domain.Utterance{
		domain.NewUtterance("Alice", "Content", time.Now().UTC(), 0.9),
	})
	repo.addTranscript("m-1", &transcript)

	srv := newTestServer(repo)

	raw, err := srv.HandleToolJSON(context.Background(), "export_embeddings", json.RawMessage(`{"meeting_ids":["m-1"]}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result mcpiface.ExportEmbeddingsResult
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if result.ChunkCount < 1 {
		t.Errorf("expected at least 1 chunk, got %d", result.ChunkCount)
	}
}

func TestServer_HandleGetMeeting_WithParticipants(t *testing.T) {
	repo := newMockRepo()
	m, _ := domain.New("m-1", "Sprint Planning", time.Now().UTC(), domain.SourceZoom, []domain.Participant{
		domain.NewParticipant("Alice", "alice@test.com", domain.RoleHost),
		domain.NewParticipant("Bob", "bob@test.com", domain.RoleAttendee),
	})
	m.ClearDomainEvents()
	repo.addMeeting(m)

	srv := newTestServer(repo)

	result, err := srv.HandleGetMeeting(context.Background(), mcpiface.GetMeetingToolInput{ID: "m-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Participants) != 2 {
		t.Errorf("got %d participants", len(result.Participants))
	}
	if result.Participants[0].Role != "host" {
		t.Errorf("got role %q", result.Participants[0].Role)
	}
}
