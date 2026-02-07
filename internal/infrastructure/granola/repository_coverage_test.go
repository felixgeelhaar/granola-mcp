package granola_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	domain "github.com/felixgeelhaar/granola-mcp/internal/domain/meeting"
	"github.com/felixgeelhaar/granola-mcp/internal/infrastructure/granola"
)

func TestRepository_SearchTranscripts(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(granola.DocumentListResponse{
			Documents: []granola.DocumentDTO{
				{ID: "m-1", Title: "Meeting with keyword", CreatedAt: now, Source: "zoom"},
			},
		})
	}))
	defer server.Close()

	client := granola.NewClient(server.URL, server.Client(), "token")
	repo := granola.NewRepository(client)

	meetings, err := repo.SearchTranscripts(context.Background(), "keyword", domain.ListFilter{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(meetings) != 1 {
		t.Errorf("got %d meetings, want 1", len(meetings))
	}
}

func TestRepository_GetActionItems(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	due := now.Add(48 * time.Hour)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(granola.DocumentDTO{
			ID:        "m-1",
			Title:     "Sprint Planning",
			CreatedAt: now,
			Source:    "zoom",
			ActionItems: []granola.ActionItemDTO{
				{ID: "ai-1", Owner: "Alice", Text: "Write report", DueDate: &due, Done: false},
				{ID: "ai-2", Owner: "Bob", Text: "Review PR", Done: true},
			},
		})
	}))
	defer server.Close()

	client := granola.NewClient(server.URL, server.Client(), "token")
	repo := granola.NewRepository(client)

	items, err := repo.GetActionItems(context.Background(), "m-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 {
		t.Errorf("got %d items, want 2", len(items))
	}
	if items[0].Owner() != "Alice" {
		t.Errorf("got owner %q", items[0].Owner())
	}
	if !items[1].IsCompleted() {
		t.Error("second item should be completed")
	}
}

func TestRepository_GetActionItems_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := granola.NewClient(server.URL, server.Client(), "token")
	repo := granola.NewRepository(client)

	_, err := repo.GetActionItems(context.Background(), "nonexistent")
	if err != domain.ErrMeetingNotFound {
		t.Errorf("got error %v, want %v", err, domain.ErrMeetingNotFound)
	}
}

func TestClient_GetWorkspaces(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/get-workspaces" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(granola.WorkspaceListResponse{
			Workspaces: []granola.WorkspaceDTO{
				{ID: "ws-1", Name: "My Workspace", Slug: "my-workspace"},
			},
		})
	}))
	defer server.Close()

	client := granola.NewClient(server.URL, server.Client(), "token")
	resp, err := client.GetWorkspaces(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Workspaces) != 1 {
		t.Errorf("got %d workspaces", len(resp.Workspaces))
	}
}

func TestClient_SetToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer new-token" {
			t.Error("expected updated token in header")
		}
		_ = json.NewEncoder(w).Encode(granola.DocumentListResponse{})
	}))
	defer server.Close()

	client := granola.NewClient(server.URL, server.Client(), "old-token")
	client.SetToken("new-token")

	_, err := client.GetDocuments(context.Background(), nil, 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_NewClient_NilHTTPClient(t *testing.T) {
	client := granola.NewClient("http://localhost", nil, "token")
	if client == nil {
		t.Fatal("client should not be nil")
	}
}
