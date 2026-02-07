// Package cache provides a SQLite-backed repository decorator
// that caches meeting data locally to reduce API calls to Granola.
// Implements the decorator pattern: wraps a domain.Repository,
// checks local cache first, falls through to inner on miss.
package cache

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	domain "github.com/felixgeelhaar/granola-mcp/internal/domain/meeting"
)

// CachedRepository decorates a domain.Repository with local SQLite caching.
type CachedRepository struct {
	inner domain.Repository
	db    *sql.DB
	ttl   time.Duration
}

// NewCachedRepository creates a cached repository decorator.
// It initializes the cache schema on the provided database connection.
func NewCachedRepository(inner domain.Repository, db *sql.DB, ttl time.Duration) (*CachedRepository, error) {
	if err := initSchema(db); err != nil {
		return nil, err
	}
	return &CachedRepository{inner: inner, db: db, ttl: ttl}, nil
}

func initSchema(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS cache_entries (
			key        TEXT PRIMARY KEY,
			value      BLOB NOT NULL,
			expires_at DATETIME NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_cache_expires ON cache_entries(expires_at);
	`)
	return err
}

func (r *CachedRepository) get(key string) ([]byte, bool) {
	var data []byte
	err := r.db.QueryRow(
		"SELECT value FROM cache_entries WHERE key = ? AND expires_at > ?",
		key, time.Now().UTC(),
	).Scan(&data)
	if err != nil {
		return nil, false
	}
	return data, true
}

func (r *CachedRepository) set(key string, value []byte) {
	_, _ = r.db.Exec(
		"INSERT OR REPLACE INTO cache_entries (key, value, expires_at) VALUES (?, ?, ?)",
		key, value, time.Now().UTC().Add(r.ttl),
	)
}

// Evict removes expired entries from the cache.
func (r *CachedRepository) Evict() error {
	_, err := r.db.Exec("DELETE FROM cache_entries WHERE expires_at <= ?", time.Now().UTC())
	return err
}

// meetingCacheEntry is the serialized form of a Meeting for cache storage.
type meetingCacheEntry struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Datetime string `json:"datetime"`
	Source   string `json:"source"`
}

func toMeetingCacheEntry(m *domain.Meeting) meetingCacheEntry {
	return meetingCacheEntry{
		ID:       string(m.ID()),
		Title:    m.Title(),
		Datetime: m.Datetime().Format(time.RFC3339),
		Source:   string(m.Source()),
	}
}

func (r *CachedRepository) FindByID(ctx context.Context, id domain.MeetingID) (*domain.Meeting, error) {
	cacheKey := "meeting:" + string(id)
	if data, ok := r.get(cacheKey); ok {
		var entry meetingCacheEntry
		if json.Unmarshal(data, &entry) == nil {
			dt, _ := time.Parse(time.RFC3339, entry.Datetime)
			m, err := domain.New(domain.MeetingID(entry.ID), entry.Title, dt, domain.Source(entry.Source), nil)
			if err == nil {
				m.ClearDomainEvents()
				return m, nil
			}
		}
	}

	m, err := r.inner.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if data, marshalErr := json.Marshal(toMeetingCacheEntry(m)); marshalErr == nil {
		r.set(cacheKey, data)
	}
	return m, nil
}

func (r *CachedRepository) List(ctx context.Context, filter domain.ListFilter) ([]*domain.Meeting, error) {
	// List queries are parameterized — delegate directly to inner, no caching.
	return r.inner.List(ctx, filter)
}

func (r *CachedRepository) GetTranscript(ctx context.Context, id domain.MeetingID) (*domain.Transcript, error) {
	// Transcripts are large — delegate to inner, no caching for now.
	return r.inner.GetTranscript(ctx, id)
}

func (r *CachedRepository) SearchTranscripts(ctx context.Context, query string, filter domain.ListFilter) ([]*domain.Meeting, error) {
	// Search is parameterized — delegate directly to inner.
	return r.inner.SearchTranscripts(ctx, query, filter)
}

func (r *CachedRepository) GetActionItems(ctx context.Context, id domain.MeetingID) ([]*domain.ActionItem, error) {
	// Action items may change frequently — delegate to inner.
	return r.inner.GetActionItems(ctx, id)
}

func (r *CachedRepository) Sync(ctx context.Context, since *time.Time) ([]domain.DomainEvent, error) {
	// Sync always hits the API — and invalidates relevant cache entries.
	events, err := r.inner.Sync(ctx, since)
	if err != nil {
		return nil, err
	}
	// Invalidate cache for any meetings referenced in events.
	for _, e := range events {
		if mc, ok := e.(domain.MeetingCreated); ok {
			_, _ = r.db.Exec("DELETE FROM cache_entries WHERE key = ?", "meeting:"+string(mc.MeetingID()))
		}
	}
	return events, nil
}
