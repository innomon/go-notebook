package worker

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"go-notebook/internal/db"
	"go-notebook/internal/domain"
	"go-notebook/internal/extractor"
	"go-notebook/internal/utils"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func createTestDocx(t *testing.T) string {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.docx")
	f, err := os.Create(filePath)
	if err != nil {
		t.Fatalf("failed to create docx: %v", err)
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	defer zw.Close()

	w, _ := zw.Create("word/document.xml")
	_, _ = io.WriteString(w, `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p><w:r><w:t>Hello Worker from Docx!</w:t></w:r></w:p>
  </w:body>
</w:document>`)
	return filePath
}

func createTestXlsx(t *testing.T) string {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.xlsx")
	f, err := os.Create(filePath)
	if err != nil {
		t.Fatalf("failed to create xlsx: %v", err)
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	defer zw.Close()

	w1, _ := zw.Create("xl/workbook.xml")
	_, _ = io.WriteString(w1, `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <sheets>
    <sheet name="Sheet1" sheetId="1" r:id="rId1"/>
  </sheets>
</workbook>`)

	w2, _ := zw.Create("xl/worksheets/sheet1.xml")
	_, _ = io.WriteString(w2, `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <sheetData>
    <row r="1">
      <c r="A1"><v>9876.54</v></c>
    </row>
  </sheetData>
</worksheet>`)
	return filePath
}

func TestJobRoutingForDocxAndXlsx(t *testing.T) {
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

	// Setup database
	_, _ = db.RepoQuery[any](ctx, "REMOVE DATABASE open_notebook_test;", nil)
	_, _ = db.RepoQuery[any](ctx, "DEFINE DATABASE open_notebook_test;", nil)
	_ = db.RunMigrationUp(ctx)

	// Create test files
	docxPath := createTestDocx(t)
	xlsxPath := createTestXlsx(t)

	// Test 1: DOCX job routing
	srcDocx, err := db.RepoCreate[domain.Source](ctx, "source", map[string]any{
		"title":     "Docx Title",
		"full_text": "Processing...",
	})
	if err != nil {
		t.Fatalf("failed to create source: %v", err)
	}

	jobDocx := &domain.CommandJob{
		Command: "process_source",
		Input: map[string]any{
			"source_id": srcDocx.ID.String(),
			"content_state": map[string]any{
				"file_path": docxPath,
			},
		},
	}

	_, err = handleProcessSource(ctx, jobDocx)
	if err != nil {
		t.Fatalf("handleProcessSource failed for docx: %v", err)
	}

	// Fetch source and verify text is extracted
	updatedDocx, err := domain.GetSource(ctx, srcDocx.ID.String())
	if err != nil {
		t.Fatalf("failed to get updated source: %v", err)
	}
	if !strings.Contains(updatedDocx.FullText, "Hello Worker from Docx!") {
		t.Errorf("expected docx text to contain parsed string, got: %s", updatedDocx.FullText)
	}

	// Test 2: XLSX job routing
	srcXlsx, err := db.RepoCreate[domain.Source](ctx, "source", map[string]any{
		"title":     "Xlsx Title",
		"full_text": "Processing...",
	})
	if err != nil {
		t.Fatalf("failed to create source: %v", err)
	}

	jobXlsx := &domain.CommandJob{
		Command: "process_source",
		Input: map[string]any{
			"source_id": srcXlsx.ID.String(),
			"content_state": map[string]any{
				"file_path": xlsxPath,
			},
		},
	}

	_, err = handleProcessSource(ctx, jobXlsx)
	if err != nil {
		t.Fatalf("handleProcessSource failed for xlsx: %v", err)
	}

	updatedXlsx, err := domain.GetSource(ctx, srcXlsx.ID.String())
	if err != nil {
		t.Fatalf("failed to get updated source: %v", err)
	}
	if !strings.Contains(updatedXlsx.FullText, "9876.54") {
		t.Errorf("expected xlsx text to contain parsed string, got: %s", updatedXlsx.FullText)
	}
}

func TestJobRoutingForImages(t *testing.T) {
	os.Setenv("SURREAL_URL", "ws://localhost:8000/rpc")
	os.Setenv("SURREAL_USER", "root")
	os.Setenv("SURREAL_PASSWORD", "root")
	os.Setenv("SURREAL_NAMESPACE", "open_notebook_test")
	os.Setenv("SURREAL_DATABASE", "open_notebook_test")

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	if err := db.Init(ctx); err != nil {
		t.Skip("SurrealDB offline")
	}
	defer db.Close(ctx)

	// Clean/Setup database
	_, _ = db.RepoQuery[any](ctx, "REMOVE DATABASE open_notebook_test;", nil)
	_, _ = db.RepoQuery[any](ctx, "DEFINE DATABASE open_notebook_test;", nil)
	_ = db.RunMigrationUp(ctx)

	// Mock HTTP Server for LLM Vision fallback
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"content": "# Whiteboard Vision Summary\n- Diagram: Flowchart of ingestion pipeline\n- Text: extracted from whiteboard image notes",
					},
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Mock Tesseract being missing to trigger LLM fallback path
	restoreTesseract := extractor.SetTesseractMock(
		func(binary string) (string, error) {
			return "", errors.New("tesseract not found")
		},
		func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return nil, errors.New("command failed")
		},
	)
	defer restoreTesseract()

	// Create test database records for credentials and models
	encKey, _ := utils.EncryptValue("test-key")
	cred, err := db.RepoCreate[domain.Credential](ctx, "credential", map[string]any{
		"name":        "Test API Key",
		"provider":    "openai",
		"api_key":     encKey,
		"base_url":    server.URL,
		"api_version": "v1",
		"modalities":  []string{"language", "vision"},
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

	// Update default models configuration
	err = domain.UpdateDefaultModels(ctx, &domain.DefaultModels{
		DefaultChatModel:           model.ID.String(),
		DefaultTransformationModel: model.ID.String(),
	})
	if err != nil {
		t.Fatalf("failed to update default models config: %v", err)
	}

	// Create a dummy png file path
	tmpDir := t.TempDir()
	pngPath := filepath.Join(tmpDir, "whiteboard_notes.png")
	if err := os.WriteFile(pngPath, []byte("fake png binary data"), 0644); err != nil {
		t.Fatalf("failed to create fake png: %v", err)
	}

	// Create the source record
	src, err := db.RepoCreate[domain.Source](ctx, "source", map[string]any{
		"title":     "Processing...",
		"full_text": "Processing...",
	})
	if err != nil {
		t.Fatalf("failed to create source record: %v", err)
	}

	job := &domain.CommandJob{
		ID:      db.EnsureRecordID("command", "test_job"),
		Command: "process_source",
		Input: map[string]any{
			"source_id": src.ID.String(),
			"content_state": map[string]any{
				"file_path": pngPath,
			},
		},
	}

	// Execute process_source
	_, err = handleProcessSource(ctx, job)
	if err != nil {
		t.Fatalf("handleProcessSource failed for image: %v", err)
	}

	// Verify full_text was successfully updated with OCR output
	updatedSrc, err := domain.GetSource(ctx, src.ID.String())
	if err != nil {
		t.Fatalf("failed to get updated source: %v", err)
	}

	expectedText := "Whiteboard Vision Summary"
	if !strings.Contains(updatedSrc.FullText, expectedText) {
		t.Errorf("expected source full_text to contain %q, but got: %s", expectedText, updatedSrc.FullText)
	}
}
