package surrealdb

import (
	"context"
	"fmt"

	"go-notebook/internal/db"
	"go-notebook/internal/db/repository"
)

type NoteRepo struct{}

func (r *NoteRepo) Get(ctx context.Context, id string) (*repository.NoteRecord, error) {
	recordID := db.EnsureRecordIDString("notes", id)
	query := fmt.Sprintf("SELECT * FROM %s;", recordID)
	res, err := db.RepoQuery[[]repository.NoteRecord](ctx, query, nil)
	if err != nil || res == nil || len(*res) == 0 {
		return nil, fmt.Errorf("note not found: %w", err)
	}
	return &(*res)[0], nil
}

func (r *NoteRepo) List(ctx context.Context) ([]repository.NoteRecord, error) {
	res, err := db.RepoQuery[[]repository.NoteRecord](ctx, "SELECT * FROM notes ORDER BY updated DESC;", nil)
	if err != nil || res == nil {
		return []repository.NoteRecord{}, nil
	}
	return *res, nil
}

func (r *NoteRepo) Create(ctx context.Context, note *repository.NoteRecord) (*repository.NoteRecord, error) {
	data := map[string]any{
		"title":   note.Title,
		"content": note.Content,
		"folder":  note.Folder,
		"tags":    note.Tags,
	}
	if note.ID != "" {
		return db.RepoCreateWithID[repository.NoteRecord](ctx, "notes", note.ID, data)
	}
	return db.RepoCreate[repository.NoteRecord](ctx, "notes", data)
}

func (r *NoteRepo) Update(ctx context.Context, id string, note *repository.NoteRecord) (*repository.NoteRecord, error) {
	data := map[string]any{
		"title":   note.Title,
		"content": note.Content,
		"folder":  note.Folder,
		"tags":    note.Tags,
	}
	return db.RepoUpdate[repository.NoteRecord](ctx, "notes", id, data)
}

func (r *NoteRepo) Delete(ctx context.Context, id string) error {
	recordID := db.EnsureRecordIDString("notes", id)
	return db.RepoDelete(ctx, recordID)
}

type DocumentRepo struct{}

func (r *DocumentRepo) Get(ctx context.Context, id string) (*repository.DocumentRecord, error) {
	recordID := db.EnsureRecordIDString("documents", id)
	query := fmt.Sprintf("SELECT * FROM %s;", recordID)
	res, err := db.RepoQuery[[]repository.DocumentRecord](ctx, query, nil)
	if err != nil || res == nil || len(*res) == 0 {
		return nil, fmt.Errorf("document not found: %w", err)
	}
	return &(*res)[0], nil
}

func (r *DocumentRepo) List(ctx context.Context) ([]repository.DocumentRecord, error) {
	res, err := db.RepoQuery[[]repository.DocumentRecord](ctx, "SELECT * FROM documents ORDER BY updated DESC;", nil)
	if err != nil || res == nil {
		return []repository.DocumentRecord{}, nil
	}
	return *res, nil
}

func (r *DocumentRepo) Create(ctx context.Context, doc *repository.DocumentRecord) (*repository.DocumentRecord, error) {
	data := map[string]any{
		"title":       doc.Title,
		"source_type": doc.SourceType,
		"content":     doc.Content,
		"metadata":    doc.Metadata,
	}
	if doc.ID != "" {
		return db.RepoCreateWithID[repository.DocumentRecord](ctx, "documents", doc.ID, data)
	}
	return db.RepoCreate[repository.DocumentRecord](ctx, "documents", data)
}

func (r *DocumentRepo) Delete(ctx context.Context, id string) error {
	recordID := db.EnsureRecordIDString("documents", id)
	return db.RepoDelete(ctx, recordID)
}

type VectorRepo struct{}

func (r *VectorRepo) Save(ctx context.Context, vec *repository.VectorRecord) error {
	data := map[string]any{
		"source_id":   vec.SourceID,
		"chunk_index": vec.ChunkIndex,
		"text":        vec.Text,
		"vector":      vec.Vector,
	}
	id := vec.ID
	if id == "" {
		id = fmt.Sprintf("%s_%d", vec.SourceID, vec.ChunkIndex)
	}
	_, err := db.RepoUpsert[repository.VectorRecord](ctx, "vectors", id, data, true)
	return err
}

func (r *VectorRepo) Search(ctx context.Context, queryVector []float32, limit int) ([]repository.VectorRecord, error) {
	query := fmt.Sprintf("SELECT *, vector::similarity::cosine(vector, $queryVector) AS score FROM vectors ORDER BY score DESC LIMIT %d;", limit)
	res, err := db.RepoQuery[[]repository.VectorRecord](ctx, query, map[string]any{"queryVector": queryVector})
	if err != nil || res == nil {
		return []repository.VectorRecord{}, nil
	}
	return *res, nil
}

func (r *VectorRepo) DeleteBySource(ctx context.Context, sourceID string) error {
	query := "DELETE FROM vectors WHERE source_id = $sourceID;"
	_, err := db.RepoQuery[any](ctx, query, map[string]any{"sourceID": sourceID})
	return err
}

type GraphRepo struct{}

func (r *GraphRepo) SaveEntity(ctx context.Context, entity *repository.EntityRecord) error {
	data := map[string]any{
		"name":        entity.Name,
		"type":        entity.Type,
		"description": entity.Description,
		"source_ids":  entity.SourceIDs,
	}
	id := entity.ID
	if id == "" {
		id = entity.Name
	}
	_, err := db.RepoUpsert[repository.EntityRecord](ctx, "entities", id, data, true)
	return err
}

func (r *GraphRepo) SaveRelation(ctx context.Context, relation *repository.RelationRecord) error {
	data := map[string]any{
		"type":        relation.Type,
		"description": relation.Description,
		"weight":      relation.Weight,
	}
	sourceID := db.EnsureRecordIDString("entities", relation.Source)
	targetID := db.EnsureRecordIDString("entities", relation.Target)
	return db.RepoRelate(ctx, sourceID, relation.Type, targetID, data)
}

func (r *GraphRepo) GetEntities(ctx context.Context) ([]repository.EntityRecord, error) {
	res, err := db.RepoQuery[[]repository.EntityRecord](ctx, "SELECT * FROM entities;", nil)
	if err != nil || res == nil {
		return []repository.EntityRecord{}, nil
	}
	return *res, nil
}

func (r *GraphRepo) GetRelations(ctx context.Context) ([]repository.RelationRecord, error) {
	res, err := db.RepoQuery[[]repository.RelationRecord](ctx, "SELECT * FROM relations;", nil)
	if err != nil || res == nil {
		return []repository.RelationRecord{}, nil
	}
	return *res, nil
}

type SettingsRepo struct{}

func (r *SettingsRepo) Get(ctx context.Context, key string) (string, error) {
	recordID := db.EnsureRecordIDString("settings", key)
	query := fmt.Sprintf("SELECT value FROM %s;", recordID)
	type settingVal struct {
		Value string `json:"value"`
	}
	res, err := db.RepoQuery[[]settingVal](ctx, query, nil)
	if err != nil || res == nil || len(*res) == 0 {
		return "", fmt.Errorf("setting not found: %w", err)
	}
	return (*res)[0].Value, nil
}

func (r *SettingsRepo) Set(ctx context.Context, key string, value string) error {
	data := map[string]any{"value": value}
	_, err := db.RepoUpsert[any](ctx, "settings", key, data, true)
	return err
}

type SurrealFactory struct{}

func NewSurrealFactory(ctx context.Context) (*SurrealFactory, error) {
	if err := db.Init(ctx); err != nil {
		return nil, err
	}
	return &SurrealFactory{}, nil
}

func (f *SurrealFactory) Notes() repository.NoteRepository         { return &NoteRepo{} }
func (f *SurrealFactory) Documents() repository.DocumentRepository { return &DocumentRepo{} }
func (f *SurrealFactory) Vectors() repository.VectorRepository     { return &VectorRepo{} }
func (f *SurrealFactory) Graph() repository.GraphRepository       { return &GraphRepo{} }
func (f *SurrealFactory) Settings() repository.SettingsRepository { return &SettingsRepo{} }
func (f *SurrealFactory) Close(ctx context.Context) error         { db.Close(ctx); return nil }
