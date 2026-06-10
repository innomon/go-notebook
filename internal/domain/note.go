package domain

import (
	"context"
	"errors"
	"fmt"
	"go-notebook/internal/db"
	"log"
	"time"

	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// Note represents a standalone or project-bound note
type Note struct {
	ID        *models.RecordID `json:"id,omitempty"`
	Title     string           `json:"title"`
	Content   string           `json:"content"`
	Summary   string           `json:"summary"`
	NoteType  string           `json:"note_type"` // "human" or "ai"
	Embedding []float32        `json:"embedding,omitempty"`
	Created   time.Time        `json:"created,omitempty"`
	Updated   time.Time        `json:"updated,omitempty"`
}

// NoteResponse represents a Note as serialized for the REST API
type NoteResponse struct {
	ID        string    `json:"id"`
	Title     string    `json:"title,omitempty"`
	Content   string    `json:"content,omitempty"`
	NoteType  string    `json:"note_type,omitempty"`
	Created   time.Time `json:"created"`
	Updated   time.Time `json:"updated"`
	CommandID string    `json:"command_id,omitempty"` // For embedding job tracking
}

// GetNote retrieves a note by ID
func GetNote(ctx context.Context, id string) (*Note, error) {
	recordID := db.EnsureRecordIDString("note", id)
	results, err := db.RepoQuery[Note](ctx, "SELECT * FROM ONLY $id;", map[string]any{"id": recordID})
	if err != nil {
		return nil, err
	}
	return results, nil
}

// ListNotebookNotes retrieves all notes related to a specific notebook
func ListNotebookNotes(ctx context.Context, notebookID string) ([]Note, error) {
	recordID := db.EnsureRecordIDString("notebook", notebookID)

	type ArtifactLink struct {
		Note Note `json:"note"`
	}

	query := `
		SELECT * omit note.content, note.embedding from (
			SELECT in as note FROM artifact WHERE out = $notebook_id
			FETCH note
		) ORDER BY note.updated DESC;
	`
	links, err := db.RepoQuery[[]ArtifactLink](ctx, query, map[string]any{"notebook_id": recordID})
	if err != nil {
		return nil, fmt.Errorf("failed to list notebook notes: %w", err)
	}

	if links == nil {
		return []Note{}, nil
	}

	notes := make([]Note, len(*links))
	for i, l := range *links {
		notes[i] = l.Note
	}
	return notes, nil
}

// CreateNote creates a new note, links it to a notebook (if provided), and schedules embedding
func CreateNote(ctx context.Context, title, content, noteType, notebookID string) (*Note, string, error) {
	if content == "" {
		return nil, "", errors.New("note content cannot be empty")
	}
	if noteType == "" {
		noteType = "human"
	}

	data := map[string]any{
		"title":     title,
		"content":   content,
		"note_type": noteType,
	}

	note, err := db.RepoCreate[Note](ctx, "note", data)
	if err != nil {
		return nil, "", err
	}

	// Link to notebook if notebookID is specified
	if notebookID != "" {
		noteStr := note.ID.String()
		notebookStr := db.EnsureRecordIDString("notebook", notebookID)
		err = db.RepoRelate(ctx, noteStr, "artifact", notebookStr, nil)
		if err != nil {
			// Clean up created note if relationship fails
			_ = db.RepoDelete(ctx, noteStr)
			return nil, "", fmt.Errorf("failed to link note to notebook: %w", err)
		}
	}

	// Trigger async embedding command
	commandID, err := submitEmbedNoteCommand(ctx, note.ID.String())
	if err != nil {
		// Log warning but don't fail note creation
		log.Printf("[Note] Warning: failed to submit embed note command: %v", err)
	}

	return note, commandID, nil
}

// UpdateNote updates a note and triggers re-embedding
func UpdateNote(ctx context.Context, id string, title, content, noteType *string) (*Note, string, error) {
	data := make(map[string]any)
	if title != nil {
		data["title"] = *title
	}
	if content != nil {
		data["content"] = *content
	}
	if noteType != nil {
		data["note_type"] = *noteType
	}

	note, err := db.RepoUpdate[Note](ctx, "note", id, data)
	if err != nil {
		return nil, "", err
	}

	// Trigger async embedding command
	commandID, err := submitEmbedNoteCommand(ctx, note.ID.String())
	if err != nil {
		log.Printf("[Note] Warning: failed to submit embed note command for updated note: %v", err)
	}

	return note, commandID, nil
}

// DeleteNote deletes a note and its database relationships
func DeleteNote(ctx context.Context, id string) error {
	recordID := db.EnsureRecordIDString("note", id)

	// Delete artifact relations
	_, _ = db.RepoQuery[any](ctx, "DELETE artifact WHERE in = $id;", map[string]any{"id": recordID})

	// Delete note
	return db.RepoDelete(ctx, recordID)
}

// Helper to submit the embedding background job (Stubs for now, will implement command workers in Phase 5)
func submitEmbedNoteCommand(ctx context.Context, noteID string) (string, error) {
	// Replicates `submit_command("embed_note", note_id)`
	// Insert a pending job into the `command` table in SurrealDB
	nowStr := time.Now().UTC().Format(time.RFC3339)
	jobData := map[string]any{
		"app":            "open_notebook",
		"command":        "embed_note",
		"status":         "pending",
		"created":        nowStr,
		"updated":        nowStr,
		"retry_attempts": 0,
		"input": map[string]any{
			"note_id": noteID,
		},
	}

	type CommandRecord struct {
		ID *models.RecordID `json:"id"`
	}

	res, err := db.RepoCreate[CommandRecord](ctx, "command", jobData)
	if err != nil {
		return "", err
	}

	return res.ID.String(), nil
}

// GetContext returns context representation of note
func (n *Note) GetContext(contextSize string) map[string]any {
	res := map[string]any{
		"id":    n.ID.String(),
		"title": n.Title,
	}
	if contextSize == "long" {
		res["content"] = n.Content
	} else {
		shortContent := n.Content
		if len(shortContent) > 100 {
			shortContent = shortContent[:100]
		}
		res["content"] = shortContent
	}
	return res
}

