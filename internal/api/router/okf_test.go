package router

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"go-notebook/internal/db"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestOKFEndpoints(t *testing.T) {
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

	// Clean database and run migrations
	_, _ = db.RepoQuery[any](ctx, "REMOVE DATABASE open_notebook_test;", nil)
	_, _ = db.RepoQuery[any](ctx, "DEFINE DATABASE open_notebook_test;", nil)
	_ = db.RunMigrationUp(ctx)

	tmpDir := t.TempDir()

	// Create valid and invalid notes in workspace
	validFile := filepath.Join(tmpDir, "valid.md")
	validContent := "---\ntype: Code\ntitle: Valid Document\ndescription: A valid document specification.\n---\nBody text."
	if err := os.WriteFile(validFile, []byte(validContent), 0644); err != nil {
		t.Fatalf("failed to write valid note: %v", err)
	}

	invalidFile := filepath.Join(tmpDir, "invalid.md")
	invalidContent := "---\ntitle: Missing fields\n---\nBody text."
	if err := os.WriteFile(invalidFile, []byte(invalidContent), 0644); err != nil {
		t.Fatalf("failed to write invalid note: %v", err)
	}

	mux := http.NewServeMux()
	RegisterOKFRoutes(mux)

	// 1. Test POST /api/okf/validate
	valPayload := map[string]string{"path": tmpDir}
	payloadBytes, _ := json.Marshal(valPayload)
	reqVal := httptest.NewRequest("POST", "/api/okf/validate", bytes.NewBuffer(payloadBytes))
	reqVal.Header.Set("Content-Type", "application/json")
	wVal := httptest.NewRecorder()

	mux.ServeHTTP(wVal, reqVal)

	if wVal.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", wVal.Code)
	}

	var valResp map[string]any
	if err := json.Unmarshal(wVal.Body.Bytes(), &valResp); err != nil {
		t.Fatalf("failed to unmarshal validation response: %v", err)
	}

	validVal, ok := valResp["valid"].(bool)
	if !ok || validVal {
		t.Errorf("expected valid=false due to invalid.md, got %v", valResp["valid"])
	}

	errorsList, ok := valResp["errors"].([]any)
	if !ok || len(errorsList) == 0 {
		t.Error("expected non-empty errors array in validation output")
	}

	// 2. Test GET /api/okf/graph?path=...
	reqGraph := httptest.NewRequest("GET", fmt.Sprintf("/api/okf/graph?path=%s", tmpDir), nil)
	wGraph := httptest.NewRecorder()

	mux.ServeHTTP(wGraph, reqGraph)

	if wGraph.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", wGraph.Code)
	}

	var graphResp map[string]any
	if err := json.Unmarshal(wGraph.Body.Bytes(), &graphResp); err != nil {
		t.Fatalf("failed to unmarshal graph response: %v", err)
	}

	nodes, ok := graphResp["nodes"].([]any)
	if !ok {
		t.Error("expected nodes list in graph response")
	}

	// Note that invalid.md is omitted from graph, only valid.md should be present
	if len(nodes) != 1 {
		t.Errorf("expected exactly 1 node (valid.md), got %d: %v", len(nodes), nodes)
	}
}
