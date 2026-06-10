package router

import (
	"encoding/json"
	"go-notebook/internal/domain"
	"net/http"
)

// RegisterInsightRoutes binds insight REST routes to the ServeMux
func RegisterInsightRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/insights/{insight_id}", handleGetInsight)
	mux.HandleFunc("DELETE /api/insights/{insight_id}", handleDeleteInsight)
	mux.HandleFunc("POST /api/insights/{insight_id}/save-as-note", handleSaveInsightAsNote)
}

func handleGetInsight(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("insight_id")
	if id == "" {
		respondError(w, http.StatusBadRequest, "Insight ID is required")
		return
	}

	insight, err := domain.GetSourceInsight(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusNotFound, "Insight not found")
		return
	}

	// Format response matching python SourceInsightResponse
	type InsightResponse struct {
		ID          string `json:"id"`
		SourceID    string `json:"source_id"`
		InsightType string `json:"insight_type"`
		Content     string `json:"content"`
		Created     string `json:"created"`
		Updated     string `json:"updated"`
	}

	response := InsightResponse{
		ID:          insight.ID.String(),
		SourceID:    insight.Source.String(),
		InsightType: insight.InsightType,
		Content:     insight.Content,
		Created:     insight.Created,
		Updated:     insight.Updated,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func handleDeleteInsight(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("insight_id")
	if id == "" {
		respondError(w, http.StatusBadRequest, "Insight ID is required")
		return
	}

	if err := domain.DeleteSourceInsight(r.Context(), id); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to delete insight: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"message": "Insight deleted successfully",
	})
}

func handleSaveInsightAsNote(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("insight_id")
	if id == "" {
		respondError(w, http.StatusBadRequest, "Insight ID is required")
		return
	}

	var payload struct {
		NotebookID string `json:"notebook_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	note, err := domain.SaveInsightAsNote(r.Context(), id, payload.NotebookID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to save insight as note: "+err.Error())
		return
	}

	// Format response matching domain.NoteResponse
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
