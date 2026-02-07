package granola_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/felixgeelhaar/granola-mcp/internal/domain/workspace"
	"github.com/felixgeelhaar/granola-mcp/internal/infrastructure/granola"
)

func TestWorkspaceRepository_List_ReturnsMappedWorkspaces(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"workspaces": []map[string]any{
				{"id": "ws-1", "name": "Engineering", "slug": "engineering"},
				{"id": "ws-2", "name": "Design", "slug": "design"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := granola.NewClient(ts.URL, ts.Client(), "test-token")
	repo := granola.NewWorkspaceRepository(client)

	workspaces, err := repo.List(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(workspaces) != 2 {
		t.Fatalf("expected 2 workspaces, got %d", len(workspaces))
	}
	if string(workspaces[0].ID()) != "ws-1" {
		t.Errorf("expected ws-1, got %s", workspaces[0].ID())
	}
	if workspaces[0].Name() != "Engineering" {
		t.Errorf("expected Engineering, got %s", workspaces[0].Name())
	}
	if workspaces[0].Slug() != "engineering" {
		t.Errorf("expected engineering, got %s", workspaces[0].Slug())
	}
}

func TestWorkspaceRepository_List_Empty(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{"workspaces": []map[string]any{}}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := granola.NewClient(ts.URL, ts.Client(), "test-token")
	repo := granola.NewWorkspaceRepository(client)

	workspaces, err := repo.List(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(workspaces) != 0 {
		t.Errorf("expected 0 workspaces, got %d", len(workspaces))
	}
}

func TestWorkspaceRepository_List_APIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal error"))
	}))
	defer ts.Close()

	client := granola.NewClient(ts.URL, ts.Client(), "test-token")
	repo := granola.NewWorkspaceRepository(client)

	_, err := repo.List(context.Background())
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestWorkspaceRepository_FindByID_Found(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"workspaces": []map[string]any{
				{"id": "ws-1", "name": "Engineering", "slug": "engineering"},
				{"id": "ws-2", "name": "Design", "slug": "design"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := granola.NewClient(ts.URL, ts.Client(), "test-token")
	repo := granola.NewWorkspaceRepository(client)

	ws, err := repo.FindByID(context.Background(), "ws-2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(ws.ID()) != "ws-2" {
		t.Errorf("expected ws-2, got %s", ws.ID())
	}
	if ws.Name() != "Design" {
		t.Errorf("expected Design, got %s", ws.Name())
	}
}

func TestWorkspaceRepository_FindByID_NotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"workspaces": []map[string]any{
				{"id": "ws-1", "name": "Engineering", "slug": "engineering"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := granola.NewClient(ts.URL, ts.Client(), "test-token")
	repo := granola.NewWorkspaceRepository(client)

	_, err := repo.FindByID(context.Background(), "nonexistent")
	if err != workspace.ErrWorkspaceNotFound {
		t.Errorf("expected ErrWorkspaceNotFound, got %v", err)
	}
}
