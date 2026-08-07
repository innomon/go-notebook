package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"go-notebook/internal/db/repository"

	_ "modernc.org/sqlite"
)

type NoteRepo struct {
	db *sql.DB
}

func (r *NoteRepo) Get(ctx context.Context, id string) (*repository.NoteRecord, error) {
	row := r.db.QueryRowContext(ctx, "SELECT id, title, content, folder, tags, created, updated FROM notes WHERE id = ?", id)
	var n repository.NoteRecord
	var folder sql.NullString
	var tagsStr sql.NullString
	var createdStr, updatedStr string

	if err := row.Scan(&n.ID, &n.Title, &n.Content, &folder, &tagsStr, &createdStr, &updatedStr); err != nil {
		return nil, err
	}
	n.Folder = folder.String
	if tagsStr.Valid && tagsStr.String != "" {
		_ = json.Unmarshal([]byte(tagsStr.String), &n.Tags)
	}
	n.Created, _ = time.Parse(time.RFC3339, createdStr)
	n.Updated, _ = time.Parse(time.RFC3339, updatedStr)
	return &n, nil
}

func (r *NoteRepo) List(ctx context.Context) ([]repository.NoteRecord, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT id, title, content, folder, tags, created, updated FROM notes ORDER BY updated DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []repository.NoteRecord
	for rows.Next() {
		var n repository.NoteRecord
		var folder sql.NullString
		var tagsStr sql.NullString
		var createdStr, updatedStr string

		if err := rows.Scan(&n.ID, &n.Title, &n.Content, &folder, &tagsStr, &createdStr, &updatedStr); err != nil {
			return nil, err
		}
		n.Folder = folder.String
		if tagsStr.Valid && tagsStr.String != "" {
			_ = json.Unmarshal([]byte(tagsStr.String), &n.Tags)
		}
		n.Created, _ = time.Parse(time.RFC3339, createdStr)
		n.Updated, _ = time.Parse(time.RFC3339, updatedStr)
		list = append(list, n)
	}
	return list, nil
}

func (r *NoteRepo) Create(ctx context.Context, note *repository.NoteRecord) (*repository.NoteRecord, error) {
	if note.ID == "" {
		note.ID = fmt.Sprintf("note_%d", time.Now().UnixNano())
	}
	now := time.Now().UTC()
	note.Created = now
	note.Updated = now

	tagsBytes, _ := json.Marshal(note.Tags)
	_, err := r.db.ExecContext(ctx,
		"INSERT INTO notes (id, title, content, folder, tags, created, updated) VALUES (?, ?, ?, ?, ?, ?, ?)",
		note.ID, note.Title, note.Content, note.Folder, string(tagsBytes), note.Created.Format(time.RFC3339), note.Updated.Format(time.RFC3339),
	)
	if err != nil {
		return nil, err
	}
	return note, nil
}

func (r *NoteRepo) Update(ctx context.Context, id string, note *repository.NoteRecord) (*repository.NoteRecord, error) {
	note.ID = id
	note.Updated = time.Now().UTC()
	tagsBytes, _ := json.Marshal(note.Tags)

	res, err := r.db.ExecContext(ctx,
		"UPDATE notes SET title = ?, content = ?, folder = ?, tags = ?, updated = ? WHERE id = ?",
		note.Title, note.Content, note.Folder, string(tagsBytes), note.Updated.Format(time.RFC3339), id,
	)
	if err != nil {
		return nil, err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return nil, sql.ErrNoRows
	}
	return note, nil
}

func (r *NoteRepo) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM notes WHERE id = ?", id)
	return err
}

type DocumentRepo struct {
	db *sql.DB
}

func (r *DocumentRepo) Get(ctx context.Context, id string) (*repository.DocumentRecord, error) {
	row := r.db.QueryRowContext(ctx, "SELECT id, title, source_type, content, metadata, created, updated FROM documents WHERE id = ?", id)
	var d repository.DocumentRecord
	var metaStr sql.NullString
	var createdStr, updatedStr string

	if err := row.Scan(&d.ID, &d.Title, &d.SourceType, &d.Content, &metaStr, &createdStr, &updatedStr); err != nil {
		return nil, err
	}
	if metaStr.Valid && metaStr.String != "" {
		_ = json.Unmarshal([]byte(metaStr.String), &d.Metadata)
	}
	d.Created, _ = time.Parse(time.RFC3339, createdStr)
	d.Updated, _ = time.Parse(time.RFC3339, updatedStr)
	return &d, nil
}

func (r *DocumentRepo) List(ctx context.Context) ([]repository.DocumentRecord, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT id, title, source_type, content, metadata, created, updated FROM documents ORDER BY updated DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []repository.DocumentRecord
	for rows.Next() {
		var d repository.DocumentRecord
		var metaStr sql.NullString
		var createdStr, updatedStr string

		if err := rows.Scan(&d.ID, &d.Title, &d.SourceType, &d.Content, &metaStr, &createdStr, &updatedStr); err != nil {
			return nil, err
		}
		if metaStr.Valid && metaStr.String != "" {
			_ = json.Unmarshal([]byte(metaStr.String), &d.Metadata)
		}
		d.Created, _ = time.Parse(time.RFC3339, createdStr)
		d.Updated, _ = time.Parse(time.RFC3339, updatedStr)
		list = append(list, d)
	}
	return list, nil
}

func (r *DocumentRepo) Create(ctx context.Context, doc *repository.DocumentRecord) (*repository.DocumentRecord, error) {
	if doc.ID == "" {
		doc.ID = fmt.Sprintf("doc_%d", time.Now().UnixNano())
	}
	now := time.Now().UTC()
	doc.Created = now
	doc.Updated = now

	metaBytes, _ := json.Marshal(doc.Metadata)
	_, err := r.db.ExecContext(ctx,
		"INSERT INTO documents (id, title, source_type, content, metadata, created, updated) VALUES (?, ?, ?, ?, ?, ?, ?)",
		doc.ID, doc.Title, doc.SourceType, doc.Content, string(metaBytes), doc.Created.Format(time.RFC3339), doc.Updated.Format(time.RFC3339),
	)
	if err != nil {
		return nil, err
	}
	return doc, nil
}

func (r *DocumentRepo) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM documents WHERE id = ?", id)
	return err
}

type VectorRepo struct {
	db *sql.DB
}

func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

func (r *VectorRepo) Save(ctx context.Context, vec *repository.VectorRecord) error {
	if vec.ID == "" {
		vec.ID = fmt.Sprintf("vec_%s_%d", vec.SourceID, vec.ChunkIndex)
	}
	vecBytes, _ := json.Marshal(vec.Vector)
	_, err := r.db.ExecContext(ctx,
		"INSERT INTO vectors (id, source_id, chunk_index, text, vector) VALUES (?, ?, ?, ?, ?) ON CONFLICT(id) DO UPDATE SET text=excluded.text, vector=excluded.vector",
		vec.ID, vec.SourceID, vec.ChunkIndex, vec.Text, string(vecBytes),
	)
	return err
}

func (r *VectorRepo) Search(ctx context.Context, queryVector []float32, limit int) ([]repository.VectorRecord, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT id, source_id, chunk_index, text, vector FROM vectors")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type scoredVector struct {
		record repository.VectorRecord
		score  float64
	}

	var scored []scoredVector
	for rows.Next() {
		var v repository.VectorRecord
		var vecStr string
		if err := rows.Scan(&v.ID, &v.SourceID, &v.ChunkIndex, &v.Text, &vecStr); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(vecStr), &v.Vector)
		sim := cosineSimilarity(queryVector, v.Vector)
		v.Distance = 1.0 - sim
		scored = append(scored, scoredVector{record: v, score: sim})
	}

	// Sort by highest similarity
	for i := 0; i < len(scored); i++ {
		for j := i + 1; j < len(scored); j++ {
			if scored[j].score > scored[i].score {
				scored[i], scored[j] = scored[j], scored[i]
			}
		}
	}

	var res []repository.VectorRecord
	for i := 0; i < len(scored) && i < limit; i++ {
		res = append(res, scored[i].record)
	}
	return res, nil
}

func (r *VectorRepo) DeleteBySource(ctx context.Context, sourceID string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM vectors WHERE source_id = ?", sourceID)
	return err
}

type GraphRepo struct {
	db *sql.DB
}

func (r *GraphRepo) SaveEntity(ctx context.Context, entity *repository.EntityRecord) error {
	if entity.ID == "" {
		entity.ID = fmt.Sprintf("ent_%s", strings.ToLower(entity.Name))
	}
	sourcesBytes, _ := json.Marshal(entity.SourceIDs)
	_, err := r.db.ExecContext(ctx,
		"INSERT INTO entities (id, name, type, description, source_ids) VALUES (?, ?, ?, ?, ?) ON CONFLICT(id) DO UPDATE SET name=excluded.name, type=excluded.type, description=excluded.description, source_ids=excluded.source_ids",
		entity.ID, entity.Name, entity.Type, entity.Description, string(sourcesBytes),
	)
	return err
}

func (r *GraphRepo) SaveRelation(ctx context.Context, relation *repository.RelationRecord) error {
	if relation.ID == "" {
		relation.ID = fmt.Sprintf("rel_%s_%s_%s", relation.Source, relation.Type, relation.Target)
	}
	_, err := r.db.ExecContext(ctx,
		"INSERT INTO relations (id, source, target, type, description, weight) VALUES (?, ?, ?, ?, ?, ?) ON CONFLICT(id) DO UPDATE SET description=excluded.description, weight=excluded.weight",
		relation.ID, relation.Source, relation.Target, relation.Type, relation.Description, relation.Weight,
	)
	return err
}

func (r *GraphRepo) GetEntities(ctx context.Context) ([]repository.EntityRecord, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT id, name, type, description, source_ids FROM entities")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []repository.EntityRecord
	for rows.Next() {
		var e repository.EntityRecord
		var sourcesStr sql.NullString
		if err := rows.Scan(&e.ID, &e.Name, &e.Type, &e.Description, &sourcesStr); err != nil {
			return nil, err
		}
		if sourcesStr.Valid && sourcesStr.String != "" {
			_ = json.Unmarshal([]byte(sourcesStr.String), &e.SourceIDs)
		}
		list = append(list, e)
	}
	return list, nil
}

func (r *GraphRepo) GetRelations(ctx context.Context) ([]repository.RelationRecord, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT id, source, target, type, description, weight FROM relations")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []repository.RelationRecord
	for rows.Next() {
		var rel repository.RelationRecord
		if err := rows.Scan(&rel.ID, &rel.Source, &rel.Target, &rel.Type, &rel.Description, &rel.Weight); err != nil {
			return nil, err
		}
		list = append(list, rel)
	}
	return list, nil
}

type SettingsRepo struct {
	db *sql.DB
}

func (r *SettingsRepo) Get(ctx context.Context, key string) (string, error) {
	var val string
	err := r.db.QueryRowContext(ctx, "SELECT value FROM settings WHERE key = ?", key).Scan(&val)
	if err != nil {
		return "", err
	}
	return val, nil
}

func (r *SettingsRepo) Set(ctx context.Context, key string, value string) error {
	_, err := r.db.ExecContext(ctx, "INSERT INTO settings (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value=excluded.value", key, value)
	return err
}

type SQLiteFactory struct {
	db *sql.DB
}

func NewSQLiteFactory(dbPath string) (*SQLiteFactory, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database: %w", err)
	}

	migrator := NewMigrator(db)
	if err := migrator.Up(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to run sqlite migrations: %w", err)
	}

	return &SQLiteFactory{db: db}, nil
}

func (f *SQLiteFactory) Notes() repository.NoteRepository         { return &NoteRepo{db: f.db} }
func (f *SQLiteFactory) Documents() repository.DocumentRepository { return &DocumentRepo{db: f.db} }
func (f *SQLiteFactory) Vectors() repository.VectorRepository     { return &VectorRepo{db: f.db} }
func (f *SQLiteFactory) Graph() repository.GraphRepository       { return &GraphRepo{db: f.db} }
func (f *SQLiteFactory) Settings() repository.SettingsRepository { return &SettingsRepo{db: f.db} }
func (f *SQLiteFactory) Close(ctx context.Context) error         { return f.db.Close() }
