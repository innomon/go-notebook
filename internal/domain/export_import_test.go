package domain

import (
	"archive/zip"
	"bytes"
	"context"
	"go-notebook/internal/db"
	"io"
	"os"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestExportImportRoundTrip(t *testing.T) {
	// Configure test DB settings
	os.Setenv("SURREAL_URL", "ws://localhost:8000/rpc")
	os.Setenv("SURREAL_USER", "root")
	os.Setenv("SURREAL_PASSWORD", "root")
	os.Setenv("SURREAL_NAMESPACE", "open_notebook_test")
	os.Setenv("SURREAL_DATABASE", "open_notebook_test")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := db.Init(ctx); err != nil {
		t.Skipf("skipping database integration test: SurrealDB offline: %v", err)
	}
	defer db.Close(ctx)

	// Clean test database and run migrations
	_, _ = db.RepoQuery[any](ctx, "REMOVE DATABASE open_notebook_test;", nil)
	_, _ = db.RepoQuery[any](ctx, "DEFINE DATABASE open_notebook_test;", nil)
	_ = db.RunMigrationUp(ctx)

	// 1. Create a test notebook, source, note, and graph RAG structures
	nb, err := CreateNotebook(ctx, "Source Vault", "Test export-import round-trip functionality")
	if err != nil {
		t.Fatalf("failed to create notebook: %v", err)
	}
	nbID := nb.ID.String()

	// Create source directly
	srcData := map[string]any{
		"title":     "Machine Learning Guide",
		"full_text": "Machine learning is a subset of artificial intelligence.",
		"hash":      "ml_guide_hash_123",
		"topics":    []string{"ML", "AI"},
	}
	src, err := db.RepoCreate[Source](ctx, "source", srcData)
	if err != nil {
		t.Fatalf("failed to create source: %v", err)
	}
	err = LinkSourceToNotebook(ctx, src.ID.String(), nbID)
	if err != nil {
		t.Fatalf("failed to link source: %v", err)
	}

	// Create note directly
	_, _, err = CreateNote(ctx, "Meeting Notes", "Discuss ML model progress and artificial intelligence goals.", "human", nbID)
	if err != nil {
		t.Fatalf("failed to create note: %v", err)
	}

	// Create entities and relation
	entML, err := CreateOrUpdateEntity(ctx, nbID, src.ID.String(), "machine learning")
	if err != nil {
		t.Fatalf("failed to create entity: %v", err)
	}
	entAI, err := CreateOrUpdateEntity(ctx, nbID, src.ID.String(), "artificial intelligence")
	if err != nil {
		t.Fatalf("failed to create entity: %v", err)
	}
	err = RelateEntities(ctx, nbID, src.ID.String(), entML, entAI)
	if err != nil {
		t.Fatalf("failed to relate entities: %v", err)
	}

	// 2. Export Notebook to Zip
	var zipBuf bytes.Buffer
	err = ExportNotebookToZip(ctx, nbID, &zipBuf)
	if err != nil {
		t.Fatalf("ExportNotebookToZip failed: %v", err)
	}

	zipBytes := zipBuf.Bytes()
	zipSize := int64(len(zipBytes))

	// 3. Inspect ZIP contents
	zr, err := zip.NewReader(bytes.NewReader(zipBytes), zipSize)
	if err != nil {
		t.Fatalf("failed to open exported zip: %v", err)
	}

	expectedFiles := map[string]bool{
		"metadata.json":                     false,
		"sources/Machine Learning Guide.md": false,
		"notes/Meeting Notes.md":            false,
		"entities/machine learning.md":      false,
		"entities/artificial intelligence.md": false,
		"graph.json":                        false,
	}

	for _, f := range zr.File {
		if _, ok := expectedFiles[f.Name]; ok {
			expectedFiles[f.Name] = true
		}
	}

	for fName, found := range expectedFiles {
		if !found {
			t.Errorf("expected file %s not found in ZIP", fName)
		}
	}

	// Verify wrapEntitiesInText formatting (it should wrap "machine learning" and "artificial intelligence" in double brackets)
	for _, f := range zr.File {
		if f.Name == "sources/Machine Learning Guide.md" {
			rc, _ := f.Open()
			contentBytes, _ := io.ReadAll(rc)
			rc.Close()
			content := string(contentBytes)
			if !strings.Contains(content, "[[Machine learning]]") {
				t.Errorf("expected entities to be wrapped, got content: %s", content)
			}
		}
	}

	// 4. Import zip as a NEW notebook
	importedNb, err := ImportNewNotebookFromZip(ctx, bytes.NewReader(zipBytes), zipSize)
	if err != nil {
		t.Fatalf("ImportNewNotebookFromZip failed: %v", err)
	}

	importedNbID := importedNb.ID.String()
	if importedNb.Name != "Source Vault (1)" {
		t.Errorf("expected resolved suffix name Source Vault (1), got %q", importedNb.Name)
	}
	// Verify imported sources
	impSources, err := GetNotebookSources(ctx, importedNbID)
	if err != nil {
		t.Fatalf("failed to fetch imported sources: %v", err)
	}
	if len(impSources) != 1 {
		t.Errorf("expected 1 imported source, got %d", len(impSources))
	} else {
		if impSources[0].Title != "Machine Learning Guide" {
			t.Errorf("expected imported source title Machine Learning Guide, got %q", impSources[0].Title)
		}
	}

	// Verify imported notes
	impNotes, err := ListNotebookNotes(ctx, importedNbID)
	if err != nil {
		t.Fatalf("failed to fetch imported notes: %v", err)
	}
	if len(impNotes) != 1 {
		t.Errorf("expected 1 imported note, got %d", len(impNotes))
	} else {
		if impNotes[0].Title != "Meeting Notes" {
			t.Errorf("expected imported note title Meeting Notes, got %q", impNotes[0].Title)
		}
	}

	// Verify imported graph structures
	graphData, err := GetNotebookGraphData(ctx, importedNbID, 10)
	if err != nil {
		t.Fatalf("failed to fetch imported graph data: %v", err)
	}
	if len(graphData.TopNodes) != 2 {
		t.Errorf("expected 2 graph nodes, got %d: %v", len(graphData.TopNodes), graphData.TopNodes)
	}
	if len(graphData.Connections) != 1 {
		t.Errorf("expected 1 connection between entities, got %d", len(graphData.Connections))
	}

	// 5. Test merging and conflict strategy into target notebook
	// If we import again into the target notebook:
	err = ImportMergeNotebookFromZip(ctx, importedNbID, bytes.NewReader(zipBytes), zipSize)
	if err != nil {
		t.Fatalf("ImportMergeNotebookFromZip duplicate import failed: %v", err)
	}

	// Since hashes matched, duplicate source should be skipped
	impSourcesMerged, _ := GetNotebookSources(ctx, importedNbID)
	if len(impSourcesMerged) != 1 {
		t.Errorf("expected source count to stay 1 after merging duplicate hash source, got %d", len(impSourcesMerged))
	}

	// If we import a modified zip with same name but different content (different hash):
	// Let's modify the ZIP content in-memory to test suffix resolution
	var modZipBuf bytes.Buffer
	zw := zip.NewWriter(&modZipBuf)

	// Copy all files except sources/Machine Learning Guide.md
	for _, f := range zr.File {
		if f.Name == "sources/Machine Learning Guide.md" {
			nf, _ := zw.Create(f.Name)
			nf.Write([]byte("Different content for machine learning to change the hash value."))
		} else {
			nf, _ := zw.Create(f.Name)
			rc, _ := f.Open()
			io.Copy(nf, rc)
			rc.Close()
		}
	}
	zw.Close()

	modZipBytes := modZipBuf.Bytes()
	err = ImportMergeNotebookFromZip(ctx, importedNbID, bytes.NewReader(modZipBytes), int64(len(modZipBytes)))
	if err != nil {
		t.Fatalf("ImportMergeNotebookFromZip conflict import failed: %v", err)
	}

	// Since hashes differed but names conflicted, suffix resolver should produce Machine Learning Guide_1
	impSourcesSuffix, _ := GetNotebookSources(ctx, importedNbID)
	if len(impSourcesSuffix) != 2 {
		t.Errorf("expected source count to be 2 after merging name conflict with different hash, got %d", len(impSourcesSuffix))
	}

	// Let's check titles of sources
	titles := []string{impSourcesSuffix[0].Title, impSourcesSuffix[1].Title}
	sort.Strings(titles)
	if titles[0] != "Machine Learning Guide" || titles[1] != "Machine Learning Guide_1" {
		t.Errorf("expected resolved titles to be Machine Learning Guide and Machine Learning Guide_1, got: %v", titles)
	}
}
