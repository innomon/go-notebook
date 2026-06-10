package router

import (
	"encoding/json"
	"errors"
	"fmt"
	"go-notebook/internal/db"
	"go-notebook/internal/domain"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// RegisterSourceRoutes binds source REST routes to the ServeMux
func RegisterSourceRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/sources", handleListSources)
	mux.HandleFunc("POST /api/sources", handleCreateSource)
	mux.HandleFunc("GET /api/sources/{id}", handleGetSource)
	mux.HandleFunc("PUT /api/sources/{id}", handleUpdateSource)
	mux.HandleFunc("DELETE /api/sources/{id}", handleDeleteSource)
	mux.HandleFunc("GET /api/sources/{id}/status", handleGetSourceStatus)
	mux.HandleFunc("POST /api/sources/{id}/retry", handleRetrySource)
	mux.HandleFunc("GET /api/sources/{id}/download", handleDownloadSourceFile)
	mux.HandleFunc("GET /api/sources/{source_id}/insights", handleGetSourceInsights)
	mux.HandleFunc("POST /api/sources/{source_id}/insights", handleCreateSourceInsight)
}

type AssetModel struct {
	FilePath string `json:"file_path,omitempty"`
	URL      string `json:"url,omitempty"`
}

type SourceListResponse struct {
	ID             string         `json:"id"`
	Title          string         `json:"title"`
	Topics         []string       `json:"topics"`
	Asset          *AssetModel    `json:"asset"`
	Embedded       bool           `json:"embedded"`
	EmbeddedChunks int            `json:"embedded_chunks"`
	InsightsCount  int            `json:"insights_count"`
	Created        time.Time      `json:"created"`
	Updated        time.Time      `json:"updated"`
	FileAvailable  *bool          `json:"file_available,omitempty"`
	CommandID      *string        `json:"command_id,omitempty"`
	Status         *string        `json:"status,omitempty"`
	ProcessingInfo map[string]any `json:"processing_info,omitempty"`
}

type SourceResponse struct {
	ID             string         `json:"id"`
	Title          string         `json:"title,omitempty"`
	Topics         []string       `json:"topics"`
	Asset          *AssetModel    `json:"asset,omitempty"`
	FullText       *string        `json:"full_text,omitempty"`
	Embedded       bool           `json:"embedded"`
	EmbeddedChunks int            `json:"embedded_chunks"`
	FileAvailable  *bool          `json:"file_available,omitempty"`
	Created        time.Time      `json:"created"`
	Updated        time.Time      `json:"updated"`
	CommandID      string         `json:"command_id,omitempty"`
	Status         string         `json:"status,omitempty"`
	ProcessingInfo map[string]any `json:"processing_info,omitempty"`
	Notebooks      []string       `json:"notebooks,omitempty"`
}

func handleListSources(w http.ResponseWriter, r *http.Request) {
	notebookID := r.URL.Query().Get("notebook_id")
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")
	sortBy := r.URL.Query().Get("sort_by")
	sortOrder := r.URL.Query().Get("sort_order")

	limit := 50
	if limitStr != "" {
		if val, err := strconv.Atoi(limitStr); err == nil {
			limit = val
		}
	}

	offset := 0
	if offsetStr != "" {
		if val, err := strconv.Atoi(offsetStr); err == nil {
			offset = val
		}
	}

	if sortBy == "" {
		sortBy = "updated"
	}
	if sortOrder == "" {
		sortOrder = "desc"
	}

	results, err := domain.ListSources(r.Context(), notebookID, limit, offset, sortBy, sortOrder)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to fetch sources: "+err.Error())
		return
	}

	response := make([]SourceListResponse, len(results))
	for i, row := range results {
		commandID, status, procInfo := parseCommandField(row.Command)

		var asset *AssetModel
		var fileAvailable *bool
		if row.Asset != nil {
			asset = &AssetModel{
				FilePath: row.Asset.FilePath,
				URL:      row.Asset.URL,
			}
			avail := row.Asset.FilePath != ""
			if avail {
				if _, err := os.Stat(row.Asset.FilePath); err == nil {
					avail = true
				} else {
					avail = false
				}
			}
			fileAvailable = &avail
		}

		response[i] = SourceListResponse{
			ID:             row.ID.String(),
			Title:          row.Title,
			Topics:         row.Topics,
			Asset:          asset,
			Embedded:       row.Embedded,
			EmbeddedChunks: 0,
			InsightsCount:  row.InsightsCount,
			Created:        row.Created,
			Updated:        row.Updated,
			FileAvailable:  fileAvailable,
			CommandID:      commandID,
			Status:         status,
			ProcessingInfo: procInfo,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func handleCreateSource(w http.ResponseWriter, r *http.Request) {
	// Support both application/json and multipart/form-data
	contentType := r.Header.Get("Content-Type")

	var sourceType string
	var notebookIDs []string
	var transformations []string
	var title string
	var url string
	var content string
	var embed bool
	var deleteSource bool
	var asyncProcessing bool
	var filePath string

	// Handle multipart/form-data
	if strings.Contains(contentType, "multipart/form-data") {
		err := r.ParseMultipartForm(32 << 20) // 32MB max in memory
		if err != nil {
			respondError(w, http.StatusBadRequest, "Failed to parse multipart form: "+err.Error())
			return
		}

		sourceType = r.FormValue("type")
		title = r.FormValue("title")
		url = r.FormValue("url")
		content = r.FormValue("content")
		embed = r.FormValue("embed") == "true"
		deleteSource = r.FormValue("delete_source") == "true"
		asyncProcessing = r.FormValue("async_processing") == "true"

		// Parse notebooks JSON string or fall back to notebook_id
		if notebooksJSON := r.FormValue("notebooks"); notebooksJSON != "" {
			var nbs []string
			if err := json.Unmarshal([]byte(notebooksJSON), &nbs); err == nil {
				notebookIDs = nbs
			} else {
				notebookIDs = []string{notebooksJSON}
			}
		}
		if notebookID := r.FormValue("notebook_id"); notebookID != "" {
			found := false
			for _, id := range notebookIDs {
				if id == notebookID {
					found = true
					break
				}
			}
			if !found {
				notebookIDs = append(notebookIDs, notebookID)
			}
		}

		// Parse transformations JSON string
		if transformationsJSON := r.FormValue("transformations"); transformationsJSON != "" {
			_ = json.Unmarshal([]byte(transformationsJSON), &transformations)
		}

		// Handle file upload
		file, header, err := r.FormFile("file")
		if err == nil {
			defer file.Close()

			uploadsDir := os.Getenv("UPLOADS_FOLDER")
			if uploadsDir == "" {
				uploadsDir = "uploads"
			}
			_ = os.MkdirAll(uploadsDir, 0755)

			uniquePath, err := generateUniqueFilename(header.Filename, uploadsDir)
			if err != nil {
				respondError(w, http.StatusBadRequest, "Invalid uploaded filename: "+err.Error())
				return
			}

			out, err := os.Create(uniquePath)
			if err != nil {
				respondError(w, http.StatusInternalServerError, "Failed to save file: "+err.Error())
				return
			}
			defer out.Close()

			_, err = io.Copy(out, file)
			if err != nil {
				respondError(w, http.StatusInternalServerError, "Failed to write file contents: "+err.Error())
				return
			}

			filePath = uniquePath
		}
	} else {
		// Fallback to JSON payload
		var payload struct {
			Type            string   `json:"type"`
			Notebooks       []string `json:"notebooks"`
			NotebookID      string   `json:"notebook_id"`
			Title           string   `json:"title"`
			URL             string   `json:"url"`
			Content         string   `json:"content"`
			Transformations []string `json:"transformations"`
			Embed           bool     `json:"embed"`
			DeleteSource    bool     `json:"delete_source"`
			AsyncProcessing bool     `json:"async_processing"`
			FilePath        string   `json:"file_path"`
		}

		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			respondError(w, http.StatusBadRequest, "Invalid JSON payload")
			return
		}

		sourceType = payload.Type
		title = payload.Title
		url = payload.URL
		content = payload.Content
		embed = payload.Embed
		deleteSource = payload.DeleteSource
		asyncProcessing = payload.AsyncProcessing
		filePath = payload.FilePath
		notebookIDs = payload.Notebooks
		if payload.NotebookID != "" {
			notebookIDs = append(notebookIDs, payload.NotebookID)
		}
		transformations = payload.Transformations
	}

	if sourceType == "" {
		respondError(w, http.StatusBadRequest, "source type is required")
		return
	}

	var asset *domain.Asset
	if sourceType == "link" {
		if url == "" {
			respondError(w, http.StatusBadRequest, "url is required for link sources")
			return
		}
		asset = &domain.Asset{URL: url}
	} else if sourceType == "upload" {
		if filePath == "" {
			respondError(w, http.StatusBadRequest, "file upload or file_path is required for upload sources")
			return
		}
		asset = &domain.Asset{FilePath: filePath}
	}

	source, commandID, err := domain.CreateSource(
		r.Context(),
		title,
		sourceType,
		asset,
		content,
		notebookIDs,
		transformations,
		embed,
		deleteSource,
		asyncProcessing,
	)

	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to create source: "+err.Error())
		return
	}

	// Translate to SourceResponse
	var assetModel *AssetModel
	if source.Asset != nil {
		assetModel = &AssetModel{
			FilePath: source.Asset.FilePath,
			URL:      source.Asset.URL,
		}
	}

	status := "new"
	procInfo := map[string]any{"async": asyncProcessing, "queued": true}

	response := SourceResponse{
		ID:             source.ID.String(),
		Title:          source.Title,
		Topics:         source.Topics,
		Asset:          assetModel,
		Embedded:       false,
		EmbeddedChunks: 0,
		Created:        source.Created,
		Updated:        source.Updated,
		CommandID:      commandID,
		Status:         status,
		ProcessingInfo: procInfo,
		Notebooks:      notebookIDs,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(response)
}

func handleGetSource(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		respondError(w, http.StatusBadRequest, "Source ID is required")
		return
	}

	ctx := r.Context()
	source, err := domain.GetSource(ctx, id)
	if err != nil {
		respondError(w, http.StatusNotFound, "Source not found")
		return
	}

	// Fetch chunks count
	chunksCount, _ := domain.GetEmbeddedChunksCount(ctx, id)

	var assetModel *AssetModel
	var fileAvailable *bool
	if source.Asset != nil {
		assetModel = &AssetModel{
			FilePath: source.Asset.FilePath,
			URL:      source.Asset.URL,
		}
		avail := source.Asset.FilePath != ""
		if avail {
			if _, err := os.Stat(source.Asset.FilePath); err == nil {
				avail = true
			} else {
				avail = false
			}
		}
		fileAvailable = &avail
	}

	// Fetch command info if set
	var commandID string
	var status string
	var procInfo map[string]any

	if source.Command != nil {
		commandID = source.Command.String()
		job, err := domain.GetCommandJob(ctx, commandID)
		if err == nil && job != nil {
			status = job.Status
			procInfo = map[string]any{}
			if job.ErrorMessage != nil {
				procInfo["error"] = *job.ErrorMessage
			}
			if job.Result != nil {
				if execMeta, ok := job.Result["execution_metadata"]; ok {
					if execMetaMap, ok := execMeta.(map[string]any); ok {
						procInfo["started_at"] = execMetaMap["started_at"]
						procInfo["completed_at"] = execMetaMap["completed_at"]
					}
				}
			}
		} else {
			status = "unknown"
		}
	}

	response := SourceResponse{
		ID:             source.ID.String(),
		Title:          source.Title,
		Topics:         source.Topics,
		Asset:          assetModel,
		FullText:       &source.FullText,
		Embedded:       chunksCount > 0,
		EmbeddedChunks: chunksCount,
		FileAvailable:  fileAvailable,
		Created:        source.Created,
		Updated:        source.Updated,
		CommandID:      commandID,
		Status:         status,
		ProcessingInfo: procInfo,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func handleUpdateSource(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		respondError(w, http.StatusBadRequest, "Source ID is required")
		return
	}

	var payload struct {
		Title  *string  `json:"title"`
		Topics []string `json:"topics"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	source, err := domain.UpdateSource(r.Context(), id, payload.Title, payload.Topics)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to update source: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(source)
}

func handleDeleteSource(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		respondError(w, http.StatusBadRequest, "Source ID is required")
		return
	}

	// Delete file from disk if auto_delete_files setting says so
	source, err := domain.GetSource(r.Context(), id)
	if err == nil && source != nil && source.Asset != nil && source.Asset.FilePath != "" {
		// Fetch settings to check if auto-delete is enabled
		settings, err := domain.GetContentSettings(r.Context())
		if err == nil && settings != nil && settings.AutoDeleteFiles == "yes" {
			_ = os.Remove(source.Asset.FilePath)
		}
	}

	if err := domain.DeleteSource(r.Context(), id); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to delete source: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"message": "Source deleted successfully",
	})
}

func handleGetSourceStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		respondError(w, http.StatusBadRequest, "Source ID is required")
		return
	}

	ctx := r.Context()
	source, err := domain.GetSource(ctx, id)
	if err != nil {
		respondError(w, http.StatusNotFound, "Source not found")
		return
	}

	status := "unknown"
	message := "No active command associated with this source"
	var procInfo map[string]any
	var commandID string

	if source.Command != nil {
		commandID = source.Command.String()
		job, err := domain.GetCommandJob(ctx, commandID)
		if err == nil && job != nil {
			status = job.Status
			message = fmt.Sprintf("Source processing status is: %s", job.Status)
			procInfo = map[string]any{}
			if job.ErrorMessage != nil {
				procInfo["error"] = *job.ErrorMessage
			}
			if job.Result != nil {
				procInfo["result"] = job.Result
				if execMeta, ok := job.Result["execution_metadata"]; ok {
					if execMetaMap, ok := execMeta.(map[string]any); ok {
						procInfo["started_at"] = execMetaMap["started_at"]
						procInfo["completed_at"] = execMetaMap["completed_at"]
					}
				}
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":          status,
		"message":         message,
		"processing_info": procInfo,
		"command_id":      commandID,
	})
}

func handleRetrySource(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		respondError(w, http.StatusBadRequest, "Source ID is required")
		return
	}

	commandID, err := domain.SubmitRetryCommand(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to submit retry command: "+err.Error())
		return
	}

	// Fetch updated source
	source, err := domain.GetSource(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to retrieve updated source: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id":         source.ID.String(),
		"title":      source.Title,
		"command_id": commandID,
		"status":     "pending",
	})
}

func handleDownloadSourceFile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		respondError(w, http.StatusBadRequest, "Source ID is required")
		return
	}

	source, err := domain.GetSource(r.Context(), id)
	if err != nil || source.Asset == nil || source.Asset.FilePath == "" {
		respondError(w, http.StatusNotFound, "Source file not found")
		return
	}

	http.ServeFile(w, r, source.Asset.FilePath)
}

func handleGetSourceInsights(w http.ResponseWriter, r *http.Request) {
	sourceID := r.PathValue("source_id")
	if sourceID == "" {
		respondError(w, http.StatusBadRequest, "Source ID is required")
		return
	}

	insights, err := domain.GetSourceInsights(r.Context(), sourceID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to fetch insights: "+err.Error())
		return
	}

	// Format response matching python SourceInsightResponse
	type InsightResponse struct {
		ID          string    `json:"id"`
		SourceID    string    `json:"source_id"`
		InsightType string    `json:"insight_type"`
		Content     string    `json:"content"`
		Created     time.Time `json:"created"`
		Updated     time.Time `json:"updated"`
	}

	response := make([]InsightResponse, len(insights))
	for i, ins := range insights {
		response[i] = InsightResponse{
			ID:          ins.ID.String(),
			SourceID:    ins.Source.String(),
			InsightType: ins.InsightType,
			Content:     ins.Content,
			Created:     ins.Created,
			Updated:     ins.Updated,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func handleCreateSourceInsight(w http.ResponseWriter, r *http.Request) {
	sourceID := r.PathValue("source_id")
	if sourceID == "" {
		respondError(w, http.StatusBadRequest, "Source ID is required")
		return
	}

	var payload struct {
		TransformationID string `json:"transformation_id"`
		ModelID          string `json:"model_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	// Submit transformation command
	nowStr := time.Now().UTC().Format(time.RFC3339)
	jobData := map[string]any{
		"app":            "open_notebook",
		"command":        "run_transformation",
		"status":         "pending",
		"created":        nowStr,
		"updated":        nowStr,
		"retry_attempts": 0,
		"input": map[string]any{
			"source_id":         sourceID,
			"transformation_id": payload.TransformationID,
			"model_id":          payload.ModelID,
		},
	}

	type CommandRecord struct {
		ID *models.RecordID `json:"id"`
	}

	res, err := db.RepoCreate[CommandRecord](r.Context(), "command", jobData)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to submit transformation job: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":            "pending",
		"message":           "Insight generation started",
		"source_id":         sourceID,
		"transformation_id": payload.TransformationID,
		"command_id":        res.ID.String(),
	})
}

// Helpers
func pointerToString(s string) *string {
	return &s
}

func parseCommandField(commandField any) (commandID *string, status *string, processingInfo map[string]any) {
	if commandField == nil {
		return nil, nil, nil
	}
	switch cmd := commandField.(type) {
	case string:
		return &cmd, pointerToString("unknown"), nil
	case map[string]any:
		idStr := ""
		if idVal, ok := cmd["id"]; ok {
			idStr = fmt.Sprintf("%v", idVal)
		}
		statusStr := ""
		if statusVal, ok := cmd["status"]; ok {
			statusStr = fmt.Sprintf("%v", statusVal)
		}
		errStr := ""
		if errVal, ok := cmd["error_message"]; ok {
			errStr = fmt.Sprintf("%v", errVal)
		}

		info := map[string]any{}
		if resVal, ok := cmd["result"]; ok {
			if resMap, ok := resVal.(map[string]any); ok {
				if execMeta, ok := resMap["execution_metadata"]; ok {
					if execMetaMap, ok := execMeta.(map[string]any); ok {
						info["started_at"] = execMetaMap["started_at"]
						info["completed_at"] = execMetaMap["completed_at"]
					}
				}
			}
		}
		if errStr != "" {
			info["error"] = errStr
		}
		return &idStr, &statusStr, info
	}
	return nil, nil, nil
}

func generateUniqueFilename(originalFilename string, uploadFolder string) (string, error) {
	cleanName := filepath.Base(originalFilename)
	if cleanName == "" || cleanName == "." || cleanName == "/" {
		return "", errors.New("invalid filename")
	}

	ext := filepath.Ext(cleanName)
	stem := strings.TrimSuffix(cleanName, ext)

	counter := 0
	for {
		var newName string
		if counter == 0 {
			newName = cleanName
		} else {
			newName = fmt.Sprintf("%s (%d)%s", stem, counter, ext)
		}

		fullPath := filepath.Join(uploadFolder, newName)
		fullPath = filepath.Clean(fullPath)
		absFolder, err := filepath.Abs(uploadFolder)
		if err != nil {
			return "", err
		}
		absPath, err := filepath.Abs(fullPath)
		if err != nil {
			return "", err
		}

		if !strings.HasPrefix(absPath, absFolder) {
			return "", errors.New("path traversal detected")
		}

		if _, err := os.Stat(absPath); os.IsNotExist(err) {
			return absPath, nil
		}
		counter++
	}
}
