package localstore_test

import (
	"database/sql"
	"testing"

	"github.com/felixgeelhaar/granola-mcp/internal/infrastructure/localstore"
	_ "github.com/mattn/go-sqlite3"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestInitSchema_CreatesAllTables(t *testing.T) {
	db := openTestDB(t)
	if err := localstore.InitSchema(db); err != nil {
		t.Fatalf("init schema: %v", err)
	}

	tables := []string{"agent_notes", "action_item_overrides", "outbox_entries"}
	for _, table := range tables {
		var name string
		err := db.QueryRow(
			"SELECT name FROM sqlite_master WHERE type='table' AND name=?", table,
		).Scan(&name)
		if err != nil {
			t.Errorf("table %q not found: %v", table, err)
		}
	}
}

func TestInitSchema_Idempotent(t *testing.T) {
	db := openTestDB(t)
	if err := localstore.InitSchema(db); err != nil {
		t.Fatalf("first init: %v", err)
	}
	if err := localstore.InitSchema(db); err != nil {
		t.Fatalf("second init should be idempotent: %v", err)
	}
}
