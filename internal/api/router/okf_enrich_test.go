package router

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"go-notebook/internal/db"
	"go-notebook/internal/domain"
	"go-notebook/internal/utils"
	"go-notebook/pkg/okf"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestOKFEnrichEndpoint(t *testing.T) {
	// Configure test DB settings
	os.Setenv("SURREAL_URL", "ws://localhost:8000/rpc")
	os.Setenv("SURREAL_USER", "root")
	os.Setenv("SURREAL_PASSWORD", "root")
	os.Setenv("SURREAL_NAMESPACE", "open_notebook_test")
	os.Setenv("SURREAL_DATABASE", "open_notebook_test")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := db.Init(ctx); err != nil {
		t.Skip("SurrealDB offline")
	}
	defer db.Close(ctx)

	// Clean database and run migrations
	_, _ = db.RepoQuery[any](ctx, "REMOVE DATABASE open_notebook_test;", nil)
	_, _ = db.RepoQuery[any](ctx, "DEFINE DATABASE open_notebook_test;", nil)
	_ = db.RunMigrationUp(ctx)

	// Set up mock HTTP Server for LLM responses (supporting streaming chat completions)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		// SSE JSON chunk format matching OpenAI API
		content := `{"description": "A sample concept document.", "tags": ["sample", "test"]}`
		chunk := map[string]any{
			"choices": []map[string]any{
				{
					"delta": map[string]any{
						"content": content,
					},
				},
			},
		}
		chunkBytes, _ := json.Marshal(chunk)
		_, _ = fmt.Fprintf(w, "data: %s\n\n", string(chunkBytes))
		_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	// Create test credentials and model
	encKey, _ := utils.EncryptValue("test-key")
	cred, err := db.RepoCreate[domain.Credential](ctx, "credential", map[string]any{
		"name":        "Test API Key",
		"provider":    "openai",
		"api_key":     encKey,
		"base_url":    server.URL,
		"api_version": "v1",
		"modalities":  []string{"language"},
	})
	if err != nil {
		t.Fatalf("failed to create credential: %v", err)
	}

	model, err := db.RepoCreate[domain.Model](ctx, "model", map[string]any{
		"name":       "gpt-4o",
		"provider":   "openai",
		"type":       "language",
		"credential": cred.ID,
	})
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}

	// Update default models config to point to our mock model
	err = domain.UpdateDefaultModels(ctx, &domain.DefaultModels{
		DefaultChatModel:           model.ID.String(),
		DefaultTransformationModel: model.ID.String(),
	})
	if err != nil {
		t.Fatalf("failed to update default models config: %v", err)
	}

	mux := http.NewServeMux()
	RegisterOKFRoutes(mux)

	// 1. Test in-memory content enrichment
	t.Run("In-Memory Content Enrichment", func(t *testing.T) {
		payload := map[string]string{
			"content": "---\ntype: Concept\ntitle: My Test Document\ndescription: Old description\ntags:\n  - old-tag\n---\nHere is the body content of the test note.",
		}
		payloadBytes, _ := json.Marshal(payload)
		req := httptest.NewRequest("POST", "/api/okf/enrich", bytes.NewBuffer(payloadBytes))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d. Body: %s", w.Code, w.Body.String())
		}

		var resp map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}

		desc, _ := resp["description"].(string)
		if desc != "A sample concept document." {
			t.Errorf("expected description 'A sample concept document.', got '%s'", desc)
		}

		tags, ok := resp["tags"].([]any)
		if !ok || len(tags) != 2 || tags[0] != "sample" || tags[1] != "test" {
			t.Errorf("expected tags ['sample', 'test'], got %v", resp["tags"])
		}
	})

	// 2. Test file-based enrichment
	t.Run("File Path Enrichment", func(t *testing.T) {
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "note.md")
		initialContent := "---\ntype: Feature\ntitle: Disk File\ndescription: Before enrichment\ntags:\n  - old\n---\nMy file-based test markdown note."
		if err := os.WriteFile(filePath, []byte(initialContent), 0644); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}

		payload := map[string]string{
			"path": filePath,
		}
		payloadBytes, _ := json.Marshal(payload)
		req := httptest.NewRequest("POST", "/api/okf/enrich", bytes.NewBuffer(payloadBytes))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d. Body: %s", w.Code, w.Body.String())
		}

		var resp map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}

		desc, _ := resp["description"].(string)
		if desc != "A sample concept document." {
			t.Errorf("expected description 'A sample concept document.', got '%s'", desc)
		}

		// Verify on-disk file content was updated cleanly without altering body
		updatedContentBytes, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatalf("failed to read updated file: %v", err)
		}
		updatedContent := string(updatedContentBytes)

		if !strings.Contains(updatedContent, "My file-based test markdown note.") {
			t.Errorf("markdown body was altered during file-enrichment! Content: %s", updatedContent)
		}

		// Parse updated document to check new frontmatter structure
		meta, _, err := okf.ParseDocument(strings.NewReader(updatedContent))
		if err != nil {
			t.Fatalf("updated document is no longer valid OKF: %v", err)
		}

		if meta.Description != "A sample concept document." {
			t.Errorf("expected updated metadata description 'A sample concept document.', got '%s'", meta.Description)
		}

		if len(meta.Tags) != 2 || meta.Tags[0] != "sample" || meta.Tags[1] != "test" {
			t.Errorf("expected updated metadata tags ['sample', 'test'], got %v", meta.Tags)
		}

		if meta.Type != "Feature" || meta.Title != "Disk File" {
			t.Errorf("other frontmatter fields were lost! Metadata: %+v", meta)
		}
	})

	// 3. Test failure case with invalid JSON
	t.Run("Invalid Payload", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/okf/enrich", bytes.NewBuffer([]byte("{invalid-json}")))
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected bad request for malformed payload, got %d", w.Code)
		}
	})
}
