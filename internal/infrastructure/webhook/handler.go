package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"

	meetingapp "github.com/felixgeelhaar/granola-mcp/internal/application/meeting"
	domain "github.com/felixgeelhaar/granola-mcp/internal/domain/meeting"
)

// Handler receives Granola webhook events and triggers sync + event dispatch.
type Handler struct {
	syncUC     *meetingapp.SyncMeetings
	dispatcher domain.EventDispatcher
	secret     string
}

// NewHandler creates a new webhook handler.
// If secret is empty, signature validation is skipped.
func NewHandler(syncUC *meetingapp.SyncMeetings, dispatcher domain.EventDispatcher, secret string) *Handler {
	return &Handler{
		syncUC:     syncUC,
		dispatcher: dispatcher,
		secret:     secret,
	}
}

// ServeHTTP handles incoming webhook requests.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}
	defer func() { _ = r.Body.Close() }()

	if h.secret != "" {
		sig := r.Header.Get("X-Granola-Signature")
		if !h.validSignature(body, sig) {
			http.Error(w, "invalid signature", http.StatusUnauthorized)
			return
		}
	}

	var payload GranolaWebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		http.Error(w, "invalid JSON payload", http.StatusBadRequest)
		return
	}

	switch payload.Event {
	case "meeting.created", "transcript.ready":
		h.handleSync(r, payload)
	default:
		// Unknown event types are accepted but not processed
	}

	w.WriteHeader(http.StatusOK)
}

func (h *Handler) handleSync(r *http.Request, payload GranolaWebhookPayload) {
	since := payload.Timestamp
	out, err := h.syncUC.Execute(r.Context(), meetingapp.SyncMeetingsInput{Since: &since})
	if err != nil {
		log.Printf("webhook: sync failed for %s: %v", payload.Event, err)
		return
	}

	if len(out.Events) > 0 && h.dispatcher != nil {
		if err := h.dispatcher.Dispatch(r.Context(), out.Events); err != nil {
			log.Printf("webhook: dispatch failed: %v", err)
		}
	}
}

func (h *Handler) validSignature(body []byte, signature string) bool {
	mac := hmac.New(sha256.New, []byte(h.secret))
	_, _ = mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}
