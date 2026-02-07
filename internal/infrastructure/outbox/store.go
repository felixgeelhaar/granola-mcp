// Package outbox implements the outbox pattern for reliable event persistence.
// Write-related events are both dispatched immediately (to MCP sessions)
// and persisted to an outbox table for future upstream sync.
package outbox

import (
	"database/sql"
	"encoding/json"
	"time"
)

// Entry represents a persisted outbox event.
type Entry struct {
	ID        string
	EventType string
	Payload   []byte
	Status    string
	CreatedAt time.Time
	SyncedAt  *time.Time
	Attempts  int
}

// Store is the interface for outbox persistence.
type Store interface {
	Append(entry Entry) error
	ListPending() ([]Entry, error)
	MarkSynced(id string) error
	MarkFailed(id string) error
}

// SQLiteStore implements Store using SQLite.
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore creates a new SQLite-backed outbox store.
func NewSQLiteStore(db *sql.DB) *SQLiteStore {
	return &SQLiteStore{db: db}
}

func (s *SQLiteStore) Append(entry Entry) error {
	payload := entry.Payload
	if payload == nil {
		payload = []byte("{}")
	}
	_, err := s.db.Exec(
		"INSERT INTO outbox_entries (id, event_type, payload, status, created_at, attempts) VALUES (?, ?, ?, ?, ?, ?)",
		entry.ID, entry.EventType, payload, "pending", entry.CreatedAt.UTC(), 0,
	)
	return err
}

func (s *SQLiteStore) ListPending() ([]Entry, error) {
	rows, err := s.db.Query(
		"SELECT id, event_type, payload, status, created_at, synced_at, attempts FROM outbox_entries WHERE status = 'pending' ORDER BY created_at ASC",
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var entries []Entry
	for rows.Next() {
		var e Entry
		var syncedAt sql.NullTime
		if err := rows.Scan(&e.ID, &e.EventType, &e.Payload, &e.Status, &e.CreatedAt, &syncedAt, &e.Attempts); err != nil {
			return nil, err
		}
		if syncedAt.Valid {
			e.SyncedAt = &syncedAt.Time
		}
		entries = append(entries, e)
	}
	if entries == nil {
		entries = []Entry{}
	}
	return entries, rows.Err()
}

func (s *SQLiteStore) MarkSynced(id string) error {
	now := time.Now().UTC()
	_, err := s.db.Exec(
		"UPDATE outbox_entries SET status = 'synced', synced_at = ? WHERE id = ?",
		now, id,
	)
	return err
}

func (s *SQLiteStore) MarkFailed(id string) error {
	_, err := s.db.Exec(
		"UPDATE outbox_entries SET status = 'failed', attempts = attempts + 1 WHERE id = ?",
		id,
	)
	return err
}

// MarshalEventPayload is a helper to serialize event data to JSON.
func MarshalEventPayload(v any) []byte {
	data, _ := json.Marshal(v)
	return data
}

var _ Store = (*SQLiteStore)(nil)
