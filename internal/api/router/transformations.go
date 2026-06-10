package router

import (
	"encoding/json"
	"go-notebook/internal/domain"
	"net/http"
)

// RegisterTransformationRoutes binds transformation REST routes to the ServeMux
func RegisterTransformationRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/transformations", handleListTransformations)
	mux.HandleFunc("POST /api/transformations", handleCreateTransformation)
	mux.HandleFunc("GET /api/transformations/default-prompt", handleGetDefaultPrompt)
	mux.HandleFunc("PUT /api/transformations/default-prompt", handleUpdateDefaultPrompt)
	mux.HandleFunc("GET /api/transformations/{id}", handleGetTransformation)
	mux.HandleFunc("PUT /api/transformations/{id}", handleUpdateTransformation)
	mux.HandleFunc("DELETE /api/transformations/{id}", handleDeleteTransformation)
}

func handleListTransformations(w http.ResponseWriter, r *http.Request) {
	transformations, err := domain.ListTransformations(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to retrieve transformations: "+err.Error())
		return
	}

	response := make([]domain.TransformationResponse, len(transformations))
	for i, t := range transformations {
		response[i] = domain.TransformationResponse{
			ID:           t.ID.String(),
			Name:         t.Name,
			Title:        t.Title,
			Description:  t.Description,
			Prompt:       t.Prompt,
			ApplyDefault: t.ApplyDefault,
			Created:      t.Created,
			Updated:      t.Updated,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func handleCreateTransformation(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Name         string `json:"name"`
		Title        string `json:"title"`
		Description  string `json:"description"`
		Prompt       string `json:"prompt"`
		ApplyDefault bool   `json:"apply_default"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	transformation, err := domain.CreateTransformation(
		r.Context(),
		payload.Name,
		payload.Title,
		payload.Description,
		payload.Prompt,
		payload.ApplyDefault,
	)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	response := domain.TransformationResponse{
		ID:           transformation.ID.String(),
		Name:         transformation.Name,
		Title:        transformation.Title,
		Description:  transformation.Description,
		Prompt:       transformation.Prompt,
		ApplyDefault: transformation.ApplyDefault,
		Created:      transformation.Created,
		Updated:      transformation.Updated,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(response)
}

func handleGetTransformation(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		respondError(w, http.StatusBadRequest, "Transformation ID is required")
		return
	}

	transformation, err := domain.GetTransformation(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusNotFound, "Transformation not found")
		return
	}

	response := domain.TransformationResponse{
		ID:           transformation.ID.String(),
		Name:         transformation.Name,
		Title:        transformation.Title,
		Description:  transformation.Description,
		Prompt:       transformation.Prompt,
		ApplyDefault: transformation.ApplyDefault,
		Created:      transformation.Created,
		Updated:      transformation.Updated,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func handleUpdateTransformation(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		respondError(w, http.StatusBadRequest, "Transformation ID is required")
		return
	}

	var payload struct {
		Name         *string `json:"name"`
		Title        *string `json:"title"`
		Description  *string `json:"description"`
		Prompt       *string `json:"prompt"`
		ApplyDefault *bool   `json:"apply_default"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	transformation, err := domain.UpdateTransformation(
		r.Context(),
		id,
		payload.Name,
		payload.Title,
		payload.Description,
		payload.Prompt,
		payload.ApplyDefault,
	)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	response := domain.TransformationResponse{
		ID:           transformation.ID.String(),
		Name:         transformation.Name,
		Title:        transformation.Title,
		Description:  transformation.Description,
		Prompt:       transformation.Prompt,
		ApplyDefault: transformation.ApplyDefault,
		Created:      transformation.Created,
		Updated:      transformation.Updated,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func handleDeleteTransformation(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		respondError(w, http.StatusBadRequest, "Transformation ID is required")
		return
	}

	if err := domain.DeleteTransformation(r.Context(), id); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to delete transformation: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"message": "Transformation deleted successfully",
	})
}

func handleGetDefaultPrompt(w http.ResponseWriter, r *http.Request) {
	defaultPrompts, err := domain.GetDefaultPrompts(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to fetch default prompt: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"transformation_instructions": defaultPrompts.TransformationInstructions,
	})
}

func handleUpdateDefaultPrompt(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		TransformationInstructions string `json:"transformation_instructions"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	ctx := r.Context()
	defaultPrompts := &domain.DefaultPrompts{
		TransformationInstructions: payload.TransformationInstructions,
	}

	err := domain.UpdateDefaultPrompts(ctx, defaultPrompts)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to save default prompt: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"transformation_instructions": defaultPrompts.TransformationInstructions,
	})
}
