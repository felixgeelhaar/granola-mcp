package webhook_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	meetingapp "github.com/felixgeelhaar/granola-mcp/internal/application/meeting"
	domain "github.com/felixgeelhaar/granola-mcp/internal/domain/meeting"
	"github.com/felixgeelhaar/granola-mcp/internal/infrastructure/webhook"
)

type mockRepo struct {
	events []domain.DomainEvent
	err    error
	calls  int
}

func (m *mockRepo) FindByID(_ context.Context, _ domain.MeetingID) (*domain.Meeting, error) {
	return nil, nil
}
func (m *mockRepo) List(_ context.Context, _ domain.ListFilter) ([]*domain.Meeting, error) {
	return nil, nil
}
func (m *mockRepo) GetTranscript(_ context.Context, _ domain.MeetingID) (*domain.Transcript, error) {
	return nil, nil
}
func (m *mockRepo) SearchTranscripts(_ context.Context, _ string, _ domain.ListFilter) ([]*domain.Meeting, error) {
	return nil, nil
}
func (m *mockRepo) GetActionItems(_ context.Context, _ domain.MeetingID) ([]*domain.ActionItem, error) {
	return nil, nil
}
func (m *mockRepo) Sync(_ context.Context, _ *time.Time) ([]domain.DomainEvent, error) {
	m.calls++
	return m.events, m.err
}

type mockDispatcher struct {
	dispatched []domain.DomainEvent
}

func (m *mockDispatcher) Dispatch(_ context.Context, events []domain.DomainEvent) error {
	m.dispatched = append(m.dispatched, events...)
	return nil
}

func signBody(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func TestHandler_ValidPayload_Returns200(t *testing.T) {
	repo := &mockRepo{}
	d := &mockDispatcher{}
	h := webhook.NewHandler(meetingapp.NewSyncMeetings(repo), d, "")

	body := `{"event":"meeting.created","meeting_id":"m-1","timestamp":"2026-01-01T00:00:00Z"}`
	req := httptest.NewRequest(http.MethodPost, "/webhook/granola", strings.NewReader(body))
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestHandler_InvalidSignature_Returns401(t *testing.T) {
	repo := &mockRepo{}
	d := &mockDispatcher{}
	h := webhook.NewHandler(meetingapp.NewSyncMeetings(repo), d, "my-secret")

	body := `{"event":"meeting.created","meeting_id":"m-1","timestamp":"2026-01-01T00:00:00Z"}`
	req := httptest.NewRequest(http.MethodPost, "/webhook/granola", strings.NewReader(body))
	req.Header.Set("X-Granola-Signature", "invalid-sig")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestHandler_MalformedJSON_Returns400(t *testing.T) {
	repo := &mockRepo{}
	d := &mockDispatcher{}
	h := webhook.NewHandler(meetingapp.NewSyncMeetings(repo), d, "")

	req := httptest.NewRequest(http.MethodPost, "/webhook/granola", strings.NewReader("{invalid"))
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandler_MeetingCreated_TriggersSyncAndDispatches(t *testing.T) {
	event := domain.NewMeetingCreatedEvent("m-1", "Test", time.Now().UTC())
	repo := &mockRepo{events: []domain.DomainEvent{event}}
	d := &mockDispatcher{}
	h := webhook.NewHandler(meetingapp.NewSyncMeetings(repo), d, "")

	body := `{"event":"meeting.created","meeting_id":"m-1","timestamp":"2026-01-01T00:00:00Z"}`
	req := httptest.NewRequest(http.MethodPost, "/webhook/granola", strings.NewReader(body))
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if repo.calls != 1 {
		t.Errorf("expected 1 sync call, got %d", repo.calls)
	}
	if len(d.dispatched) != 1 {
		t.Errorf("expected 1 dispatched event, got %d", len(d.dispatched))
	}
}

func TestHandler_TranscriptReady_TriggersSyncAndDispatches(t *testing.T) {
	event := domain.NewTranscriptUpdatedEvent("m-1", 42)
	repo := &mockRepo{events: []domain.DomainEvent{event}}
	d := &mockDispatcher{}
	h := webhook.NewHandler(meetingapp.NewSyncMeetings(repo), d, "")

	body := `{"event":"transcript.ready","meeting_id":"m-1","timestamp":"2026-01-01T00:00:00Z"}`
	req := httptest.NewRequest(http.MethodPost, "/webhook/granola", strings.NewReader(body))
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if repo.calls != 1 {
		t.Errorf("expected 1 sync call, got %d", repo.calls)
	}
	if len(d.dispatched) != 1 {
		t.Errorf("expected 1 dispatched event, got %d", len(d.dispatched))
	}
}

func TestHandler_UnknownEvent_Returns200_NoOp(t *testing.T) {
	repo := &mockRepo{}
	d := &mockDispatcher{}
	h := webhook.NewHandler(meetingapp.NewSyncMeetings(repo), d, "")

	body := `{"event":"unknown.event","meeting_id":"m-1","timestamp":"2026-01-01T00:00:00Z"}`
	req := httptest.NewRequest(http.MethodPost, "/webhook/granola", strings.NewReader(body))
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if repo.calls != 0 {
		t.Errorf("expected 0 sync calls for unknown event, got %d", repo.calls)
	}
}

func TestHandler_MethodNotAllowed_Returns405(t *testing.T) {
	repo := &mockRepo{}
	d := &mockDispatcher{}
	h := webhook.NewHandler(meetingapp.NewSyncMeetings(repo), d, "")

	req := httptest.NewRequest(http.MethodGet, "/webhook/granola", nil)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestHandler_NoSecret_SkipsSignatureValidation(t *testing.T) {
	repo := &mockRepo{}
	d := &mockDispatcher{}
	h := webhook.NewHandler(meetingapp.NewSyncMeetings(repo), d, "") // empty secret

	body := `{"event":"meeting.created","meeting_id":"m-1","timestamp":"2026-01-01T00:00:00Z"}`
	req := httptest.NewRequest(http.MethodPost, "/webhook/granola", strings.NewReader(body))
	// No signature header
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 without secret, got %d", w.Code)
	}
}

func TestHandler_ValidSignature_Returns200(t *testing.T) {
	secret := "test-secret"
	repo := &mockRepo{}
	d := &mockDispatcher{}
	h := webhook.NewHandler(meetingapp.NewSyncMeetings(repo), d, secret)

	body := []byte(`{"event":"meeting.created","meeting_id":"m-1","timestamp":"2026-01-01T00:00:00Z"}`)
	sig := signBody(secret, body)

	req := httptest.NewRequest(http.MethodPost, "/webhook/granola", strings.NewReader(string(body)))
	req.Header.Set("X-Granola-Signature", sig)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 with valid signature, got %d", w.Code)
	}
}
