package domain

import (
	"context"
	"errors"
	"go-notebook/internal/db"
	"time"

	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// Transformation represents a custom prompt transformation
type Transformation struct {
	ID           *models.RecordID `json:"id,omitempty"`
	Name         string           `json:"name"`
	Title        string           `json:"title"`
	Description  string           `json:"description"`
	Prompt       string           `json:"prompt"`
	ApplyDefault bool             `json:"apply_default"`
	Created      time.Time        `json:"created,omitempty"`
	Updated      time.Time        `json:"updated,omitempty"`
}

// TransformationResponse represents a Transformation as serialized for the REST API
type TransformationResponse struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	Prompt       string    `json:"prompt"`
	ApplyDefault bool      `json:"apply_default"`
	Created      time.Time `json:"created"`
	Updated      time.Time `json:"updated"`
}

// GetTransformation retrieves a transformation by ID
func GetTransformation(ctx context.Context, id string) (*Transformation, error) {
	recordID := db.EnsureRecordIDString("transformation", id)
	results, err := db.RepoQuery[Transformation](ctx, "SELECT * FROM ONLY $id;", map[string]any{"id": recordID})
	if err != nil {
		return nil, err
	}
	return results, nil
}

// ListTransformations retrieves all transformations sorted by name
func ListTransformations(ctx context.Context) ([]Transformation, error) {
	results, err := db.RepoQuery[[]Transformation](ctx, "SELECT * FROM transformation ORDER BY name ASC;", nil)
	if err != nil {
		return nil, err
	}
	if results == nil {
		return []Transformation{}, nil
	}
	return *results, nil
}

// CreateTransformation creates a new transformation record
func CreateTransformation(ctx context.Context, name, title, description, prompt string, applyDefault bool) (*Transformation, error) {
	if name == "" {
		return nil, errors.New("transformation name cannot be empty")
	}

	data := map[string]any{
		"name":          name,
		"title":         title,
		"description":   description,
		"prompt":        prompt,
		"apply_default": applyDefault,
	}

	return db.RepoCreate[Transformation](ctx, "transformation", data)
}

// UpdateTransformation updates an existing transformation record
func UpdateTransformation(ctx context.Context, id string, name, title, description, prompt *string, applyDefault *bool) (*Transformation, error) {
	data := make(map[string]any)
	if name != nil {
		if *name == "" {
			return nil, errors.New("transformation name cannot be empty")
		}
		data["name"] = *name
	}
	if title != nil {
		data["title"] = *title
	}
	if description != nil {
		data["description"] = *description
	}
	if prompt != nil {
		data["prompt"] = *prompt
	}
	if applyDefault != nil {
		data["apply_default"] = *applyDefault
	}

	return db.RepoUpdate[Transformation](ctx, "transformation", id, data)
}

// DeleteTransformation deletes a transformation record by ID
func DeleteTransformation(ctx context.Context, id string) error {
	recordID := db.EnsureRecordIDString("transformation", id)
	return db.RepoDelete(ctx, recordID)
}
