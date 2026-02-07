package resilience_test

import (
	"context"
	"testing"
	"time"

	domain "github.com/felixgeelhaar/granola-mcp/internal/domain/meeting"
	"github.com/felixgeelhaar/granola-mcp/internal/infrastructure/resilience"
)

type stubRepo struct {
	findByIDCalled       bool
	listCalled           bool
	getTranscriptCalled  bool
	searchCalled         bool
	getActionItemsCalled bool
	syncCalled           bool
	callCount            int
}

func (s *stubRepo) FindByID(_ context.Context, _ domain.MeetingID) (*domain.Meeting, error) {
	s.findByIDCalled = true
	s.callCount++
	return nil, domain.ErrMeetingNotFound
}

func (s *stubRepo) List(_ context.Context, _ domain.ListFilter) ([]*domain.Meeting, error) {
	s.listCalled = true
	s.callCount++
	return nil, nil
}

func (s *stubRepo) GetTranscript(_ context.Context, _ domain.MeetingID) (*domain.Transcript, error) {
	s.getTranscriptCalled = true
	s.callCount++
	return nil, nil
}

func (s *stubRepo) SearchTranscripts(_ context.Context, _ string, _ domain.ListFilter) ([]*domain.Meeting, error) {
	s.searchCalled = true
	s.callCount++
	return nil, nil
}

func (s *stubRepo) GetActionItems(_ context.Context, _ domain.MeetingID) ([]*domain.ActionItem, error) {
	s.getActionItemsCalled = true
	s.callCount++
	return nil, nil
}

func (s *stubRepo) Sync(_ context.Context, _ *time.Time) ([]domain.DomainEvent, error) {
	s.syncCalled = true
	s.callCount++
	return nil, nil
}

func TestResilientRepository_DelegatesToInner(t *testing.T) {
	inner := &stubRepo{}
	repo := resilience.NewResilientRepository(inner, resilience.DefaultConfig())
	defer func() { _ = repo.Close() }()

	_, err := repo.FindByID(context.Background(), "m-1")
	if err != domain.ErrMeetingNotFound {
		t.Errorf("got error %v, want %v", err, domain.ErrMeetingNotFound)
	}
	if !inner.findByIDCalled {
		t.Error("expected inner FindByID to be called")
	}
}

func TestResilientRepository_List(t *testing.T) {
	inner := &stubRepo{}
	repo := resilience.NewResilientRepository(inner, resilience.DefaultConfig())
	defer func() { _ = repo.Close() }()

	_, _ = repo.List(context.Background(), domain.ListFilter{})
	if !inner.listCalled {
		t.Error("expected inner List to be called")
	}
}

func TestResilientRepository_GetTranscript(t *testing.T) {
	inner := &stubRepo{}
	repo := resilience.NewResilientRepository(inner, resilience.DefaultConfig())
	defer func() { _ = repo.Close() }()

	_, _ = repo.GetTranscript(context.Background(), "m-1")
	if !inner.getTranscriptCalled {
		t.Error("expected inner GetTranscript to be called")
	}
}

func TestResilientRepository_SearchTranscripts(t *testing.T) {
	inner := &stubRepo{}
	repo := resilience.NewResilientRepository(inner, resilience.DefaultConfig())
	defer func() { _ = repo.Close() }()

	_, _ = repo.SearchTranscripts(context.Background(), "query", domain.ListFilter{})
	if !inner.searchCalled {
		t.Error("expected inner SearchTranscripts to be called")
	}
}

func TestResilientRepository_GetActionItems(t *testing.T) {
	inner := &stubRepo{}
	repo := resilience.NewResilientRepository(inner, resilience.DefaultConfig())
	defer func() { _ = repo.Close() }()

	_, _ = repo.GetActionItems(context.Background(), "m-1")
	if !inner.getActionItemsCalled {
		t.Error("expected inner GetActionItems to be called")
	}
}

func TestResilientRepository_Sync(t *testing.T) {
	inner := &stubRepo{}
	repo := resilience.NewResilientRepository(inner, resilience.DefaultConfig())
	defer func() { _ = repo.Close() }()

	_, _ = repo.Sync(context.Background(), nil)
	if !inner.syncCalled {
		t.Error("expected inner Sync to be called")
	}
}

func TestResilientRepository_Close(t *testing.T) {
	inner := &stubRepo{}
	repo := resilience.NewResilientRepository(inner, resilience.DefaultConfig())

	if err := repo.Close(); err != nil {
		t.Errorf("unexpected close error: %v", err)
	}
}

func TestResilientRepository_CancelledContext(t *testing.T) {
	inner := &stubRepo{}
	repo := resilience.NewResilientRepository(inner, resilience.DefaultConfig())
	defer func() { _ = repo.Close() }()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := repo.FindByID(ctx, "m-1")
	if err == nil {
		t.Error("expected error for cancelled context")
	}
}
