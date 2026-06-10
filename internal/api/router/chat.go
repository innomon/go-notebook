package router

import (
	"context"
	"encoding/json"
	"fmt"
	"go-notebook/internal/ai"
	"go-notebook/internal/domain"
	"go-notebook/internal/utils"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/flosch/pongo2/v6"
)

// RegisterChatRoutes binds chat and context routes to the ServeMux
func RegisterChatRoutes(mux *http.ServeMux) {
	// Notebook Chat Session management
	mux.HandleFunc("GET /api/chat/sessions", handleListNotebookChatSessions)
	mux.HandleFunc("POST /api/chat/sessions", handleCreateNotebookChatSession)
	mux.HandleFunc("GET /api/chat/sessions/{session_id}", handleGetChatSession)
	mux.HandleFunc("PUT /api/chat/sessions/{session_id}", handleUpdateChatSession)
	mux.HandleFunc("DELETE /api/chat/sessions/{session_id}", handleDeleteChatSession)

	// Notebook Chat Execution
	mux.HandleFunc("POST /api/chat/execute", handleExecuteChat)
	mux.HandleFunc("POST /api/chat/context", handleBuildContext)

	// Notebook Context (Alternate API route)
	mux.HandleFunc("POST /api/notebooks/{notebook_id}/context", handleGetNotebookContext)

	// Source Chat Session management
	mux.HandleFunc("GET /api/sources/{source_id}/chat/sessions", handleListSourceChatSessions)
	mux.HandleFunc("POST /api/sources/{source_id}/chat/sessions", handleCreateSourceChatSession)
	mux.HandleFunc("GET /api/sources/{source_id}/chat/sessions/{session_id}", handleGetSourceChatSession)
	mux.HandleFunc("PUT /api/sources/{source_id}/chat/sessions/{session_id}", handleUpdateSourceChatSession)
	mux.HandleFunc("DELETE /api/sources/{source_id}/chat/sessions/{session_id}", handleDeleteSourceChatSession)

	// Source Chat Message Sending (SSE Streaming)
	mux.HandleFunc("POST /api/sources/{source_id}/chat/sessions/{session_id}/messages", handleSendSourceChatMessage)
}

// --- Notebook Chat Session Handlers ---

func handleListNotebookChatSessions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	notebookID := r.URL.Query().Get("notebook_id")
	if notebookID == "" {
		respondError(w, http.StatusBadRequest, "notebook_id query parameter is required")
		return
	}

	sessions, err := domain.ListNotebookChatSessions(ctx, notebookID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to retrieve chat sessions: "+err.Error())
		return
	}

	response := make([]domain.ChatSessionResponse, len(sessions))
	for i, s := range sessions {
		response[i] = domain.ChatSessionResponse{
			ID:            s.ID.String(),
			Title:         s.Title,
			NotebookID:    &notebookID,
			ModelOverride: s.ModelOverride,
			Created:       s.Created,
			Updated:       s.Updated,
			MessageCount:  len(s.Messages),
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func handleCreateNotebookChatSession(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		NotebookID    string  `json:"notebook_id"`
		Title         string  `json:"title"`
		ModelOverride *string `json:"model_override"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	if payload.NotebookID == "" {
		respondError(w, http.StatusBadRequest, "notebook_id is required")
		return
	}

	if payload.Title == "" {
		payload.Title = fmt.Sprintf("Chat %d", time.Now().Unix())
	}

	session, err := domain.CreateNotebookChatSession(r.Context(), payload.Title, payload.ModelOverride, payload.NotebookID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to create chat session: "+err.Error())
		return
	}

	response := domain.ChatSessionResponse{
		ID:            session.ID.String(),
		Title:         session.Title,
		NotebookID:    &payload.NotebookID,
		ModelOverride: session.ModelOverride,
		Created:       session.Created,
		Updated:       session.Updated,
		MessageCount:  0,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(response)
}

func handleGetChatSession(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("session_id")
	if sessionID == "" {
		respondError(w, http.StatusBadRequest, "Session ID is required")
		return
	}

	ctx := r.Context()
	session, err := domain.GetChatSession(ctx, sessionID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Session not found")
		return
	}

	_, relID, err := domain.GetChatSessionRelation(ctx, sessionID)
	var notebookID *string
	if err == nil {
		notebookID = &relID
	}

	response := domain.ChatSessionResponse{
		ID:            session.ID.String(),
		Title:         session.Title,
		NotebookID:    notebookID,
		ModelOverride: session.ModelOverride,
		Created:       session.Created,
		Updated:       session.Updated,
		MessageCount:  len(session.Messages),
		Messages:      session.Messages,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func handleUpdateChatSession(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("session_id")
	if sessionID == "" {
		respondError(w, http.StatusBadRequest, "Session ID is required")
		return
	}

	var payload struct {
		Title         *string `json:"title"`
		ModelOverride *string `json:"model_override"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	ctx := r.Context()
	session, err := domain.UpdateChatSession(ctx, sessionID, payload.Title, payload.ModelOverride, true)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to update session: "+err.Error())
		return
	}

	_, relID, err := domain.GetChatSessionRelation(ctx, sessionID)
	var notebookID *string
	if err == nil {
		notebookID = &relID
	}

	response := domain.ChatSessionResponse{
		ID:            session.ID.String(),
		Title:         session.Title,
		NotebookID:    notebookID,
		ModelOverride: session.ModelOverride,
		Created:       session.Created,
		Updated:       session.Updated,
		MessageCount:  len(session.Messages),
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func handleDeleteChatSession(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("session_id")
	if sessionID == "" {
		respondError(w, http.StatusBadRequest, "Session ID is required")
		return
	}

	err := domain.DeleteChatSession(r.Context(), sessionID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to delete session: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"message": "Session deleted successfully",
	})
}

// --- Source Chat Session Handlers ---

func handleListSourceChatSessions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sourceID := r.PathValue("source_id")
	if sourceID == "" {
		respondError(w, http.StatusBadRequest, "Source ID is required")
		return
	}

	sessions, err := domain.ListSourceChatSessions(ctx, sourceID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to retrieve source chat sessions: "+err.Error())
		return
	}

	response := make([]domain.ChatSessionResponse, len(sessions))
	for i, s := range sessions {
		response[i] = domain.ChatSessionResponse{
			ID:            s.ID.String(),
			Title:         s.Title,
			SourceID:      &sourceID,
			ModelOverride: s.ModelOverride,
			Created:       s.Created,
			Updated:       s.Updated,
			MessageCount:  len(s.Messages),
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func handleCreateSourceChatSession(w http.ResponseWriter, r *http.Request) {
	sourceID := r.PathValue("source_id")
	if sourceID == "" {
		respondError(w, http.StatusBadRequest, "Source ID is required")
		return
	}

	var payload struct {
		Title         string  `json:"title"`
		ModelOverride *string `json:"model_override"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	if payload.Title == "" {
		payload.Title = fmt.Sprintf("Source Chat %d", time.Now().Unix())
	}

	session, err := domain.CreateSourceChatSession(r.Context(), payload.Title, payload.ModelOverride, sourceID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to create source chat session: "+err.Error())
		return
	}

	response := domain.ChatSessionResponse{
		ID:            session.ID.String(),
		Title:         session.Title,
		SourceID:      &sourceID,
		ModelOverride: session.ModelOverride,
		Created:       session.Created,
		Updated:       session.Updated,
		MessageCount:  0,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(response)
}

func handleGetSourceChatSession(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("session_id")
	sourceID := r.PathValue("source_id")

	ctx := r.Context()
	session, err := domain.GetChatSession(ctx, sessionID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Session not found")
		return
	}

	response := domain.ChatSessionResponse{
		ID:            session.ID.String(),
		Title:         session.Title,
		SourceID:      &sourceID,
		ModelOverride: session.ModelOverride,
		Created:       session.Created,
		Updated:       session.Updated,
		MessageCount:  len(session.Messages),
		Messages:      session.Messages,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func handleUpdateSourceChatSession(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("session_id")
	sourceID := r.PathValue("source_id")

	var payload struct {
		Title         *string `json:"title"`
		ModelOverride *string `json:"model_override"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	ctx := r.Context()
	session, err := domain.UpdateChatSession(ctx, sessionID, payload.Title, payload.ModelOverride, true)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to update session: "+err.Error())
		return
	}

	response := domain.ChatSessionResponse{
		ID:            session.ID.String(),
		Title:         session.Title,
		SourceID:      &sourceID,
		ModelOverride: session.ModelOverride,
		Created:       session.Created,
		Updated:       session.Updated,
		MessageCount:  len(session.Messages),
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func handleDeleteSourceChatSession(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("session_id")
	if sessionID == "" {
		respondError(w, http.StatusBadRequest, "Session ID is required")
		return
	}

	err := domain.DeleteChatSession(r.Context(), sessionID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to delete session: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"message": "Source chat session deleted successfully",
	})
}

// --- Chat Context building & execution ---

type ContextConfig struct {
	Sources map[string]string `json:"sources"`
	Notes   map[string]string `json:"notes"`
}

type BuildContextRequest struct {
	NotebookID    string         `json:"notebook_id"`
	ContextConfig *ContextConfig `json:"context_config"`
}

type BuildContextResponse struct {
	Context    map[string][]map[string]any `json:"context"`
	TokenCount int                         `json:"token_count"`
	CharCount  int                         `json:"char_count"`
}

func handleBuildContext(w http.ResponseWriter, r *http.Request) {
	var payload BuildContextRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	ctx := r.Context()
	resContext, tokenCount, charCount, err := buildContextInternal(ctx, payload.NotebookID, payload.ContextConfig)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Error building context: "+err.Error())
		return
	}

	response := BuildContextResponse{
		Context:    resContext,
		TokenCount: tokenCount,
		CharCount:  charCount,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func handleGetNotebookContext(w http.ResponseWriter, r *http.Request) {
	notebookID := r.PathValue("notebook_id")
	if notebookID == "" {
		respondError(w, http.StatusBadRequest, "Notebook ID is required")
		return
	}

	var payload struct {
		ContextConfig *ContextConfig `json:"context_config"`
	}

	// Body is optional
	_ = json.NewDecoder(r.Body).Decode(&payload)

	ctx := r.Context()
	resContext, tokenCount, _, err := buildContextInternal(ctx, notebookID, payload.ContextConfig)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Error building context: "+err.Error())
		return
	}

	// Replicate python ContextResponse format
	type ContextResponse struct {
		NotebookID  string           `json:"notebook_id"`
		Sources     []map[string]any `json:"sources"`
		Notes       []map[string]any `json:"notes"`
		TotalTokens int              `json:"total_tokens"`
	}

	response := ContextResponse{
		NotebookID:  notebookID,
		Sources:     resContext["sources"],
		Notes:       resContext["notes"],
		TotalTokens: tokenCount,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func buildContextInternal(ctx context.Context, notebookID string, cfg *ContextConfig) (map[string][]map[string]any, int, int, error) {
	_, err := domain.GetNotebook(ctx, notebookID)
	if err != nil {
		return nil, 0, 0, err
	}

	resSources := []map[string]any{}
	resNotes := []map[string]any{}
	var totalContent strings.Builder

	if cfg != nil && (len(cfg.Sources) > 0 || len(cfg.Notes) > 0) {
		// Process sources
		for sourceID, status := range cfg.Sources {
			if strings.Contains(status, "not in") {
				continue
			}

			src, err := domain.GetSource(ctx, sourceID)
			if err != nil {
				continue
			}

			size := "short"
			if strings.Contains(status, "full content") {
				size = "long"
			}

			srcCtx, err := src.GetContext(ctx, size)
			if err == nil {
				resSources = append(resSources, srcCtx)
				totalContent.WriteString(fmt.Sprintf("%v", srcCtx))
			}
		}

		// Process notes
		for noteID, status := range cfg.Notes {
			if strings.Contains(status, "not in") {
				continue
			}

			note, err := domain.GetNote(ctx, noteID)
			if err != nil {
				continue
			}

			size := "short"
			if strings.Contains(status, "full content") {
				size = "long"
			}

			noteCtx := note.GetContext(size)
			resNotes = append(resNotes, noteCtx)
			totalContent.WriteString(fmt.Sprintf("%v", noteCtx))
		}
	} else {
		// Default: list all sources (short/insights) and all notes (short)
		sources, err := domain.GetNotebookSources(ctx, notebookID)
		if err == nil {
			for _, src := range sources {
				srcCtx, err := src.GetContext(ctx, "short")
				if err == nil {
					resSources = append(resSources, srcCtx)
					totalContent.WriteString(fmt.Sprintf("%v", srcCtx))
				}
			}
		}

		notes, err := domain.ListNotebookNotes(ctx, notebookID)
		if err == nil {
			for _, note := range notes {
				noteCtx := note.GetContext("short")
				resNotes = append(resNotes, noteCtx)
				totalContent.WriteString(fmt.Sprintf("%v", noteCtx))
			}
		}
	}

	contentStr := totalContent.String()
	tokenCount := utils.TokenCount(contentStr)
	charCount := len(contentStr)

	res := map[string][]map[string]any{
		"sources": resSources,
		"notes":   resNotes,
	}

	return res, tokenCount, charCount, nil
}

// handleExecuteChat executes a chat request synchronously and returns the complete message history
func handleExecuteChat(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		SessionID     string `json:"session_id"`
		Message       string `json:"message"`
		Context       string `json:"context"`
		ModelOverride string `json:"model_override"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	if payload.SessionID == "" || payload.Message == "" {
		respondError(w, http.StatusBadRequest, "session_id and message are required")
		return
	}

	ctx := r.Context()
	session, err := domain.GetChatSession(ctx, payload.SessionID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Session not found")
		return
	}

	// Get notebook related to this session
	relType, notebookID, err := domain.GetChatSessionRelation(ctx, payload.SessionID)
	var notebook *domain.Notebook
	if err == nil && relType == "notebook" {
		notebook, _ = domain.GetNotebook(ctx, notebookID)
	}

	// Determine model
	modelID := payload.ModelOverride
	if modelID == "" && session.ModelOverride != nil {
		modelID = *session.ModelOverride
	}

	var client ai.AIClient
	if modelID != "" {
		client, err = ai.GetClientForModel(ctx, modelID)
	} else {
		client, err = ai.GetClientForDefaultModel(ctx, "chat")
	}

	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to initialize AI Client: "+err.Error())
		return
	}

	// Render prompt template
	tpl, err := pongo2.FromFile("prompts/chat/system.jinja")
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to load system prompt: "+err.Error())
		return
	}

	var nbMap map[string]any
	if notebook != nil {
		nbMap = map[string]any{
			"name":        notebook.Name,
			"description": notebook.Description,
		}
	}

	systemPrompt, err := tpl.Execute(pongo2.Context{
		"notebook": nbMap,
		"context":  payload.Context,
	})
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to render system prompt: "+err.Error())
		return
	}

	// Map chat history
	aiMessages := make([]ai.ChatMessage, len(session.Messages))
	for i, m := range session.Messages {
		role := "user"
		if m.Type == "ai" {
			role = "assistant"
		}
		aiMessages[i] = ai.ChatMessage{
			Role:    role,
			Content: m.Content,
		}
	}

	// Generate text
	reply, err := client.GenerateText(ctx, systemPrompt, payload.Message)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Model generation failed: "+err.Error())
		return
	}

	// Save human message
	humanMsg := domain.ChatMessage{
		ID:      fmt.Sprintf("msg_user_%d", len(session.Messages)),
		Type:    "human",
		Content: payload.Message,
	}
	_ = domain.SaveChatMessage(ctx, payload.SessionID, humanMsg)

	// Save AI message
	aiMsg := domain.ChatMessage{
		ID:      fmt.Sprintf("msg_ai_%d", len(session.Messages)+1),
		Type:    "ai",
		Content: reply,
	}
	_ = domain.SaveChatMessage(ctx, payload.SessionID, aiMsg)

	// Fetch updated session messages
	updatedSession, err := domain.GetChatSession(ctx, payload.SessionID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to reload chat session: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"session_id": payload.SessionID,
		"messages":   updatedSession.Messages,
	})
}

// handleSendSourceChatMessage handles sending messages to a source chat session using SSE streaming
func handleSendSourceChatMessage(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		respondError(w, http.StatusInternalServerError, "Streaming is not supported by this server")
		return
	}

	sourceID := r.PathValue("source_id")
	sessionID := r.PathValue("session_id")

	var payload struct {
		Message       string `json:"message"`
		ModelOverride string `json:"model_override"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	if payload.Message == "" {
		respondError(w, http.StatusBadRequest, "message is required")
		return
	}

	ctx := r.Context()

	// Verify source and session
	source, err := domain.GetSource(ctx, sourceID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Source not found")
		return
	}

	session, err := domain.GetChatSession(ctx, sessionID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Session not found")
		return
	}

	// Verify session relation refers to this source
	relType, relID, err := domain.GetChatSessionRelation(ctx, sessionID)
	if err != nil || relType != "source" || relID != source.ID.String() {
		respondError(w, http.StatusNotFound, "Session not found for this source")
		return
	}

	// Determine model
	modelID := payload.ModelOverride
	if modelID == "" && session.ModelOverride != nil {
		modelID = *session.ModelOverride
	}

	var client ai.AIClient
	if modelID != "" {
		client, err = ai.GetClientForModel(ctx, modelID)
	} else {
		client, err = ai.GetClientForDefaultModel(ctx, "chat")
	}

	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to initialize AI Client: "+err.Error())
		return
	}

	// Set SSE Headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	// Emit user message event first
	userEvent := map[string]any{
		"type":      "user_message",
		"content":   payload.Message,
		"timestamp": nil,
	}
	userEventBytes, _ := json.Marshal(userEvent)
	_, _ = fmt.Fprintf(w, "data: %s\n\n", string(userEventBytes))
	flusher.Flush()

	// Gather source context & insights
	insights, _ := domain.GetSourceInsights(ctx, sourceID)

	var sb strings.Builder
	sb.WriteString("## SOURCE CONTENT\n")
	sb.WriteString(fmt.Sprintf("**Source ID:** %s\n", source.ID.String()))
	sb.WriteString(fmt.Sprintf("**Title:** %s\n", source.Title))
	if source.FullText != "" {
		ft := source.FullText
		if len(ft) > 5000 {
			ft = ft[:5000] + "...\n[Content truncated]"
		}
		sb.WriteString(fmt.Sprintf("**Content:**\n%s\n", ft))
	}
	sb.WriteString("\n")

	insightIDs := make([]string, len(insights))
	if len(insights) > 0 {
		sb.WriteString("## SOURCE INSIGHTS\n")
		for i, ins := range insights {
			insightIDs[i] = ins.ID.String()
			sb.WriteString(fmt.Sprintf("**Insight ID:** %s\n", ins.ID.String()))
			sb.WriteString(fmt.Sprintf("**Type:** %s\n", ins.InsightType))
			sb.WriteString(fmt.Sprintf("**Content:** %s\n\n", ins.Content))
		}
	}
	formattedContext := sb.String()

	// Render prompt
	tpl, err := pongo2.FromFile("prompts/source_chat/system.jinja")
	if err != nil {
		log.Printf("Failed to load source_chat template: %v", err)
		errorEvent := map[string]any{
			"type":    "error",
			"message": "Failed to load chat prompt template: " + err.Error(),
		}
		eventBytes, _ := json.Marshal(errorEvent)
		_, _ = fmt.Fprintf(w, "data: %s\n\n", string(eventBytes))
		flusher.Flush()
		return
	}

	srcMap := map[string]any{
		"id":     source.ID.String(),
		"title":  source.Title,
		"topics": source.Topics,
	}

	systemPrompt, err := tpl.Execute(pongo2.Context{
		"source":  srcMap,
		"context": formattedContext,
	})
	if err != nil {
		log.Printf("Failed to execute source_chat template: %v", err)
		errorEvent := map[string]any{
			"type":    "error",
			"message": "Failed to render chat prompt template: " + err.Error(),
		}
		eventBytes, _ := json.Marshal(errorEvent)
		_, _ = fmt.Fprintf(w, "data: %s\n\n", string(eventBytes))
		flusher.Flush()
		return
	}

	// Map chat history
	aiMessages := make([]ai.ChatMessage, len(session.Messages))
	for i, m := range session.Messages {
		role := "user"
		if m.Type == "ai" {
			role = "assistant"
		}
		aiMessages[i] = ai.ChatMessage{
			Role:    role,
			Content: m.Content,
		}
	}
	// Append current message
	aiMessages = append(aiMessages, ai.ChatMessage{
		Role:    "user",
		Content: payload.Message,
	})

	var responseBuilder strings.Builder

	err = client.GenerateChatStream(ctx, systemPrompt, aiMessages, func(token string) {
		responseBuilder.WriteString(token)

		aiEvent := map[string]any{
			"type":      "ai_message",
			"content":   token,
			"timestamp": nil,
		}
		eventBytes, _ := json.Marshal(aiEvent)
		_, _ = fmt.Fprintf(w, "data: %s\n\n", string(eventBytes))
		flusher.Flush()
	})

	if err != nil {
		log.Printf("Error generating source chat stream: %v", err)
		errorEvent := map[string]any{
			"type":    "error",
			"message": "Stream generation failed: " + err.Error(),
		}
		eventBytes, _ := json.Marshal(errorEvent)
		_, _ = fmt.Fprintf(w, "data: %s\n\n", string(eventBytes))
		flusher.Flush()
		return
	}

	// Emit context indicators
	contextEvent := map[string]any{
		"type": "context_indicators",
		"data": map[string]any{
			"sources":  []string{source.ID.String()},
			"insights": insightIDs,
			"notes":    []string{},
		},
	}
	contextEventBytes, _ := json.Marshal(contextEvent)
	_, _ = fmt.Fprintf(w, "data: %s\n\n", string(contextEventBytes))
	flusher.Flush()

	// Emit complete event
	completeEvent := map[string]any{
		"type": "complete",
	}
	completeEventBytes, _ := json.Marshal(completeEvent)
	_, _ = fmt.Fprintf(w, "data: %s\n\n", string(completeEventBytes))
	flusher.Flush()

	// Save messages to database
	humanMsg := domain.ChatMessage{
		ID:      fmt.Sprintf("msg_user_%d", len(session.Messages)),
		Type:    "human",
		Content: payload.Message,
	}
	_ = domain.SaveChatMessage(ctx, sessionID, humanMsg)

	aiMsg := domain.ChatMessage{
		ID:      fmt.Sprintf("msg_ai_%d", len(session.Messages)+1),
		Type:    "ai",
		Content: responseBuilder.String(),
	}
	_ = domain.SaveChatMessage(ctx, sessionID, aiMsg)

	// Save updated timestamp on session
	_, _ = domain.UpdateChatSession(ctx, sessionID, nil, nil, true)
}
