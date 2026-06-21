package router

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"go-notebook/internal/db"
	"go-notebook/internal/domain"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestNotebookExportImportEndpoints(t *testing.T) {
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

	// Create a test notebook to export
	nb, err := domain.CreateNotebook(ctx, "Endpoint Vault", "Test HTTP endpoints")
	if err != nil {
		t.Fatalf("failed to create notebook: %v", err)
	}
	nbID := nb.ID.String()

	// Add a note directly using domain
	_, _, err = domain.CreateNote(ctx, "Secrets", "Obsidian and Go pair programming is awesome.", "human", nbID)
	if err != nil {
		t.Fatalf("failed to create note: %v", err)
	}

	// Setup ServerMux for routing tests
	mux := http.NewServeMux()
	RegisterNotebookRoutes(mux)

	// 1. Test GET /api/notebooks/{id}/export
	req := httptest.NewRequest("GET", fmt.Sprintf("/api/notebooks/%s/export", nbID), nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected export status 200 OK, got %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType != "application/zip" {
		t.Errorf("expected Content-Type application/zip, got %q", contentType)
	}

	contentDisp := resp.Header.Get("Content-Disposition")
	if !bytes.Contains([]byte(contentDisp), []byte("attachment; filename=\"Endpoint Vault_export.zip\"")) {
		t.Errorf("expected Content-Disposition header with filename, got %q", contentDisp)
	}

	zipBytes, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatalf("failed to read zip bytes from response: %v", err)
	}
	zipSize := int64(len(zipBytes))

	// Verify zip contents contains notes/Secrets.md
	zr, err := zip.NewReader(bytes.NewReader(zipBytes), zipSize)
	if err != nil {
		t.Fatalf("invalid zip returned: %v", err)
	}
	foundNote := false
	for _, f := range zr.File {
		if f.Name == "notes/Secrets.md" {
			foundNote = true
			break
		}
	}
	if !foundNote {
		t.Errorf("notes/Secrets.md not found in exported zip")
	}

	// 2. Test POST /api/notebooks/import (Import as a NEW notebook)
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "Endpoint_Vault_export.zip")
	if err != nil {
		t.Fatalf("failed to create form file: %v", err)
	}
	part.Write(zipBytes)
	writer.Close()

	importReq := httptest.NewRequest("POST", "/api/notebooks/import", &body)
	importReq.Header.Set("Content-Type", writer.FormDataContentType())
	wImport := httptest.NewRecorder()
	mux.ServeHTTP(wImport, importReq)

	respImport := wImport.Result()
	if respImport.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(respImport.Body)
		t.Fatalf("expected import status 201 Created, got %d, body: %s", respImport.StatusCode, string(bodyBytes))
	}

	var nbResp domain.NotebookResponse
	err = json.NewDecoder(respImport.Body).Decode(&nbResp)
	respImport.Body.Close()
	if err != nil {
		t.Fatalf("failed to decode import response: %v", err)
	}

	if nbResp.Name != "Endpoint Vault (1)" {
		t.Errorf("expected imported notebook name Endpoint Vault (1), got %q", nbResp.Name)
	}

	// 3. Test POST /api/notebooks/{id}/import (Merge import)
	var bodyMerge bytes.Buffer
	writerMerge := multipart.NewWriter(&bodyMerge)
	partMerge, err := writerMerge.CreateFormFile("file", "Endpoint_Vault_export.zip")
	if err != nil {
		t.Fatalf("failed to create form file: %v", err)
	}
	partMerge.Write(zipBytes)
	writerMerge.Close()

	mergeReq := httptest.NewRequest("POST", fmt.Sprintf("/api/notebooks/%s/import", nbResp.ID), &bodyMerge)
	mergeReq.Header.Set("Content-Type", writerMerge.FormDataContentType())
	wMerge := httptest.NewRecorder()
	mux.ServeHTTP(wMerge, mergeReq)

	respMerge := wMerge.Result()
	if respMerge.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(respMerge.Body)
		t.Fatalf("expected merge status 200 OK, got %d, body: %s", respMerge.StatusCode, string(bodyBytes))
	}

	var successResp map[string]string
	_ = json.NewDecoder(respMerge.Body).Decode(&successResp)
	respMerge.Body.Close()
	if successResp["status"] != "success" {
		t.Errorf("expected status 'success' in merge response, got: %v", successResp)
	}
}
