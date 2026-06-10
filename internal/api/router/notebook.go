package router

import (
	"encoding/json"
	"go-notebook/internal/domain"
	"net/http"
	"strconv"
)

// RegisterNotebookRoutes binds notebook CRUD routes to the ServeMux
func RegisterNotebookRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/notebooks", handleListNotebooks)
	mux.HandleFunc("POST /api/notebooks", handleCreateNotebook)
	mux.HandleFunc("GET /api/notebooks/{id}", handleGetNotebook)
	mux.HandleFunc("PUT /api/notebooks/{id}", handleUpdateNotebook)
	mux.HandleFunc("DELETE /api/notebooks/{id}", handleDeleteNotebook)
	mux.HandleFunc("GET /api/notebooks/{id}/delete-preview", handleGetNotebookDeletePreview)
}

func handleListNotebooks(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	notebooks, err := domain.ListNotebooks(ctx)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to retrieve notebooks: "+err.Error())
		return
	}

	response := make([]domain.NotebookResponse, len(notebooks))
	for i, n := range notebooks {
		idStr := n.ID.String()
		sourceCount, noteCount, _ := domain.GetNotebookCounts(ctx, idStr)

		response[i] = domain.NotebookResponse{
			ID:          idStr,
			Name:        n.Name,
			Description: n.Description,
			Archived:    n.Archived,
			Created:     n.Created,
			Updated:     n.Updated,
			SourceCount: sourceCount,
			NoteCount:   noteCount,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func handleCreateNotebook(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	notebook, err := domain.CreateNotebook(r.Context(), payload.Name, payload.Description)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	response := domain.NotebookResponse{
		ID:          notebook.ID.String(),
		Name:        notebook.Name,
		Description: notebook.Description,
		Archived:    notebook.Archived,
		Created:     notebook.Created,
		Updated:     notebook.Updated,
		SourceCount: 0,
		NoteCount:   0,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(response)
}

func handleGetNotebook(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		respondError(w, http.StatusBadRequest, "Notebook ID is required")
		return
	}

	ctx := r.Context()
	notebook, err := domain.GetNotebook(ctx, id)
	if err != nil {
		respondError(w, http.StatusNotFound, "Notebook not found")
		return
	}

	sourceCount, noteCount, _ := domain.GetNotebookCounts(ctx, notebook.ID.String())

	response := domain.NotebookResponse{
		ID:          notebook.ID.String(),
		Name:        notebook.Name,
		Description: notebook.Description,
		Archived:    notebook.Archived,
		Created:     notebook.Created,
		Updated:     notebook.Updated,
		SourceCount: sourceCount,
		NoteCount:   noteCount,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func handleUpdateNotebook(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		respondError(w, http.StatusBadRequest, "Notebook ID is required")
		return
	}

	var payload struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
		Archived    *bool   `json:"archived"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	ctx := r.Context()
	notebook, err := domain.UpdateNotebook(ctx, id, payload.Name, payload.Description, payload.Archived)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	sourceCount, noteCount, _ := domain.GetNotebookCounts(ctx, notebook.ID.String())

	response := domain.NotebookResponse{
		ID:          notebook.ID.String(),
		Name:        notebook.Name,
		Description: notebook.Description,
		Archived:    notebook.Archived,
		Created:     notebook.Created,
		Updated:     notebook.Updated,
		SourceCount: sourceCount,
		NoteCount:   noteCount,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func handleDeleteNotebook(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		respondError(w, http.StatusBadRequest, "Notebook ID is required")
		return
	}

	// Read optional query parameter delete_exclusive_sources
	deleteExclusive := false
	if q := r.URL.Query().Get("delete_exclusive_sources"); q != "" {
		if b, err := strconv.ParseBool(q); err == nil {
			deleteExclusive = b
		}
	}

	deletedNotes, deletedSources, unlinkedSources, err := domain.DeleteNotebook(r.Context(), id, deleteExclusive)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to delete notebook: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]int{
		"deleted_notes":    deletedNotes,
		"deleted_sources":  deletedSources,
		"unlinked_sources": unlinkedSources,
	})
}

func handleGetNotebookDeletePreview(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		respondError(w, http.StatusBadRequest, "Notebook ID is required")
		return
	}

	preview, err := domain.GetNotebookDeletePreview(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusNotFound, "Notebook not found: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(preview)
}

// respondError is a helper to write standard JSON error detail messages
func respondError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"detail": message,
	})
}
