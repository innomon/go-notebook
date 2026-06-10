package domain

import (
	"context"
	"fmt"
	"go-notebook/internal/db"
	"strings"
	"time"

	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// ChatMessage represents a single message in a chat session
type ChatMessage struct {
	ID        string `json:"id"`
	Type      string `json:"type"`                // "human" or "ai"
	Content   string `json:"content"`
	Timestamp any    `json:"timestamp,omitempty"` // null or string
}

// ChatSession represents a chat session in the database
type ChatSession struct {
	ID            *models.RecordID `json:"id,omitempty"`
	Title         string           `json:"title"`
	ModelOverride *string          `json:"model_override,omitempty"`
	Messages      []ChatMessage    `json:"messages"`
	Created       time.Time        `json:"created,omitempty"`
	Updated       time.Time        `json:"updated,omitempty"`
}

// ChatSessionResponse represents a ChatSession formatted for REST API responses
type ChatSessionResponse struct {
	ID            string        `json:"id"`
	Title         string        `json:"title"`
	NotebookID    *string       `json:"notebook_id,omitempty"`
	SourceID      *string       `json:"source_id,omitempty"`
	ModelOverride *string       `json:"model_override,omitempty"`
	Created       time.Time     `json:"created"`
	Updated       time.Time     `json:"updated"`
	MessageCount  int           `json:"message_count"`
	Messages      []ChatMessage `json:"messages,omitempty"`
}

// GetChatSession retrieves a chat session by record ID
func GetChatSession(ctx context.Context, id string) (*ChatSession, error) {
	recordID := db.EnsureRecordIDString("chat_session", id)
	results, err := db.RepoQuery[ChatSession](ctx, "SELECT * FROM ONLY $id;", map[string]any{"id": recordID})
	if err != nil {
		return nil, err
	}
	return results, nil
}

// CreateNotebookChatSession creates a new chat session and links it to a notebook
func CreateNotebookChatSession(ctx context.Context, title string, modelOverride *string, notebookID string) (*ChatSession, error) {
	now := time.Now().UTC()
	data := map[string]any{
		"title":    title,
		"messages": []ChatMessage{},
		"created":  now,
		"updated":  now,
	}
	if modelOverride != nil && *modelOverride != "" {
		data["model_override"] = *modelOverride
	}

	session, err := db.RepoCreate[ChatSession](ctx, "chat_session", data)
	if err != nil {
		return nil, err
	}

	sessionIDStr := session.ID.String()
	notebookIDStr := db.EnsureRecordIDString("notebook", notebookID)
	err = db.RepoRelate(ctx, sessionIDStr, "refers_to", notebookIDStr, nil)
	if err != nil {
		_ = db.RepoDelete(ctx, sessionIDStr)
		return nil, fmt.Errorf("failed to link chat session to notebook: %w", err)
	}

	return session, nil
}

// CreateSourceChatSession creates a new chat session and links it to a source
func CreateSourceChatSession(ctx context.Context, title string, modelOverride *string, sourceID string) (*ChatSession, error) {
	now := time.Now().UTC()
	data := map[string]any{
		"title":    title,
		"messages": []ChatMessage{},
		"created":  now,
		"updated":  now,
	}
	if modelOverride != nil && *modelOverride != "" {
		data["model_override"] = *modelOverride
	}

	session, err := db.RepoCreate[ChatSession](ctx, "chat_session", data)
	if err != nil {
		return nil, err
	}

	sessionIDStr := session.ID.String()
	sourceIDStr := db.EnsureRecordIDString("source", sourceID)
	err = db.RepoRelate(ctx, sessionIDStr, "refers_to", sourceIDStr, nil)
	if err != nil {
		_ = db.RepoDelete(ctx, sessionIDStr)
		return nil, fmt.Errorf("failed to link chat session to source: %w", err)
	}

	return session, nil
}

// ListNotebookChatSessions returns all chat sessions related to a notebook
func ListNotebookChatSessions(ctx context.Context, notebookID string) ([]ChatSession, error) {
	nbRecordID := db.EnsureRecordIDString("notebook", notebookID)
	type RelLink struct {
		Session ChatSession `json:"in"`
	}

	query := `
		SELECT in from refers_to 
		WHERE out = $notebook_id AND in.id != NONE 
		ORDER BY in.updated DESC 
		FETCH in;
	`
	links, err := db.RepoQuery[[]RelLink](ctx, query, map[string]any{"notebook_id": nbRecordID})
	if err != nil {
		return nil, err
	}
	if links == nil {
		return []ChatSession{}, nil
	}

	sessions := make([]ChatSession, 0, len(*links))
	for _, link := range *links {
		if link.Session.ID != nil {
			sessions = append(sessions, link.Session)
		}
	}
	return sessions, nil
}

// ListSourceChatSessions returns all chat sessions related to a source
func ListSourceChatSessions(ctx context.Context, sourceID string) ([]ChatSession, error) {
	srcRecordID := db.EnsureRecordIDString("source", sourceID)
	type RelLink struct {
		Session ChatSession `json:"in"`
	}

	query := `
		SELECT in from refers_to 
		WHERE out = $source_id AND in.id != NONE 
		ORDER BY in.updated DESC 
		FETCH in;
	`
	links, err := db.RepoQuery[[]RelLink](ctx, query, map[string]any{"source_id": srcRecordID})
	if err != nil {
		return nil, err
	}
	if links == nil {
		return []ChatSession{}, nil
	}

	sessions := make([]ChatSession, 0, len(*links))
	for _, link := range *links {
		if link.Session.ID != nil {
			sessions = append(sessions, link.Session)
		}
	}
	return sessions, nil
}

// GetChatSessionRelation returns the destination (notebook or source record ID) for a chat session
func GetChatSessionRelation(ctx context.Context, sessionID string) (string, string, error) {
	sessionRecordID := db.EnsureRecordIDString("chat_session", sessionID)
	type RelOut struct {
		Out *models.RecordID `json:"out"`
	}

	res, err := db.RepoQuery[[]RelOut](ctx, "SELECT out FROM refers_to WHERE in = $session_id LIMIT 1;", map[string]any{"session_id": sessionRecordID})
	if err != nil || res == nil || len(*res) == 0 || (*res)[0].Out == nil {
		return "", "", fmt.Errorf("no relation found for chat session: %w", err)
	}

	target := (*res)[0].Out
	targetStr := target.String()
	tb := ""
	if strings.HasPrefix(targetStr, "notebook:") {
		tb = "notebook"
	} else if strings.HasPrefix(targetStr, "source:") {
		tb = "source"
	}
	return tb, targetStr, nil
}

// UpdateChatSession updates the title and/or model override of a chat session
func UpdateChatSession(ctx context.Context, sessionID string, title *string, modelOverride *string, updateTime bool) (*ChatSession, error) {
	sessionRecordID := db.EnsureRecordIDString("chat_session", sessionID)
	data := make(map[string]any)

	if title != nil {
		data["title"] = *title
	}
	if modelOverride != nil {
		if *modelOverride == "" {
			data["model_override"] = nil
		} else {
			data["model_override"] = *modelOverride
		}
	}
	if updateTime {
		data["updated"] = time.Now().UTC()
	}

	return db.RepoUpdate[ChatSession](ctx, "chat_session", sessionRecordID, data)
}

// DeleteChatSession deletes a chat session and its relation
func DeleteChatSession(ctx context.Context, sessionID string) error {
	sessionRecordID := db.EnsureRecordIDString("chat_session", sessionID)

	// Delete relation
	_, _ = db.RepoQuery[any](ctx, "DELETE refers_to WHERE in = $session_id;", map[string]any{"session_id": sessionRecordID})

	// Delete session itself
	return db.RepoDelete(ctx, sessionRecordID)
}

// SaveChatMessage appends a message to the chat session's history
func SaveChatMessage(ctx context.Context, sessionID string, message ChatMessage) error {
	sessionRecordID := db.EnsureRecordIDString("chat_session", sessionID)

	query := `
		UPDATE $session_id SET 
			messages = array::append(messages, $msg),
			updated = $now;
	`
	now := time.Now().UTC()
	_, err := db.RepoQuery[any](ctx, query, map[string]any{
		"session_id": sessionRecordID,
		"msg":        message,
		"now":        now,
	})
	return err
}
