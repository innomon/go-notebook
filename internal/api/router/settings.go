package router

import (
	"encoding/json"
	"go-notebook/internal/domain"
	"net/http"
)

// RegisterSettingsRoutes binds settings REST routes to the ServeMux
func RegisterSettingsRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/settings", handleGetSettings)
	mux.HandleFunc("POST /api/settings", handleUpdateSettings)
}

func handleGetSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := domain.GetContentSettings(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to fetch settings: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(settings)
}

func handleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	var payload domain.ContentSettings

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	// Validate inputs matching python constraints if needed
	if payload.DefaultContentProcessingEngineDoc == "" {
		payload.DefaultContentProcessingEngineDoc = "auto"
	}
	if payload.DefaultContentProcessingEngineURL == "" {
		payload.DefaultContentProcessingEngineURL = "auto"
	}
	if payload.DefaultEmbeddingOption == "" {
		payload.DefaultEmbeddingOption = "ask"
	}
	if payload.AutoDeleteFiles == "" {
		payload.AutoDeleteFiles = "yes"
	}
	if len(payload.YoutubePreferredLanguages) == 0 {
		payload.YoutubePreferredLanguages = []string{"en"}
	}

	ctx := r.Context()
	err := domain.UpdateContentSettings(ctx, &payload)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to save settings: "+err.Error())
		return
	}

	// Return updated settings
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(payload)
}
