package okf

import (
	"context"
	"go-notebook/internal/db"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWorkspaceIndexer(t *testing.T) {
	// Configure test DB settings
	os.Setenv("SURREAL_URL", "ws://localhost:8000/rpc")
	os.Setenv("SURREAL_USER", "root")
	os.Setenv("SURREAL_PASSWORD", "root")
	os.Setenv("SURREAL_NAMESPACE", "open_notebook_test")
	os.Setenv("SURREAL_DATABASE", "open_notebook_test")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := db.Init(ctx); err != nil {
		t.Skip("SurrealDB offline")
	}
	defer db.Close(ctx)

	// Clean test database and run migrations
	_, _ = db.RepoQuery[any](ctx, "REMOVE DATABASE open_notebook_test;", nil)
	_, _ = db.RepoQuery[any](ctx, "DEFINE DATABASE open_notebook_test;", nil)
	_ = db.RunMigrationUp(ctx)

	// Setup a temporary workspace directory
	tmpDir := t.TempDir()

	file1 := filepath.Join(tmpDir, "note1.md")
	content1 := "---\ntype: Concept\ntitle: Note One\ndescription: First test note.\n---\nHere is some body link: [Note Two](./sub/note2.md)"
	if err := os.WriteFile(file1, []byte(content1), 0644); err != nil {
		t.Fatalf("failed to create note1: %v", err)
	}

	subDir := filepath.Join(tmpDir, "sub")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}

	file2 := filepath.Join(subDir, "note2.md")
	content2 := "---\ntype: Concept\ntitle: Note Two\ndescription: Second test note.\n---\nThis is body content."
	if err := os.WriteFile(file2, []byte(content2), 0644); err != nil {
		t.Fatalf("failed to create note2: %v", err)
	}

	// Initialize the indexer
	indexer := NewWorkspaceIndexer(tmpDir)
	if err := indexer.Index(ctx); err != nil {
		t.Fatalf("Index failed: %v", err)
	}

	// Retrieve the graph and verify nodes & links are in database
	graph, err := indexer.GetGraph(ctx)
	if err != nil {
		t.Fatalf("failed to fetch graph: %v", err)
	}

	if len(graph) != 2 {
		t.Errorf("expected 2 nodes in graph, got %d", len(graph))
	}

	// Verify links resolution (note1.md -> sub/note2.md)
	var foundLink bool
	for _, node := range graph {
		if node.ID == "note1.md" {
			for _, outbound := range node.OutboundLinks {
				if outbound == "sub/note2.md" {
					foundLink = true
				}
			}
		}
	}
	if !foundLink {
		t.Error("expected link from note1.md to sub/note2.md to be resolved and recorded")
	}

	// Test incremental indexing / hash caching
	// Modify note1.md and check if it gets updated
	newContent := "---\ntype: Concept\ntitle: Note One\ndescription: Updated description.\n---\nNo links now."
	if err := os.WriteFile(file1, []byte(newContent), 0644); err != nil {
		t.Fatalf("failed to update note1: %v", err)
	}

	if err := indexer.Index(ctx); err != nil {
		t.Fatalf("Index after modification failed: %v", err)
	}

	graphUpdated, _ := indexer.GetGraph(ctx)
	for _, node := range graphUpdated {
		if node.ID == "note1.md" {
			if node.Metadata.Description != "Updated description." {
				t.Errorf("expected updated description, got %q", node.Metadata.Description)
			}
			if len(node.OutboundLinks) != 0 {
				t.Errorf("expected link to be removed, got: %v", node.OutboundLinks)
			}
		}
	}

	// Test removal of deleted files
	if err := os.Remove(file2); err != nil {
		t.Fatalf("failed to remove note2: %v", err)
	}

	if err := indexer.Index(ctx); err != nil {
		t.Fatalf("Index after removal failed: %v", err)
	}

	graphFinal, _ := indexer.GetGraph(ctx)
	if len(graphFinal) != 1 {
		t.Errorf("expected 1 node after file deletion, got %d", len(graphFinal))
	}
}

func TestWatcherIncrementalUpdates(t *testing.T) {
	// Configure test DB settings
	os.Setenv("SURREAL_URL", "ws://localhost:8000/rpc")
	os.Setenv("SURREAL_USER", "root")
	os.Setenv("SURREAL_PASSWORD", "root")
	os.Setenv("SURREAL_NAMESPACE", "open_notebook_test")
	os.Setenv("SURREAL_DATABASE", "open_notebook_test")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := db.Init(ctx); err != nil {
		t.Skip("SurrealDB offline")
	}
	defer db.Close(ctx)

	tmpDir := t.TempDir()

	// Start a watcher pool manager
	wm := NewWatcherManager()
	defer wm.Close()

	indexer := NewWorkspaceIndexer(tmpDir)
	if err := indexer.Index(ctx); err != nil {
		t.Fatalf("initial indexing failed: %v", err)
	}

	// Dynamic watch registration
	err := wm.Watch(ctx, tmpDir, indexer)
	if err != nil {
		t.Fatalf("failed to start watcher: %v", err)
	}

	// Create a new file, watcher should detect it and add to DB
	newFile := filepath.Join(tmpDir, "watch_note.md")
	noteContent := "---\ntype: WatcherTest\ntitle: Watch Note\ndescription: Created during watching.\n---\nWatcher body."
	if err := os.WriteFile(newFile, []byte(noteContent), 0644); err != nil {
		t.Fatalf("failed to write watcher file: %v", err)
	}

	// Sleep briefly for fsnotify event propagation
	time.Sleep(500 * time.Millisecond)

	graph, _ := indexer.GetGraph(ctx)
	var found bool
	for _, n := range graph {
		if n.ID == "watch_note.md" {
			found = true
			if n.Metadata.Type != "WatcherTest" {
				t.Errorf("expected metadata type WatcherTest, got %q", n.Metadata.Type)
			}
		}
	}

	if !found {
		t.Error("expected watch_note.md to be created and indexed automatically by the watcher")
	}

	// Now test file deletion
	if err := os.Remove(newFile); err != nil {
		t.Fatalf("failed to delete watcher file: %v", err)
	}
	time.Sleep(500 * time.Millisecond)

	graphDeleted, _ := indexer.GetGraph(ctx)
	foundDeleted := false
	for _, n := range graphDeleted {
		if n.ID == "watch_note.md" {
			foundDeleted = true
		}
	}
	if foundDeleted {
		t.Error("expected watch_note.md to be deleted from database after physical deletion")
	}

	// Now test invalid document frontmatter (should be removed from DB graph)
	invalidContent := "this is invalid document content with no yaml frontmatter"
	if err := os.WriteFile(newFile, []byte(invalidContent), 0644); err != nil {
		t.Fatalf("failed to write invalid file: %v", err)
	}
	time.Sleep(500 * time.Millisecond)

	graphInvalid, _ := indexer.GetGraph(ctx)
	foundInvalid := false
	for _, n := range graphInvalid {
		if n.ID == "watch_note.md" {
			foundInvalid = true
		}
	}
	if foundInvalid {
		t.Error("expected watch_note.md to be removed from database because it became invalid")
	}
}

func TestWatcherReaperAndTouch(t *testing.T) {
	// Configure test DB settings
	os.Setenv("SURREAL_URL", "ws://localhost:8000/rpc")
	os.Setenv("SURREAL_USER", "root")
	os.Setenv("SURREAL_PASSWORD", "root")
	os.Setenv("SURREAL_NAMESPACE", "open_notebook_test")
	os.Setenv("SURREAL_DATABASE", "open_notebook_test")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := db.Init(ctx); err != nil {
		t.Skip("SurrealDB offline")
	}
	defer db.Close(ctx)

	tmpDir := t.TempDir()
	indexer := NewWorkspaceIndexer(tmpDir)

	wm := NewWatcherManager()
	wm.reapInterval = 100 * time.Millisecond
	wm.timeout = 200 * time.Millisecond
	defer wm.Close()

	err := wm.Watch(ctx, tmpDir, indexer)
	if err != nil {
		t.Fatalf("Watch failed: %v", err)
	}

	wm.mu.Lock()
	_, existsBefore := wm.instances[tmpDir]
	wm.mu.Unlock()
	if !existsBefore {
		t.Fatal("expected watcher instance to exist")
	}

	// Touch to keep it alive
	time.Sleep(50 * time.Millisecond)
	wm.Touch(tmpDir)

	// Wait longer than timeout
	time.Sleep(400 * time.Millisecond)

	wm.mu.Lock()
	_, existsAfter := wm.instances[tmpDir]
	wm.mu.Unlock()
	if existsAfter {
		t.Error("expected watcher instance to be reaped due to inactivity")
	}
}
