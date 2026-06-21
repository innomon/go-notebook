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

// Notebook represents a project container in the database
type Notebook struct {
	ID          *models.RecordID `json:"id,omitempty"`
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Archived    bool             `json:"archived"`
	Created     time.Time        `json:"created,omitempty"`
	Updated     time.Time        `json:"updated,omitempty"`
}

// NotebookResponse represents a notebook as serialized for the REST API
type NotebookResponse struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Archived    bool      `json:"archived"`
	Created     time.Time `json:"created"`
	Updated     time.Time `json:"updated"`
	SourceCount int       `json:"source_count"`
	NoteCount   int       `json:"note_count"`
}

// NotebookDeletePreview represents counts of items affected by a deletion
type NotebookDeletePreview struct {
	NoteCount            int `json:"note_count"`
	ExclusiveSourceCount int `json:"exclusive_source_count"`
	SharedSourceCount    int `json:"shared_source_count"`
}

// GetNotebook retrieves a notebook by record ID
func GetNotebook(ctx context.Context, id string) (*Notebook, error) {
	recordID := db.EnsureRecordID("notebook", id)
	results, err := db.RepoQuery[Notebook](ctx, "SELECT * FROM ONLY $id;", map[string]any{"id": recordID})
	if err != nil {
		return nil, err
	}
	if results == nil || results.ID == nil {
		return nil, errors.New("notebook not found")
	}
	return results, nil
}

// ListNotebooks retrieves all notebooks sorted by updated timestamp
func ListNotebooks(ctx context.Context) ([]Notebook, error) {
	results, err := db.RepoQuery[[]Notebook](ctx, "SELECT * FROM notebook ORDER BY updated DESC;", nil)
	if err != nil {
		return nil, err
	}
	if results == nil {
		return []Notebook{}, nil
	}
	return *results, nil
}

// CreateNotebook creates a new notebook in SurrealDB
func CreateNotebook(ctx context.Context, name, description string) (*Notebook, error) {
	if name == "" {
		return nil, errors.New("notebook name cannot be empty")
	}

	data := map[string]any{
		"name":        name,
		"description": description,
		"archived":    false,
	}

	return db.RepoCreate[Notebook](ctx, "notebook", data)
}

// UpdateNotebook updates notebook properties
func UpdateNotebook(ctx context.Context, id string, name, description *string, archived *bool) (*Notebook, error) {
	data := make(map[string]any)
	if name != nil {
		if *name == "" {
			return nil, errors.New("notebook name cannot be empty")
		}
		data["name"] = *name
	}
	if description != nil {
		data["description"] = *description
	}
	if archived != nil {
		data["archived"] = *archived
	}

	return db.RepoUpdate[Notebook](ctx, "notebook", id, data)
}

// GetNotebookCounts retrieves source and note counts for a notebook
func GetNotebookCounts(ctx context.Context, id string) (sourceCount int, noteCount int, err error) {
	recordID := db.EnsureRecordIDString("notebook", id)

	// Count sources linked via `reference` relation
	type CountResult struct {
		Count int `json:"count"`
	}
	sourceRes, err := db.RepoQuery[[]CountResult](ctx, "SELECT count() as count FROM reference WHERE out = $id GROUP ALL;", map[string]any{"id": recordID})
	if err == nil && sourceRes != nil && len(*sourceRes) > 0 {
		sourceCount = (*sourceRes)[0].Count
	}

	// Count notes linked via `artifact` relation
	noteRes, err := db.RepoQuery[[]CountResult](ctx, "SELECT count() as count FROM artifact WHERE out = $id GROUP ALL;", map[string]any{"id": recordID})
	if err == nil && noteRes != nil && len(*noteRes) > 0 {
		noteCount = (*noteRes)[0].Count
	}

	return sourceCount, noteCount, nil
}

// GetNotebookDeletePreview calculates cascade delete statistics for a notebook
func GetNotebookDeletePreview(ctx context.Context, id string) (*NotebookDeletePreview, error) {
	recordID := db.EnsureRecordIDString("notebook", id)

	// Verify notebook exists
	if _, err := GetNotebook(ctx, id); err != nil {
		return nil, err
	}

	// 1. Count notes linked via `artifact`
	type CountResult struct {
		Count int `json:"count"`
	}
	noteRes, err := db.RepoQuery[[]CountResult](ctx, "SELECT count() as count FROM artifact WHERE out = $id GROUP ALL;", map[string]any{"id": recordID})
	noteCount := 0
	if err == nil && noteRes != nil && len(*noteRes) > 0 {
		noteCount = (*noteRes)[0].Count
	}

	// 2. Count exclusive vs shared sources
	// A source is exclusive if it has no reference to any other notebook
	type SourceRelCount struct {
		ID             *models.RecordID `json:"id"`
		AssignedOthers int              `json:"assigned_others"`
	}

	query := `
		SELECT
			id,
			count(->reference[WHERE out != $notebook_id].out) as assigned_others
		FROM (SELECT VALUE in FROM reference WHERE out = $notebook_id)
	`
	counts, err := db.RepoQuery[[]SourceRelCount](ctx, query, map[string]any{"notebook_id": recordID})
	if err != nil {
		return nil, fmt.Errorf("failed to compute source relationship counts: %w", err)
	}

	exclusiveCount := 0
	sharedCount := 0
	if counts != nil {
		for _, item := range *counts {
			if item.AssignedOthers == 0 {
				exclusiveCount++
			} else {
				sharedCount++
			}
		}
	}

	return &NotebookDeletePreview{
		NoteCount:            noteCount,
		ExclusiveSourceCount: exclusiveCount,
		SharedSourceCount:    sharedCount,
	}, nil
}

// DeleteNotebook deletes a notebook, cascades to notes, and unlinks/deletes sources
func DeleteNotebook(ctx context.Context, id string, deleteExclusiveSources bool) (deletedNotes int, deletedSources int, unlinkedSources int, err error) {
	recordID := db.EnsureRecordIDString("notebook", id)

	// Verify notebook exists
	if _, err := GetNotebook(ctx, id); err != nil {
		return 0, 0, 0, err
	}

	// 1. Fetch and delete all notes linked to this notebook
	type NoteLink struct {
		NoteID *models.RecordID `json:"in"`
	}
	noteLinks, err := db.RepoQuery[[]NoteLink](ctx, "SELECT in FROM artifact WHERE out = $id;", map[string]any{"id": recordID})
	if err == nil && noteLinks != nil {
		for _, link := range *noteLinks {
			if link.NoteID != nil {
				// Delete note record (SurrealDB event triggers downstream cleanup of note embeds)
				if err := db.RepoDelete(ctx, link.NoteID.String()); err == nil {
					deletedNotes++
				} else {
					log.Printf("[DB] Warning: failed to delete note %s: %v", link.NoteID.String(), err)
				}
			}
		}
	}

	// Delete artifact relations
	_, _ = db.RepoQuery[any](ctx, "DELETE artifact WHERE out = $id;", map[string]any{"id": recordID})

	// 2. Handle sources
	if deleteExclusiveSources {
		type SourceRelCount struct {
			ID             *models.RecordID `json:"id"`
			AssignedOthers int              `json:"assigned_others"`
		}
		query := `
			SELECT
				id,
				count(->reference[WHERE out != $notebook_id].out) as assigned_others
			FROM (SELECT VALUE in FROM reference WHERE out = $notebook_id)
		`
		counts, err := db.RepoQuery[[]SourceRelCount](ctx, query, map[string]any{"notebook_id": recordID})
		if err == nil && counts != nil {
			for _, item := range *counts {
				if item.ID != nil {
					if item.AssignedOthers == 0 {
						// Exclusive source - delete it
						if err := db.RepoDelete(ctx, item.ID.String()); err == nil {
							deletedSources++
						} else {
							log.Printf("[DB] Warning: failed to delete exclusive source %s: %v", item.ID.String(), err)
						}
					} else {
						unlinkedSources++
					}
				}
			}
		}
	} else {
		// Just count sources to be unlinked
		type CountResult struct {
			Count int `json:"count"`
		}
		res, err := db.RepoQuery[[]CountResult](ctx, "SELECT count() as count FROM reference WHERE out = $id GROUP ALL;", map[string]any{"id": recordID})
		if err == nil && res != nil && len(*res) > 0 {
			unlinkedSources = (*res)[0].Count
		}
	}

	// Delete reference relations
	_, _ = db.RepoQuery[any](ctx, "DELETE reference WHERE out = $id;", map[string]any{"id": recordID})

	// 3. Delete the notebook record itself
	if err := db.RepoDelete(ctx, recordID); err != nil {
		return 0, 0, 0, fmt.Errorf("failed to delete notebook record: %w", err)
	}

	return deletedNotes, deletedSources, unlinkedSources, nil
}
