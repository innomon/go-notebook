package router

import (
	"bytes"
	"context"
	"encoding/json"
	"go-notebook/internal/db"
	"go-notebook/internal/domain"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestBasesRouter_Integration(t *testing.T) {
	// Configure test DB settings
	os.Setenv("SURREAL_URL", "ws://localhost:8000/rpc")
	os.Setenv("SURREAL_USER", "root")
	os.Setenv("SURREAL_PASSWORD", "root")
	os.Setenv("SURREAL_NAMESPACE", "open_notebook_test")
	os.Setenv("SURREAL_DATABASE", "open_notebook_test")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := db.Init(ctx); err != nil {
		t.Skip("SurrealDB offline")
	}
	defer db.Close(ctx)

	// Clean test database and run migrations
	_, _ = db.RepoQuery[any](ctx, "REMOVE DATABASE open_notebook_test;", nil)
	_, _ = db.RepoQuery[any](ctx, "DEFINE DATABASE open_notebook_test;", nil)
	_ = db.RunMigrationUp(ctx)

	// Create a test notebook and note
	nb, err := domain.CreateNotebook(ctx, "Bases Vault", "Testing Obsidian Bases REST Endpoints")
	if err != nil {
		t.Fatalf("failed to create notebook: %v", err)
	}
	nbID := nb.ID.String()

	// Create Note with frontmatter content
	noteContent := `---
status: active
priority: 5
created_at: 2026-07-01
---
# Test note body
Testing bases REST API.`
	_, _, err = domain.CreateNote(ctx, "REST Note", noteContent, "human", nbID)
	if err != nil {
		t.Fatalf("failed to create note: %v", err)
	}

	// Setup ServerMux for routing tests
	mux := http.NewServeMux()
	RegisterBasesRoutes(mux)

	// 1. Test GET /api/bases/plugins
	req := httptest.NewRequest("GET", "/api/bases/plugins", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200 OK, got %d", resp.StatusCode)
	}

	var plugins []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&plugins); err != nil {
		t.Fatalf("failed to parse plugins response: %v", err)
	}
	resp.Body.Close()

	// 2. Test POST /api/bases/plugins/permissions
	permPayload := map[string]any{
		"name":             "calculate_days_since",
		"read_other_notes": true,
		"access_env":       false,
	}
	permBytes, _ := json.Marshal(permPayload)
	reqPerm := httptest.NewRequest("POST", "/api/bases/plugins/permissions", bytes.NewReader(permBytes))
	wPerm := httptest.NewRecorder()
	mux.ServeHTTP(wPerm, reqPerm)

	respPerm := wPerm.Result()
	if respPerm.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200 OK, got %d", respPerm.StatusCode)
	}
	respPerm.Body.Close()

	// 3. Test POST /api/bases/evaluate
	evalPayload := map[string]any{
		"notebook_id": nbID,
		"config": map[string]any{
			"view_type": "table",
			"filters": []map[string]any{
				{"property": "status", "operator": "eq", "value": "active"},
			},
			"formulas": map[string]any{
				"age": "calculate_days_since",
			},
		},
	}
	evalBytes, _ := json.Marshal(evalPayload)
	reqEval := httptest.NewRequest("POST", "/api/bases/evaluate", bytes.NewReader(evalBytes))
	wEval := httptest.NewRecorder()
	mux.ServeHTTP(wEval, reqEval)

	respEval := wEval.Result()
	if respEval.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(respEval.Body)
		respEval.Body.Close()
		t.Fatalf("expected status 200 OK, got %d, body: %s", respEval.StatusCode, string(bodyBytes))
	}

	var evalOut map[string]any
	if err := json.NewDecoder(respEval.Body).Decode(&evalOut); err != nil {
		t.Fatalf("failed to parse evaluate response: %v", err)
	}
	respEval.Body.Close()

	if evalOut["type"] != "table" {
		t.Errorf("expected A2UI type 'table', got '%v'", evalOut["type"])
	}

	// 4. Test POST /api/bases/plugins/permissions with malformed JSON
	reqBadPerm := httptest.NewRequest("POST", "/api/bases/plugins/permissions", bytes.NewReader([]byte("invalid-json")))
	wBadPerm := httptest.NewRecorder()
	mux.ServeHTTP(wBadPerm, reqBadPerm)
	if wBadPerm.Result().StatusCode != http.StatusBadRequest {
		t.Errorf("expected status 400 Bad Request for bad JSON in permissions, got %d", wBadPerm.Result().StatusCode)
	}

	// 5. Test POST /api/bases/plugins/permissions with empty plugin name
	emptyNamePerm := map[string]any{"name": "", "read_other_notes": true}
	emptyNameBytes, _ := json.Marshal(emptyNamePerm)
	reqEmptyName := httptest.NewRequest("POST", "/api/bases/plugins/permissions", bytes.NewReader(emptyNameBytes))
	wEmptyName := httptest.NewRecorder()
	mux.ServeHTTP(wEmptyName, reqEmptyName)
	if wEmptyName.Result().StatusCode != http.StatusBadRequest {
		t.Errorf("expected status 400 Bad Request for empty plugin name, got %d", wEmptyName.Result().StatusCode)
	}

	// 6. Test POST /api/bases/evaluate with malformed JSON
	reqBadEval := httptest.NewRequest("POST", "/api/bases/evaluate", bytes.NewReader([]byte("invalid-json")))
	wBadEval := httptest.NewRecorder()
	mux.ServeHTTP(wBadEval, reqBadEval)
	if wBadEval.Result().StatusCode != http.StatusBadRequest {
		t.Errorf("expected status 400 Bad Request for bad JSON in evaluate, got %d", wBadEval.Result().StatusCode)
	}

	// 7. Test POST /api/bases/evaluate with missing config
	missingConfig := map[string]any{"notebook_id": nbID}
	missingBytes, _ := json.Marshal(missingConfig)
	reqMissingConfig := httptest.NewRequest("POST", "/api/bases/evaluate", bytes.NewReader(missingBytes))
	wMissingConfig := httptest.NewRecorder()
	mux.ServeHTTP(wMissingConfig, reqMissingConfig)
	if wMissingConfig.Result().StatusCode != http.StatusBadRequest {
		t.Errorf("expected status 400 Bad Request for missing config, got %d", wMissingConfig.Result().StatusCode)
	}

	// 8. Test POST /api/bases/evaluate without notebook_id (All-notes fallback)
	allNotesEval := map[string]any{
		"config": map[string]any{
			"view_type": "table",
			"filters": []map[string]any{
				{"property": "status", "operator": "eq", "value": "active"},
			},
			"formulas": map[string]any{
				"age": "calculate_days_since",
			},
		},
	}
	allNotesBytes, _ := json.Marshal(allNotesEval)
	reqAllNotes := httptest.NewRequest("POST", "/api/bases/evaluate", bytes.NewReader(allNotesBytes))
	wAllNotes := httptest.NewRecorder()
	mux.ServeHTTP(wAllNotes, reqAllNotes)

	respAllNotes := wAllNotes.Result()
	if respAllNotes.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(respAllNotes.Body)
		t.Fatalf("expected status 200 OK for all-notes evaluate, got %d, body: %s", respAllNotes.StatusCode, string(bodyBytes))
	}
	respAllNotes.Body.Close()
}
