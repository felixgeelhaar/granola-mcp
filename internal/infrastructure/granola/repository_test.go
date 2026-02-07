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

func TestRepository_FindByID(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(granola.DocumentDTO{
			ID:        "m-1",
			Title:     "Sprint Planning",
			CreatedAt: now,
			Source:    "zoom",
			Participants: []granola.ParticipantDTO{
				{Name: "Alice", Email: "alice@test.com", Role: "host"},
			},
		})
	}))
	defer server.Close()

	client := granola.NewClient(server.URL, server.Client(), "token")
	repo := granola.NewRepository(client)

	mtg, err := repo.FindByID(context.Background(), "m-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mtg.ID() != "m-1" {
		t.Errorf("got id %q", mtg.ID())
	}
	if mtg.Title() != "Sprint Planning" {
		t.Errorf("got title %q", mtg.Title())
	}
	if len(mtg.Participants()) != 1 {
		t.Errorf("got %d participants", len(mtg.Participants()))
	}
}

func TestRepository_FindByID_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := granola.NewClient(server.URL, server.Client(), "token")
	repo := granola.NewRepository(client)

	_, err := repo.FindByID(context.Background(), "nonexistent")
	if err != domain.ErrMeetingNotFound {
		t.Errorf("got error %v, want %v", err, domain.ErrMeetingNotFound)
	}
}

func TestRepository_List(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(granola.DocumentListResponse{
			Documents: []granola.DocumentDTO{
				{ID: "m-1", Title: "Meeting 1", CreatedAt: now, Source: "zoom"},
				{ID: "m-2", Title: "Meeting 2", CreatedAt: now, Source: "teams"},
			},
		})
	}))
	defer server.Close()

	client := granola.NewClient(server.URL, server.Client(), "token")
	repo := granola.NewRepository(client)

	meetings, err := repo.List(context.Background(), domain.ListFilter{Limit: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(meetings) != 2 {
		t.Errorf("got %d meetings, want 2", len(meetings))
	}
}

func TestRepository_GetTranscript(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(granola.TranscriptResponse{
			MeetingID: "m-1",
			Utterances: []granola.UtteranceDTO{
				{Speaker: "Alice", Text: "Hello", Timestamp: now, Confidence: 0.95},
			},
		})
	}))
	defer server.Close()

	client := granola.NewClient(server.URL, server.Client(), "token")
	repo := granola.NewRepository(client)

	transcript, err := repo.GetTranscript(context.Background(), "m-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(transcript.Utterances()) != 1 {
		t.Errorf("got %d utterances", len(transcript.Utterances()))
	}
}

func TestRepository_Sync(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(granola.DocumentListResponse{
			Documents: []granola.DocumentDTO{
				{ID: "m-new", Title: "New Meeting", CreatedAt: now, Source: "zoom"},
			},
		})
	}))
	defer server.Close()

	client := granola.NewClient(server.URL, server.Client(), "token")
	repo := granola.NewRepository(client)

	since := now.Add(-1 * time.Hour)
	events, err := repo.Sync(context.Background(), &since)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 1 {
		t.Errorf("got %d events, want 1", len(events))
	}
	if events[0].EventName() != "meeting.created" {
		t.Errorf("got event %q", events[0].EventName())
	}
}

func TestRepository_FindByID_Unauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	client := granola.NewClient(server.URL, server.Client(), "bad")
	repo := granola.NewRepository(client)

	_, err := repo.FindByID(context.Background(), "m-1")
	if err != domain.ErrAccessDenied {
		t.Errorf("got error %v, want %v", err, domain.ErrAccessDenied)
	}
}
