package localstore

import (
	"context"
	"database/sql"
	"time"

	"github.com/felixgeelhaar/granola-mcp/internal/domain/annotation"
)

// NoteRepository implements annotation.NoteRepository using SQLite.
type NoteRepository struct {
	db *sql.DB
}

// NewNoteRepository creates a new SQLite-backed note repository.
func NewNoteRepository(db *sql.DB) *NoteRepository {
	return &NoteRepository{db: db}
}

func (r *NoteRepository) Save(_ context.Context, note *annotation.AgentNote) error {
	_, err := r.db.Exec(
		"INSERT OR REPLACE INTO agent_notes (id, meeting_id, author, content, created_at) VALUES (?, ?, ?, ?, ?)",
		string(note.ID()), note.MeetingID(), note.Author(), note.Content(), note.CreatedAt().UTC(),
	)
	return err
}

func (r *NoteRepository) FindByID(_ context.Context, id annotation.NoteID) (*annotation.AgentNote, error) {
	var (
		noteID    string
		meetingID string
		author    string
		content   string
		createdAt time.Time
	)
	err := r.db.QueryRow(
		"SELECT id, meeting_id, author, content, created_at FROM agent_notes WHERE id = ?",
		string(id),
	).Scan(&noteID, &meetingID, &author, &content, &createdAt)
	if err == sql.ErrNoRows {
		return nil, annotation.ErrNoteNotFound
	}
	if err != nil {
		return nil, err
	}
	return annotation.ReconstructAgentNote(
		annotation.NoteID(noteID), meetingID, author, content, createdAt,
	), nil
}

func (r *NoteRepository) ListByMeeting(_ context.Context, meetingID string) ([]*annotation.AgentNote, error) {
	rows, err := r.db.Query(
		"SELECT id, meeting_id, author, content, created_at FROM agent_notes WHERE meeting_id = ? ORDER BY created_at ASC",
		meetingID,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var notes []*annotation.AgentNote
	for rows.Next() {
		var (
			noteID    string
			mid       string
			author    string
			content   string
			createdAt time.Time
		)
		if err := rows.Scan(&noteID, &mid, &author, &content, &createdAt); err != nil {
			return nil, err
		}
		notes = append(notes, annotation.ReconstructAgentNote(
			annotation.NoteID(noteID), mid, author, content, createdAt,
		))
	}
	if notes == nil {
		notes = []*annotation.AgentNote{}
	}
	return notes, rows.Err()
}

func (r *NoteRepository) Delete(_ context.Context, id annotation.NoteID) error {
	result, err := r.db.Exec("DELETE FROM agent_notes WHERE id = ?", string(id))
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return annotation.ErrNoteNotFound
	}
	return nil
}

var _ annotation.NoteRepository = (*NoteRepository)(nil)
