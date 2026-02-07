package cache_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	domain "github.com/felixgeelhaar/granola-mcp/internal/domain/meeting"
	"github.com/felixgeelhaar/granola-mcp/internal/infrastructure/cache"
	_ "github.com/mattn/go-sqlite3"
)

type mockRepo struct {
	meetings    map[domain.MeetingID]*domain.Meeting
	findCalls   int
	listCalls   int
	syncCalls   int
	searchCalls int
}

func newMockRepo() *mockRepo {
	return &mockRepo{meetings: make(map[domain.MeetingID]*domain.Meeting)}
}

func (m *mockRepo) FindByID(_ context.Context, id domain.MeetingID) (*domain.Meeting, error) {
	m.findCalls++
	if meeting, ok := m.meetings[id]; ok {
		return meeting, nil
	}
	return nil, domain.ErrMeetingNotFound
}

func (m *mockRepo) List(_ context.Context, _ domain.ListFilter) ([]*domain.Meeting, error) {
	m.listCalls++
	result := make([]*domain.Meeting, 0, len(m.meetings))
	for _, meeting := range m.meetings {
		result = append(result, meeting)
	}
	return result, nil
}

func (m *mockRepo) GetTranscript(_ context.Context, _ domain.MeetingID) (*domain.Transcript, error) {
	return nil, nil
}

func (m *mockRepo) SearchTranscripts(_ context.Context, _ string, _ domain.ListFilter) ([]*domain.Meeting, error) {
	m.searchCalls++
	return nil, nil
}

func (m *mockRepo) GetActionItems(_ context.Context, _ domain.MeetingID) ([]*domain.ActionItem, error) {
	return nil, nil
}

func (m *mockRepo) Sync(_ context.Context, _ *time.Time) ([]domain.DomainEvent, error) {
	m.syncCalls++
	return nil, nil
}

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func mustMeeting(t *testing.T, id, title string) *domain.Meeting {
	t.Helper()
	m, err := domain.New(domain.MeetingID(id), title, time.Now().UTC(), domain.SourceZoom, nil)
	if err != nil {
		t.Fatalf("create meeting: %v", err)
	}
	m.ClearDomainEvents()
	return m
}

func TestCachedRepository_FindByID_CacheMiss(t *testing.T) {
	db := openTestDB(t)
	inner := newMockRepo()
	inner.meetings["m-1"] = mustMeeting(t, "m-1", "Sprint Planning")

	repo, err := cache.NewCachedRepository(inner, db, 15*time.Minute)
	if err != nil {
		t.Fatalf("new cached repo: %v", err)
	}

	m, err := repo.FindByID(context.Background(), "m-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Title() != "Sprint Planning" {
		t.Errorf("got title %q", m.Title())
	}
	if inner.findCalls != 1 {
		t.Errorf("expected 1 inner call, got %d", inner.findCalls)
	}
}

func TestCachedRepository_FindByID_CacheHit(t *testing.T) {
	db := openTestDB(t)
	inner := newMockRepo()
	inner.meetings["m-1"] = mustMeeting(t, "m-1", "Sprint Planning")

	repo, err := cache.NewCachedRepository(inner, db, 15*time.Minute)
	if err != nil {
		t.Fatalf("new cached repo: %v", err)
	}

	// First call — cache miss
	_, _ = repo.FindByID(context.Background(), "m-1")
	// Second call — cache hit
	m, err := repo.FindByID(context.Background(), "m-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Title() != "Sprint Planning" {
		t.Errorf("got title %q", m.Title())
	}
	if inner.findCalls != 1 {
		t.Errorf("expected 1 inner call (cache hit), got %d", inner.findCalls)
	}
}

func TestCachedRepository_FindByID_NotFound(t *testing.T) {
	db := openTestDB(t)
	inner := newMockRepo()

	repo, err := cache.NewCachedRepository(inner, db, 15*time.Minute)
	if err != nil {
		t.Fatalf("new cached repo: %v", err)
	}

	_, err = repo.FindByID(context.Background(), "nonexistent")
	if err != domain.ErrMeetingNotFound {
		t.Errorf("got %v, want ErrMeetingNotFound", err)
	}
}

func TestCachedRepository_ListDelegatesToInner(t *testing.T) {
	db := openTestDB(t)
	inner := newMockRepo()

	repo, err := cache.NewCachedRepository(inner, db, 15*time.Minute)
	if err != nil {
		t.Fatalf("new cached repo: %v", err)
	}

	_, _ = repo.List(context.Background(), domain.ListFilter{})
	if inner.listCalls != 1 {
		t.Errorf("expected 1 list call, got %d", inner.listCalls)
	}
}

func TestCachedRepository_SyncDelegatesToInner(t *testing.T) {
	db := openTestDB(t)
	inner := newMockRepo()

	repo, err := cache.NewCachedRepository(inner, db, 15*time.Minute)
	if err != nil {
		t.Fatalf("new cached repo: %v", err)
	}

	_, _ = repo.Sync(context.Background(), nil)
	if inner.syncCalls != 1 {
		t.Errorf("expected 1 sync call, got %d", inner.syncCalls)
	}
}

func TestCachedRepository_Evict(t *testing.T) {
	db := openTestDB(t)
	inner := newMockRepo()
	inner.meetings["m-1"] = mustMeeting(t, "m-1", "Old Meeting")

	repo, err := cache.NewCachedRepository(inner, db, 1*time.Millisecond)
	if err != nil {
		t.Fatalf("new cached repo: %v", err)
	}

	// Populate cache
	_, _ = repo.FindByID(context.Background(), "m-1")
	// Wait for TTL expiry
	time.Sleep(5 * time.Millisecond)

	if err := repo.Evict(); err != nil {
		t.Fatalf("evict error: %v", err)
	}

	// Next call should hit inner again
	_, _ = repo.FindByID(context.Background(), "m-1")
	if inner.findCalls != 2 {
		t.Errorf("expected 2 inner calls after evict, got %d", inner.findCalls)
	}
}

func TestCachedRepository_SearchDelegatesToInner(t *testing.T) {
	db := openTestDB(t)
	inner := newMockRepo()

	repo, err := cache.NewCachedRepository(inner, db, 15*time.Minute)
	if err != nil {
		t.Fatalf("new cached repo: %v", err)
	}

	_, _ = repo.SearchTranscripts(context.Background(), "query", domain.ListFilter{})
	if inner.searchCalls != 1 {
		t.Errorf("expected 1 search call, got %d", inner.searchCalls)
	}
}
