package router

import (
	"encoding/json"
	"fmt"
	"go-notebook/pkg/okf"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

var globalWatcherManager = okf.NewWatcherManager()

// RegisterOKFRoutes registers the REST endpoints for OKF integration.
func RegisterOKFRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/okf/validate", handleOKFValidate)
	mux.HandleFunc("GET /api/okf/graph", handleOKFGraph)
}

func handleOKFValidate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	type RequestPayload struct {
		Path string `json:"path"`
	}
	var payload RequestPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || payload.Path == "" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "missing or invalid path in payload"})
		return
	}

	cleanPath := filepath.Clean(payload.Path)
	info, err := os.Stat(cleanPath)
	if err != nil || !info.IsDir() {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "target path is not a valid directory"})
		return
	}

	type ValidationErrorItem struct {
		File  string `json:"file"`
		Error string `json:"error"`
	}

	var errorsList []ValidationErrorItem

	err = filepath.WalkDir(cleanPath, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			name := d.Name()
			if name == ".git" || name == "node_modules" || name == ".obsidian" || name == ".gemini" {
				return filepath.SkipDir
			}
			return nil
		}

		if strings.HasSuffix(strings.ToLower(d.Name()), ".md") {
			relPath, relErr := filepath.Rel(cleanPath, path)
			if relErr != nil {
				return nil
			}
			relPath = filepath.ToSlash(relPath)

			content, readErr := os.ReadFile(path)
			if readErr != nil {
				errorsList = append(errorsList, ValidationErrorItem{
					File:  relPath,
					Error: fmt.Sprintf("failed to read file: %v", readErr),
				})
				return nil
			}

			_, _, parseErr := okf.ParseDocument(strings.NewReader(string(content)))
			if parseErr != nil {
				errorsList = append(errorsList, ValidationErrorItem{
					File:  relPath,
					Error: parseErr.Error(),
				})
			}
		}
		return nil
	})

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	response := map[string]any{
		"valid":  len(errorsList) == 0,
		"errors": errorsList,
	}

	_ = json.NewEncoder(w).Encode(response)
}

func handleOKFGraph(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	path := r.URL.Query().Get("path")
	if path == "" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "missing path query parameter"})
		return
	}

	cleanPath := filepath.Clean(path)
	info, err := os.Stat(cleanPath)
	if err != nil || !info.IsDir() {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "target path is not a valid directory"})
		return
	}

	indexer := okf.NewWorkspaceIndexer(cleanPath)

	// Run index sync
	if err := indexer.Index(r.Context()); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": fmt.Sprintf("indexing failed: %v", err)})
		return
	}

	// Register watcher and touch timestamp to prevent timeout reaping
	_ = globalWatcherManager.Watch(r.Context(), cleanPath, indexer)
	globalWatcherManager.Touch(cleanPath)

	graph, err := indexer.GetGraph(r.Context())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": fmt.Sprintf("failed to retrieve graph: %v", err)})
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]any{
		"nodes": graph,
	})
}
