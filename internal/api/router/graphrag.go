package router

import (
	"encoding/json"
	"go-notebook/internal/ai"
	"go-notebook/internal/db"
	"go-notebook/internal/domain"
	"go-notebook/internal/graphrag"
	"net/http"
	"strconv"
)

// RegisterGraphRAGRoutes binds GraphRAG visualizer and query routes to the ServeMux
func RegisterGraphRAGRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/notebooks/{id}/graph/build", handleBuildNotebookGraph)
	mux.HandleFunc("POST /api/notebooks/{id}/graph/query", handleQueryNotebookGraph)
	mux.HandleFunc("GET /api/notebooks/{id}/graph", handleGetNotebookGraph)
}

func handleBuildNotebookGraph(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	notebookID := r.PathValue("id")
	if notebookID == "" {
		respondError(w, http.StatusBadRequest, "Notebook ID is required")
		return
	}

	// Verify notebook exists
	_, err := domain.GetNotebook(ctx, notebookID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Notebook not found")
		return
	}

	// Submit background command job to build the GraphRAG pipeline
	data := map[string]any{
		"command": "build_graphrag",
		"app":     "notebook",
		"status":  "pending",
		"input": map[string]any{
			"notebook_id": notebookID,
		},
	}

	job, err := db.RepoCreate[domain.CommandJob](ctx, "command", data)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to submit background build job: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status": "processing",
		"job_id": job.ID.String(),
	})
}

func handleQueryNotebookGraph(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	notebookID := r.PathValue("id")
	if notebookID == "" {
		respondError(w, http.StatusBadRequest, "Notebook ID is required")
		return
	}

	var payload struct {
		Query string `json:"query"`
		Mode  string `json:"mode"` // "local", "global", "hybrid"
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	if payload.Query == "" {
		respondError(w, http.StatusBadRequest, "Query is required")
		return
	}
	if payload.Mode == "" {
		payload.Mode = "hybrid"
	}

	// 1. Run RAG Graph retrieval pipeline to get context
	pipeline, err := graphrag.NewPipeline(ctx)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to initialize GraphRAG pipeline: "+err.Error())
		return
	}

	res, err := pipeline.Query(ctx, notebookID, payload.Query, payload.Mode)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "GraphRAG retrieval query failed: "+err.Error())
		return
	}

	// 2. Resolve LLM client and call generation
	chatClient, err := ai.GetClientForDefaultModel(ctx, "chat")
	if err != nil {
		respondError(w, http.StatusInternalServerError, "AI chat client offline: "+err.Error())
		return
	}

	systemPrompt := `You are an intelligent assistant analyzing text using a Knowledge Graph enhanced RAG system.
You have been given both entity-specific excerpts and thematic community summaries. Combine both for a comprehensive answer.

CRITICAL RULES:
1. Answer ONLY using information from the provided <context> tags. Do NOT use outside knowledge.
2. If the context doesn't contain enough information, say exactly: "This topic isn't sufficiently covered in the available text."
3. Keep the final answer concise, well-structured, and visually appealing for a user interface.
4. You MUST cite the sources of your information at the end of all responses you generate.

REASONING PROCESS (MANDATORY):
- First, write your step-by-step reasoning.
- Extract facts and compare them.`

	userPrompt := "<context>\n" + res.Context + "\n</context>\n\nQuery: " + payload.Query

	answer, err := chatClient.GenerateText(ctx, systemPrompt, userPrompt)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to generate answer from LLM: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"role":    "assistant",
		"content": answer,
		"metadata": map[string]any{
			"mode":     res.Mode,
			"sources":  res.Sources,
			"entities": res.Entities,
		},
	})
}

func handleGetNotebookGraph(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	notebookID := r.PathValue("id")
	if notebookID == "" {
		respondError(w, http.StatusBadRequest, "Notebook ID is required")
		return
	}

	maxNodes := 25
	if maxNodesVal := r.URL.Query().Get("max_nodes"); maxNodesVal != "" {
		if val, err := strconv.Atoi(maxNodesVal); err == nil && val > 0 {
			maxNodes = val
		}
	}

	graphData, err := domain.GetNotebookGraphData(ctx, notebookID, maxNodes)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to retrieve graph visualization data: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(graphData)
}
