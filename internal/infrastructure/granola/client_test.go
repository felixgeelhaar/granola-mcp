package granola_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/felixgeelhaar/granola-mcp/internal/infrastructure/granola"
)

func TestClient_GetDocuments(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/get-documents" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Error("missing or wrong auth header")
		}

		resp := granola.DocumentListResponse{
			Documents: []granola.DocumentDTO{
				{
					ID:        "m-1",
					Title:     "Sprint Planning",
					CreatedAt: now,
					Source:    "zoom",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := granola.NewClient(server.URL, server.Client(), "test-token")
	resp, err := client.GetDocuments(context.Background(), nil, 10, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Documents) != 1 {
		t.Fatalf("got %d documents, want 1", len(resp.Documents))
	}
	if resp.Documents[0].ID != "m-1" {
		t.Errorf("got id %q", resp.Documents[0].ID)
	}
}

func TestClient_GetDocument_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := granola.NewClient(server.URL, server.Client(), "test-token")
	_, err := client.GetDocument(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestClient_GetTranscript(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := granola.TranscriptResponse{
			MeetingID: "m-1",
			Utterances: []granola.UtteranceDTO{
				{Speaker: "Alice", Text: "Hello", Timestamp: now, Confidence: 0.95},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := granola.NewClient(server.URL, server.Client(), "test-token")
	resp, err := client.GetTranscript(context.Background(), "m-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Utterances) != 1 {
		t.Errorf("got %d utterances", len(resp.Utterances))
	}
}

func TestClient_Unauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	client := granola.NewClient(server.URL, server.Client(), "bad-token")
	_, err := client.GetDocuments(context.Background(), nil, 0, 0)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestClient_RateLimited(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	client := granola.NewClient(server.URL, server.Client(), "test-token")
	_, err := client.GetDocuments(context.Background(), nil, 0, 0)
	if err == nil {
		t.Fatal("expected error")
	}
}
