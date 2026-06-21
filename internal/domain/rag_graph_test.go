package domain

import (
	"context"
	"encoding/json"
	"go-notebook/internal/db"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/surrealdb/surrealdb.go/pkg/models"
)

func TestRAGEntitySerialization(t *testing.T) {
	nbID := models.NewRecordID("notebook", "proj123")
	srcID1 := models.NewRecordID("source", "docA")
	srcID2 := models.NewRecordID("source", "docB")
	entID := models.NewRecordID("RAGEntity", "entityX")

	entity := RAGEntity{
		ID:       &entID,
		Name:     "artificial intelligence",
		Count:    2,
		Sources:  []*models.RecordID{&srcID1, &srcID2},
		Notebook: &nbID,
		Created:  time.Now(),
	}

	// Test Marshal JSON
	data, err := json.Marshal(entity)
	if err != nil {
		t.Fatalf("failed to marshal RAGEntity: %v", err)
	}

	jsonStr := string(data)
	if !strings.Contains(jsonStr, `"sources"`) {
		t.Errorf("expected json string to contain sources field, got: %s", jsonStr)
	}
	if !strings.Contains(jsonStr, `"Table":"source"`) || !strings.Contains(jsonStr, `"ID":"docA"`) {
		t.Errorf("expected json string to contain source Table and ID representation, got: %s", jsonStr)
	}

	// Test Unmarshal JSON
	var unmarshaled RAGEntity
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("failed to unmarshal RAGEntity: %v", err)
	}

	if unmarshaled.Name != entity.Name {
		t.Errorf("expected name %q, got %q", entity.Name, unmarshaled.Name)
	}

	if len(unmarshaled.Sources) != 2 {
		t.Errorf("expected 2 sources, got %d", len(unmarshaled.Sources))
	} else {
		if unmarshaled.Sources[0].String() != srcID1.String() {
			t.Errorf("expected first source to be %s, got %s", srcID1, unmarshaled.Sources[0])
		}
	}
}

func TestGraphOperationsLineage(t *testing.T) {
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

	// Clean test namespace database
	_, _ = db.RepoQuery[any](ctx, "REMOVE DATABASE open_notebook_test;", nil)
	_, _ = db.RepoQuery[any](ctx, "DEFINE DATABASE open_notebook_test;", nil)
	_ = db.RunMigrationUp(ctx)

	notebookID := "nb_test_123"
	sourceID1 := "src_test_A"
	sourceID2 := "src_test_B"

	// 1. Create or Update Entity (First Ingestion)
	ent1, err := CreateOrUpdateEntity(ctx, notebookID, sourceID1, "artificial intelligence")
	if err != nil {
		t.Fatalf("CreateOrUpdateEntity failed: %v", err)
	}

	if ent1.Count != 1 {
		t.Errorf("expected count 1, got %d", ent1.Count)
	}
	if len(ent1.Sources) != 1 || ent1.Sources[0].ID != sourceID1 {
		t.Errorf("expected sources array to contain %q, got: %v", sourceID1, ent1.Sources)
	}

	// 2. Create or Update Entity (Second Ingestion - same source, should not duplicate but count stays 1)
	ent1Dup, err := CreateOrUpdateEntity(ctx, notebookID, sourceID1, "artificial intelligence")
	if err != nil {
		t.Fatalf("CreateOrUpdateEntity dup failed: %v", err)
	}
	if ent1Dup.Count != 1 || len(ent1Dup.Sources) != 1 {
		t.Errorf("expected count 1 and 1 source after duplicate source run, got count %d, sources %v", ent1Dup.Count, ent1Dup.Sources)
	}

	// 3. Create or Update Entity (Third Ingestion - new source, should append source and count becomes 2)
	ent1New, err := CreateOrUpdateEntity(ctx, notebookID, sourceID2, "artificial intelligence")
	if err != nil {
		t.Fatalf("CreateOrUpdateEntity new failed: %v", err)
	}
	if ent1New.Count != 2 || len(ent1New.Sources) != 2 {
		t.Errorf("expected count 2 and 2 sources, got count %d, sources %v", ent1New.Count, ent1New.Sources)
	}

	// 4. Relate Entities (First Co-occurrence)
	ent2, err := CreateOrUpdateEntity(ctx, notebookID, sourceID1, "machine learning")
	if err != nil {
		t.Fatalf("failed to create second entity: %v", err)
	}

	err = RelateEntities(ctx, notebookID, sourceID1, ent1, ent2)
	if err != nil {
		t.Fatalf("RelateEntities failed: %v", err)
	}

	// Query relation edge
	type edgeResult struct {
		Weight  int                `json:"weight"`
		Sources []*models.RecordID `json:"sources"`
	}
	edges, err := db.RepoQuery[[]edgeResult](ctx, "SELECT weight, sources FROM co_occurs;", nil)
	if err != nil || len(*edges) == 0 {
		t.Fatalf("failed to query co_occurs relationship: %v", err)
	}

	firstEdge := (*edges)[0]
	if firstEdge.Weight != 1 {
		t.Errorf("expected relation weight 1, got %d", firstEdge.Weight)
	}
	if len(firstEdge.Sources) != 1 || firstEdge.Sources[0].ID != sourceID1 {
		t.Errorf("expected relation sources to contain %q, got: %v", sourceID1, firstEdge.Sources)
	}
}

func TestClearSourceGraphLineage(t *testing.T) {
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

	notebookID := "nb_test_lin"
	sourceID1 := "src_test_1"
	sourceID2 := "src_test_2"

	// Create entity with source 1 and 2
	_, _ = CreateOrUpdateEntity(ctx, notebookID, sourceID1, "artificial intelligence")
	ent, err := CreateOrUpdateEntity(ctx, notebookID, sourceID2, "artificial intelligence")
	if err != nil {
		t.Fatalf("failed to create entity: %v", err)
	}

	if ent.Count != 2 || len(ent.Sources) != 2 {
		t.Fatalf("expected count 2, got %d and sources %v", ent.Count, ent.Sources)
	}

	// 1. Clear lineage for source 1 (should decrement count to 1 and remove source 1)
	err = ClearSourceGraphLineage(ctx, notebookID, sourceID1)
	if err != nil {
		t.Fatalf("ClearSourceGraphLineage failed for source 1: %v", err)
	}

	res, err := db.RepoQuery[[]RAGEntity](ctx, "SELECT * FROM RAGEntity WHERE notebook = $nb AND name = 'artificial intelligence';", map[string]any{"nb": db.EnsureRecordID("notebook", notebookID)})
	if err != nil || len(*res) == 0 {
		t.Fatalf("entity should not be deleted yet, but query failed or returned empty: %v", err)
	}

	updatedEnt := (*res)[0]
	if updatedEnt.Count != 1 {
		t.Errorf("expected count 1, got %d", updatedEnt.Count)
	}
	if len(updatedEnt.Sources) != 1 || updatedEnt.Sources[0].ID != sourceID2 {
		t.Errorf("expected remaining source to be %s, got: %v", sourceID2, updatedEnt.Sources)
	}

	// 2. Clear lineage for source 2 (should completely delete the entity since no sources remain)
	err = ClearSourceGraphLineage(ctx, notebookID, sourceID2)
	if err != nil {
		t.Fatalf("ClearSourceGraphLineage failed for source 2: %v", err)
	}

	resDeleted, err := db.RepoQuery[[]RAGEntity](ctx, "SELECT * FROM RAGEntity WHERE notebook = $nb AND name = 'artificial intelligence';", map[string]any{"nb": db.EnsureRecordID("notebook", notebookID)})
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if len(*resDeleted) != 0 {
		t.Errorf("expected entity to be deleted completely, but found: %v", *resDeleted)
	}
}

