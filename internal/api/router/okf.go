package router

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go-notebook/internal/ai"
	"go-notebook/pkg/okf"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

var globalWatcherManager = okf.NewWatcherManager()

// RegisterOKFRoutes registers the REST endpoints for OKF integration.
func RegisterOKFRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/okf/validate", handleOKFValidate)
	mux.HandleFunc("GET /api/okf/graph", handleOKFGraph)
	mux.HandleFunc("POST /api/okf/enrich", handleOKFEnrich)
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

func handleOKFEnrich(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	type EnrichRequest struct {
		Content string `json:"content"`
		Path    string `json:"path"`
	}

	var req EnrichRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid payload: " + err.Error()})
		return
	}

	var content string
	var path string
	if req.Path != "" {
		path = filepath.Clean(req.Path)
		fileBytes, err := os.ReadFile(path)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "failed to read file: " + err.Error()})
			return
		}
		content = string(fileBytes)
	} else if req.Content != "" {
		content = req.Content
	} else {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "missing content or path in request"})
		return
	}

	// Parse document to validate format and extract components
	meta, bodyBytes, err := okf.ParseDocument(strings.NewReader(content))
	if err != nil {
		// If some mandatory fields are missing (like description which we are enriching!),
		// ParseDocument returns the partially parsed meta and ErrMissingFields.
		// We only reject if it's not a missing fields error or no frontmatter.
		if err != okf.ErrMissingFields {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "failed to parse document: " + err.Error()})
			return
		}
	}

	// Resolve model
	client, err := ai.GetClientForDefaultModel(r.Context(), "transformation")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "failed to resolve AI client: " + err.Error()})
		return
	}

	// Prepare LLM prompt
	systemPrompt := `You are an expert technical editor. Analyze the provided Markdown document and generate a concise description (1-2 sentences summarizing its intent and capabilities) and a list of relevant tags for categorization.
Your response MUST be a single, valid JSON object containing exactly two keys: "description" (a string) and "tags" (an array of strings).
Do NOT include any explanation, intro, outro, markdown formatting blocks, or backticks. Just return raw JSON.`

	userPrompt := fmt.Sprintf("Document Content:\n%s", content)

	llmResponse, err := client.GenerateText(r.Context(), systemPrompt, userPrompt)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "failed to generate suggestions: " + err.Error()})
		return
	}

	// Sanitize response and extract JSON
	cleanedJSON := cleanJSONResponse(llmResponse)
	
	type LLMResponse struct {
		Description string   `json:"description"`
		Tags        []string `json:"tags"`
	}

	var suggestion LLMResponse
	if err := json.Unmarshal([]byte(cleanedJSON), &suggestion); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": "failed to parse LLM response: " + err.Error(),
			"raw":   llmResponse,
		})
		return
	}

	// If file path was provided, save the updated note back to disk
	if path != "" {
		// Update meta fields
		meta.Description = suggestion.Description
		meta.Tags = suggestion.Tags

		// Serialize back to file format
		updatedBytes, err := serializeDocument(meta, bodyBytes)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "failed to serialize updated metadata: " + err.Error()})
			return
		}

		// Write to disk
		if err := os.WriteFile(path, updatedBytes, 0644); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "failed to write file to disk: " + err.Error()})
			return
		}
	}

	// Respond with suggestion
	_ = json.NewEncoder(w).Encode(suggestion)
}

func cleanJSONResponse(raw string) string {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "```") {
		// Find first newline
		idx := strings.Index(raw, "\n")
		if idx != -1 {
			raw = raw[idx:]
		}
		raw = strings.TrimSuffix(raw, "```")
		raw = strings.TrimSpace(raw)
	}
	// Also strip any custom ```json and ``` prefix / suffix
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	return strings.TrimSpace(raw)
}

func serializeDocument(meta *okf.Metadata, body []byte) ([]byte, error) {
	metaBytes, err := yaml.Marshal(meta)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	buf.WriteString("---\n")
	buf.Write(metaBytes)
	buf.WriteString("---\n")
	buf.Write(body)
	return buf.Bytes(), nil
}
