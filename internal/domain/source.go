package domain

import (
	"context"
	"errors"
	"fmt"
	"go-notebook/internal/db"
	"log"
	"strings"
	"time"

	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// Asset represents the source file or url
type Asset struct {
	FilePath string `json:"file_path,omitempty"`
	URL      string `json:"url,omitempty"`
}

// Source represents a document or web page imported into a notebook
type Source struct {
	ID        *models.RecordID `json:"id,omitempty"`
	Asset     *Asset           `json:"asset,omitempty"`
	Title     string           `json:"title,omitempty"`
	Topics    []string         `json:"topics,omitempty"`
	FullText  string           `json:"full_text,omitempty"`
	Command   *models.RecordID `json:"command,omitempty"`
	Hash          string           `json:"hash,omitempty"`
	LastGraphHash string           `json:"last_graph_hash,omitempty"`
	Created       time.Time        `json:"created,omitempty"`
	Updated       time.Time        `json:"updated,omitempty"`
}

// CommandJob represents a queued or executing background worker command
type CommandJob struct {
	ID            *models.RecordID `json:"id,omitempty"`
	App           string           `json:"app"`
	Command       string           `json:"command"`
	Status        string           `json:"status"`
	Created       time.Time        `json:"created"`
	Updated       time.Time        `json:"updated"`
	RetryAttempts int              `json:"retry_attempts"`
	Input         map[string]any   `json:"input"`
	ErrorMessage  *string          `json:"error_message,omitempty"`
	Result        map[string]any   `json:"result,omitempty"`
}

// SourceInsight represents a generated summary or key fact about a source
type SourceInsight struct {
	ID          *models.RecordID `json:"id,omitempty"`
	Source      *models.RecordID `json:"source"`
	InsightType string           `json:"insight_type"`
	Content     string           `json:"content"`
	Created     time.Time        `json:"created,omitempty"`
	Updated     time.Time        `json:"updated,omitempty"`
}

// SourceEmbedding represents a single text chunk and its vector embedding
type SourceEmbedding struct {
	ID        *models.RecordID `json:"id,omitempty"`
	Source    *models.RecordID `json:"source"`
	Order     int              `json:"order"`
	Content   string           `json:"content"`
	Embedding []float32        `json:"embedding,omitempty"`
}

// GetSource retrieves a source by ID
func GetSource(ctx context.Context, id string) (*Source, error) {
	recordID := db.EnsureRecordID("source", id)
	results, err := db.RepoQuery[Source](ctx, "SELECT * FROM ONLY $id;", map[string]any{"id": recordID})
	if err != nil {
		return nil, err
	}
	return results, nil
}

// GetCommandJob retrieves a command job by record ID
func GetCommandJob(ctx context.Context, commandID string) (*CommandJob, error) {
	recordObj := db.EnsureRecordID("command", commandID)
	results, err := db.RepoQuery[CommandJob](ctx, "SELECT * FROM ONLY $id;", map[string]any{"id": recordObj})
	if err != nil {
		return nil, err
	}
	return results, nil
}

// LinkSourceToNotebook relates a source to a notebook
func LinkSourceToNotebook(ctx context.Context, sourceID, notebookID string) error {
	src := db.EnsureRecordIDString("source", sourceID)
	nb := db.EnsureRecordIDString("notebook", notebookID)
	return db.RepoRelate(ctx, src, "reference", nb, nil)
}

// GetEmbeddedChunksCount returns the count of embedding chunks for a source
func GetEmbeddedChunksCount(ctx context.Context, sourceID string) (int, error) {
	recordID := db.EnsureRecordIDString("source", sourceID)
	type CountResult struct {
		Chunks int `json:"chunks"`
	}
	query := "SELECT count() as chunks FROM source_embedding WHERE source = $id GROUP ALL;"
	res, err := db.RepoQuery[[]CountResult](ctx, query, map[string]any{"id": recordID})
	if err != nil {
		return 0, err
	}
	if res == nil || len(*res) == 0 {
		return 0, nil
	}
	return (*res)[0].Chunks, nil
}

// GetSourceInsights retrieves all insights for a source
func GetSourceInsights(ctx context.Context, sourceID string) ([]SourceInsight, error) {
	recordID := db.EnsureRecordIDString("source", sourceID)
	results, err := db.RepoQuery[[]SourceInsight](ctx, "SELECT * FROM source_insight WHERE source = $id ORDER BY created ASC;", map[string]any{"id": recordID})
	if err != nil {
		return nil, err
	}
	if results == nil {
		return []SourceInsight{}, nil
	}
	return *results, nil
}

// GetSourceInsight retrieves a specific insight by ID
func GetSourceInsight(ctx context.Context, id string) (*SourceInsight, error) {
	recordID := db.EnsureRecordID("source_insight", id)
	results, err := db.RepoQuery[SourceInsight](ctx, "SELECT * FROM ONLY $id;", map[string]any{"id": recordID})
	if err != nil {
		return nil, err
	}
	return results, nil
}

// DeleteSourceInsight deletes an insight by ID
func DeleteSourceInsight(ctx context.Context, id string) error {
	recordID := db.EnsureRecordID("source_insight", id)
	return db.RepoDelete(ctx, recordID.String())
}

// CreateSource creates a source record, links it to notebooks, and submits the background job
func CreateSource(ctx context.Context, title, sourceType string, asset *Asset, content string, notebookIDs []string, transformations []string, embed, deleteSource, asyncProcessing bool) (*Source, string, error) {
	if title == "" {
		if sourceType == "link" && asset != nil {
			title = asset.URL
		} else if sourceType == "upload" && asset != nil {
			// strip directory components for filename
			parts := strings.Split(asset.FilePath, "/")
			title = parts[len(parts)-1]
		} else {
			title = "Untitled Source"
		}
	}

	// Create source record
	data := map[string]any{
		"title":  title,
		"topics": []string{},
	}
	if asset != nil {
		data["asset"] = map[string]any{
			"file_path": asset.FilePath,
			"url":       asset.URL,
		}
	}

	source, err := db.RepoCreate[Source](ctx, "source", data)
	if err != nil {
		return nil, "", err
	}

	sourceIDStr := source.ID.String()

	// Link source to notebooks
	for _, nbID := range notebookIDs {
		err = LinkSourceToNotebook(ctx, sourceIDStr, nbID)
		if err != nil {
			log.Printf("[Source] Warning: failed to link source %s to notebook %s: %v", sourceIDStr, nbID, err)
		}
	}

	// Prepare background command input
	contentState := map[string]any{}
	if sourceType == "link" && asset != nil {
		contentState["url"] = asset.URL
	} else if sourceType == "upload" && asset != nil {
		contentState["file_path"] = asset.FilePath
		contentState["delete_source"] = deleteSource
	} else if sourceType == "text" {
		contentState["content"] = content
	}

	now := time.Now().UTC()
	jobData := map[string]any{
		"app":            "open_notebook",
		"command":        "process_source",
		"status":         "pending",
		"created":        now,
		"updated":        now,
		"retry_attempts": 0,
		"input": map[string]any{
			"source_id":       sourceIDStr,
			"content_state":   contentState,
			"notebook_ids":    notebookIDs,
			"transformations": transformations,
			"embed":           embed,
		},
	}

	type CommandRecord struct {
		ID *models.RecordID `json:"id"`
	}

	res, err := db.RepoCreate[CommandRecord](ctx, "command", jobData)
	if err != nil {
		// Clean up source if command submission fails
		_ = db.RepoDelete(ctx, sourceIDStr)
		return nil, "", fmt.Errorf("failed to submit process_source command: %w", err)
	}

	commandID := res.ID.String()

	// Update source with command ID
	updateData := map[string]any{
		"command": res.ID,
	}
	source, err = db.RepoUpdate[Source](ctx, "source", sourceIDStr, updateData)
	if err != nil {
		log.Printf("[Source] Warning: failed to update source with command ID: %v", err)
	}

	return source, commandID, nil
}

// UpdateSource updates title and topics of a source
func UpdateSource(ctx context.Context, id string, title *string, topics []string) (*Source, error) {
	data := make(map[string]any)
	if title != nil {
		data["title"] = *title
	}
	if topics != nil {
		data["topics"] = topics
	}

	return db.RepoUpdate[Source](ctx, "source", id, data)
}

// DeleteSource deletes a source and links
func DeleteSource(ctx context.Context, id string) error {
	recordID := db.EnsureRecordID("source", id)

	// Delete reference relations
	_, _ = db.RepoQuery[any](ctx, "DELETE reference WHERE in = $id;", map[string]any{"id": recordID})

	// Delete source (event triggers downstream deletes of source_embedding and source_insight)
	return db.RepoDelete(ctx, recordID.String())
}

// SubmitRetryCommand schedules retry command for source
func SubmitRetryCommand(ctx context.Context, sourceID string) (string, error) {
	source, err := GetSource(ctx, sourceID)
	if err != nil {
		return "", err
	}

	if source.Command == nil {
		return "", errors.New("source does not have an associated command to retry")
	}

	// Fetch original command input to reuse
	origCommand, err := GetCommandJob(ctx, source.Command.String())
	if err != nil {
		return "", fmt.Errorf("failed to fetch original command: %w", err)
	}

	now := time.Now().UTC()
	jobData := map[string]any{
		"app":            "open_notebook",
		"command":        "process_source",
		"status":         "pending",
		"created":        now,
		"updated":        now,
		"retry_attempts": 0,
		"input":          origCommand.Input,
	}

	type CommandRecord struct {
		ID *models.RecordID `json:"id"`
	}

	res, err := db.RepoCreate[CommandRecord](ctx, "command", jobData)
	if err != nil {
		return "", err
	}

	commandID := res.ID.String()

	// Update source with new command ID
	updateData := map[string]any{
		"command": res.ID,
	}
	_, err = db.RepoUpdate[Source](ctx, "source", sourceID, updateData)
	if err != nil {
		log.Printf("[Source] Warning: failed to update source with command ID on retry: %v", err)
	}

	return commandID, nil
}

// ListSourceResult maps the query result with embedded flags
type ListSourceResult struct {
	ID            *models.RecordID `json:"id"`
	Asset         *Asset           `json:"asset"`
	Title         string           `json:"title"`
	Topics        []string         `json:"topics"`
	Command       any              `json:"command"`
	InsightsCount int              `json:"insights_count"`
	Embedded      bool             `json:"embedded"`
	Created       time.Time        `json:"created"`
	Updated       time.Time        `json:"updated"`
}

// ListSources returns paginated list of sources optionally filtered by notebook
func ListSources(ctx context.Context, notebookID string, limit, offset int, sortBy, sortOrder string) ([]ListSourceResult, error) {
	if limit <= 0 {
		limit = 50
	}
	if sortBy != "created" && sortBy != "updated" {
		sortBy = "updated"
	}
	if strings.ToLower(sortOrder) != "asc" {
		sortOrder = "DESC"
	} else {
		sortOrder = "ASC"
	}

	orderClause := fmt.Sprintf("ORDER BY %s %s", sortBy, sortOrder)

	var query string
	var vars map[string]any

	if notebookID != "" {
		nb := db.EnsureRecordIDString("notebook", notebookID)
		query = fmt.Sprintf(`
			SELECT id, asset, created, title, updated, topics, command,
			(SELECT VALUE count() FROM source_insight WHERE source = $parent.id GROUP ALL)[0].count OR 0 AS insights_count,
			(SELECT VALUE id FROM source_embedding WHERE source = $parent.id LIMIT 1) != [] AS embedded
			FROM (select value in from reference where out=$notebook_id)
			%s
			LIMIT $limit START $offset
			FETCH command;
		`, orderClause)
		vars = map[string]any{
			"notebook_id": nb,
			"limit":       limit,
			"offset":      offset,
		}
	} else {
		query = fmt.Sprintf(`
			SELECT id, asset, created, title, updated, topics, command,
			(SELECT VALUE count() FROM source_insight WHERE source = $parent.id GROUP ALL)[0].count OR 0 AS insights_count,
			(SELECT VALUE id FROM source_embedding WHERE source = $parent.id LIMIT 1) != [] AS embedded
			FROM source
			%s
			LIMIT $limit START $offset
			FETCH command;
		`, orderClause)
		vars = map[string]any{
			"limit":  limit,
			"offset": offset,
		}
	}

	results, err := db.RepoQuery[[]ListSourceResult](ctx, query, vars)
	if err != nil {
		return nil, err
	}
	if results == nil {
		return []ListSourceResult{}, nil
	}
	return *results, nil
}

// SaveInsightAsNote saves a source insight as a note and optionally links it to a notebook
func SaveInsightAsNote(ctx context.Context, insightID string, notebookID string) (*Note, error) {
	insight, err := GetSourceInsight(ctx, insightID)
	if err != nil {
		return nil, err
	}

	sourceTitle := "Unknown"
	source, err := GetSource(ctx, insight.Source.String())
	if err == nil && source != nil {
		sourceTitle = source.Title
	}

	title := fmt.Sprintf("%s from source %s", insight.InsightType, sourceTitle)

	// Create note using domain.CreateNote
	note, _, err := CreateNote(ctx, title, insight.Content, "ai", notebookID)
	return note, err
}

// GetContext returns context representation of source
func (s *Source) GetContext(ctx context.Context, contextSize string) (map[string]any, error) {
	insights, err := GetSourceInsights(ctx, s.ID.String())
	if err != nil {
		return nil, err
	}

	insightsData := make([]map[string]any, len(insights))
	for i, ins := range insights {
		insightsData[i] = map[string]any{
			"id":           ins.ID.String(),
			"source":       ins.Source.String(),
			"insight_type": ins.InsightType,
			"content":      ins.Content,
			"created":      ins.Created,
			"updated":      ins.Updated,
		}
	}

	res := map[string]any{
		"id":       s.ID.String(),
		"title":    s.Title,
		"insights": insightsData,
	}
	if contextSize == "long" {
		res["full_text"] = s.FullText
	}
	return res, nil
}

// GetNotebookSources retrieves all sources linked to a notebook
func GetNotebookSources(ctx context.Context, notebookID string) ([]Source, error) {
	recordID := db.EnsureRecordIDString("notebook", notebookID)

	type ReferenceLink struct {
		Source Source `json:"source"`
	}

	query := `
		SELECT * omit source.full_text from (
			SELECT in as source FROM reference WHERE out = $notebook_id
			FETCH source
		) ORDER BY source.updated DESC;
	`
	links, err := db.RepoQuery[[]ReferenceLink](ctx, query, map[string]any{"notebook_id": recordID})
	if err != nil {
		return nil, fmt.Errorf("failed to list notebook sources: %w", err)
	}
	if links == nil {
		return []Source{}, nil
	}
	sources := make([]Source, len(*links))
	for i, l := range *links {
		sources[i] = l.Source
	}
	return sources, nil
}

