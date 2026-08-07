package sqlite

import (
	"context"
	"os"
	"testing"

	"go-notebook/internal/db/repository"
)

func TestSQLiteRepositories(t *testing.T) {
	dbPath := "./test_repo.db"
	defer os.Remove(dbPath)

	factory, err := NewSQLiteFactory(dbPath)
	if err != nil {
		t.Fatalf("failed to create sqlite factory: %v", err)
	}
	defer factory.Close(context.Background())

	ctx := context.Background()

	// 1. Test Notes
	noteRepo := factory.Notes()
	createdNote, err := noteRepo.Create(ctx, &repository.NoteRecord{
		Title:   "Test Note",
		Content: "Hello SQLite",
		Folder:  "work",
		Tags:    []string{"test", "sqlite"},
	})
	if err != nil {
		t.Fatalf("failed to create note: %v", err)
	}

	fetchedNote, err := noteRepo.Get(ctx, createdNote.ID)
	if err != nil || fetchedNote.Title != "Test Note" {
		t.Fatalf("failed to fetch note or title mismatch: %v", err)
	}

	// 2. Test Vectors & Similarity
	vecRepo := factory.Vectors()
	err = vecRepo.Save(ctx, &repository.VectorRecord{
		ID:         "v1",
		SourceID:   "doc1",
		ChunkIndex: 0,
		Text:       "Go notebook vector",
		Vector:     []float32{1.0, 0.0, 0.0},
	})
	if err != nil {
		t.Fatalf("failed to save vector: %v", err)
	}

	err = vecRepo.Save(ctx, &repository.VectorRecord{
		ID:         "v2",
		SourceID:   "doc1",
		ChunkIndex: 1,
		Text:       "Other text",
		Vector:     []float32{0.0, 1.0, 0.0},
	})
	if err != nil {
		t.Fatalf("failed to save vector: %v", err)
	}

	searchRes, err := vecRepo.Search(ctx, []float32{0.9, 0.1, 0.0}, 1)
	if err != nil || len(searchRes) != 1 || searchRes[0].ID != "v1" {
		t.Fatalf("vector search failed to return closest vector v1: %v", err)
	}

	// 3. Test Settings
	settingsRepo := factory.Settings()
	err = settingsRepo.Set(ctx, "theme", "dark")
	if err != nil {
		t.Fatalf("failed to set setting: %v", err)
	}
	val, err := settingsRepo.Get(ctx, "theme")
	if err != nil || val != "dark" {
		t.Fatalf("failed to get setting or value mismatch: %v", err)
	}
}
