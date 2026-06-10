package router

import (
	"encoding/json"
	"go-notebook/internal/domain"
	"net/http"
)

// RegisterNoteRoutes binds note REST routes to the ServeMux
func RegisterNoteRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/notes", handleListNotes)
	mux.HandleFunc("POST /api/notes", handleCreateNote)
	mux.HandleFunc("GET /api/notes/{id}", handleGetNote)
	mux.HandleFunc("PUT /api/notes/{id}", handleUpdateNote)
	mux.HandleFunc("DELETE /api/notes/{id}", handleDeleteNote)
}

func handleListNotes(w http.ResponseWriter, r *http.Request) {
	notebookID := r.URL.Query().Get("notebook_id")
	if notebookID == "" {
		respondError(w, http.StatusBadRequest, "notebook_id query parameter is required")
		return
	}

	notes, err := domain.ListNotebookNotes(r.Context(), notebookID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to retrieve notes: "+err.Error())
		return
	}

	response := make([]domain.NoteResponse, len(notes))
	for i, n := range notes {
		response[i] = domain.NoteResponse{
			ID:       n.ID.String(),
			Title:    n.Title,
			Content:  n.Content,
			NoteType: n.NoteType,
			Created:  n.Created,
			Updated:  n.Updated,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func handleCreateNote(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Title      string `json:"title"`
		Content    string `json:"content"`
		NoteType   string `json:"note_type"`
		NotebookID string `json:"notebook_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	note, commandID, err := domain.CreateNote(r.Context(), payload.Title, payload.Content, payload.NoteType, payload.NotebookID)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	response := domain.NoteResponse{
		ID:        note.ID.String(),
		Title:     note.Title,
		Content:   note.Content,
		NoteType:  note.NoteType,
		Created:   note.Created,
		Updated:   note.Updated,
		CommandID: commandID,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(response)
}

func handleGetNote(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		respondError(w, http.StatusBadRequest, "Note ID is required")
		return
	}

	note, err := domain.GetNote(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusNotFound, "Note not found")
		return
	}

	response := domain.NoteResponse{
		ID:       note.ID.String(),
		Title:    note.Title,
		Content:  note.Content,
		NoteType: note.NoteType,
		Created:  note.Created,
		Updated:  note.Updated,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func handleUpdateNote(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		respondError(w, http.StatusBadRequest, "Note ID is required")
		return
	}

	var payload struct {
		Title    *string `json:"title"`
		Content  *string `json:"content"`
		NoteType *string `json:"note_type"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	note, commandID, err := domain.UpdateNote(r.Context(), id, payload.Title, payload.Content, payload.NoteType)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	response := domain.NoteResponse{
		ID:        note.ID.String(),
		Title:     note.Title,
		Content:   note.Content,
		NoteType:  note.NoteType,
		Created:   note.Created,
		Updated:   note.Updated,
		CommandID: commandID,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func handleDeleteNote(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		respondError(w, http.StatusBadRequest, "Note ID is required")
		return
	}

	if err := domain.DeleteNote(r.Context(), id); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to delete note: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"message": "Note deleted successfully",
	})
}
