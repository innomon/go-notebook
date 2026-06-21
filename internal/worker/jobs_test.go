package worker

import (
	"archive/zip"
	"context"
	"go-notebook/internal/db"
	"go-notebook/internal/domain"
	"io"
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
