package outbox_test

import (
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/felixgeelhaar/granola-mcp/internal/infrastructure/localstore"
	"github.com/felixgeelhaar/granola-mcp/internal/infrastructure/outbox"
	_ "github.com/mattn/go-sqlite3"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := localstore.InitSchema(db); err != nil {
		t.Fatalf("init schema: %v", err)
	}
	return db
}

func TestSQLiteStore_AppendAndListPending(t *testing.T) {
	store := outbox.NewSQLiteStore(openTestDB(t))

	entry := outbox.Entry{
		ID:        "evt-1",
		EventType: "note.added",
		Payload:   []byte(`{"note_id":"n-1"}`),
		CreatedAt: time.Now().UTC(),
	}
	if err := store.Append(entry); err != nil {
		t.Fatalf("append: %v", err)
	}

	pending, err := store.ListPending()
	if err != nil {
		t.Fatalf("list pending: %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("got %d entries, want 1", len(pending))
	}
	if pending[0].ID != "evt-1" {
		t.Errorf("got id %q", pending[0].ID)
	}
	if pending[0].EventType != "note.added" {
		t.Errorf("got type %q", pending[0].EventType)
	}
	if pending[0].Status != "pending" {
		t.Errorf("got status %q", pending[0].Status)
	}
}

func TestSQLiteStore_MarkSynced(t *testing.T) {
	store := outbox.NewSQLiteStore(openTestDB(t))

	entry := outbox.Entry{
		ID:        "evt-1",
		EventType: "note.added",
		Payload:   []byte(`{}`),
		CreatedAt: time.Now().UTC(),
	}
	if err := store.Append(entry); err != nil {
		t.Fatalf("append: %v", err)
	}

	if err := store.MarkSynced("evt-1"); err != nil {
		t.Fatalf("mark synced: %v", err)
	}

	pending, err := store.ListPending()
	if err != nil {
		t.Fatalf("list pending: %v", err)
	}
	if len(pending) != 0 {
		t.Errorf("got %d pending, want 0 (synced entry should not appear)", len(pending))
	}
}

func TestSQLiteStore_MarkFailed(t *testing.T) {
	store := outbox.NewSQLiteStore(openTestDB(t))

	entry := outbox.Entry{
		ID:        "evt-1",
		EventType: "note.added",
		Payload:   []byte(`{}`),
		CreatedAt: time.Now().UTC(),
	}
	if err := store.Append(entry); err != nil {
		t.Fatalf("append: %v", err)
	}

	if err := store.MarkFailed("evt-1"); err != nil {
		t.Fatalf("mark failed: %v", err)
	}

	// Failed entries should not appear in pending list
	pending, err := store.ListPending()
	if err != nil {
		t.Fatalf("list pending: %v", err)
	}
	if len(pending) != 0 {
		t.Errorf("got %d pending, want 0 (failed entry should not appear)", len(pending))
	}
}

func TestSQLiteStore_ListPending_Empty(t *testing.T) {
	store := outbox.NewSQLiteStore(openTestDB(t))

	pending, err := store.ListPending()
	if err != nil {
		t.Fatalf("list pending: %v", err)
	}
	if len(pending) != 0 {
		t.Errorf("got %d entries, want 0", len(pending))
	}
}

func TestSQLiteStore_MultiplePending(t *testing.T) {
	store := outbox.NewSQLiteStore(openTestDB(t))

	for i, evtType := range []string{"note.added", "action_item.completed"} {
		entry := outbox.Entry{
			ID:        fmt.Sprintf("evt-%d", i),
			EventType: evtType,
			Payload:   []byte(`{}`),
			CreatedAt: time.Now().UTC(),
		}
		if err := store.Append(entry); err != nil {
			t.Fatalf("append: %v", err)
		}
	}

	pending, err := store.ListPending()
	if err != nil {
		t.Fatalf("list pending: %v", err)
	}
	if len(pending) != 2 {
		t.Errorf("got %d entries, want 2", len(pending))
	}
}

func TestMarshalEventPayload(t *testing.T) {
	data := outbox.MarshalEventPayload(map[string]string{"key": "val"})
	if string(data) != `{"key":"val"}` {
		t.Errorf("got %q", string(data))
	}
}
