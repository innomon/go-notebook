package repository

import (
	"context"
	"time"
)

// NoteRecord represents domain storage for notes
type NoteRecord struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	Folder    string    `json:"folder,omitempty"`
	Tags      []string  `json:"tags,omitempty"`
	Created   time.Time `json:"created"`
	Updated   time.Time `json:"updated"`
}

// DocumentRecord represents domain storage for ingested documents/sources
type DocumentRecord struct {
	ID          string         `json:"id"`
	Title       string         `json:"title"`
	SourceType  string         `json:"source_type"`
	Content     string         `json:"content"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	Created     time.Time      `json:"created"`
	Updated     time.Time      `json:"updated"`
}

// VectorRecord represents an embedding vector stored for similarity retrieval
type VectorRecord struct {
	ID         string    `json:"id"`
	SourceID   string    `json:"source_id"`
	ChunkIndex int       `json:"chunk_index"`
	Text       string    `json:"text"`
	Vector     []float32 `json:"vector"`
	Distance   float64   `json:"distance,omitempty"`
}

// EntityRecord represents a GraphRAG extracted entity node
type EntityRecord struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Type        string    `json:"type"`
	Description string    `json:"description"`
	SourceIDs   []string  `json:"source_ids,omitempty"`
}

// RelationRecord represents a GraphRAG relationship edge
type RelationRecord struct {
	ID          string  `json:"id"`
	Source      string  `json:"source"`
	Target      string  `json:"target"`
	Type        string  `json:"type"`
	Description string  `json:"description"`
	Weight      float64 `json:"weight"`
}

// NoteRepository defines storage operations for notes
type NoteRepository interface {
	Get(ctx context.Context, id string) (*NoteRecord, error)
	List(ctx context.Context) ([]NoteRecord, error)
	Create(ctx context.Context, note *NoteRecord) (*NoteRecord, error)
	Update(ctx context.Context, id string, note *NoteRecord) (*NoteRecord, error)
	Delete(ctx context.Context, id string) error
}

// DocumentRepository defines storage operations for documents
type DocumentRepository interface {
	Get(ctx context.Context, id string) (*DocumentRecord, error)
	List(ctx context.Context) ([]DocumentRecord, error)
	Create(ctx context.Context, doc *DocumentRecord) (*DocumentRecord, error)
	Delete(ctx context.Context, id string) error
}

// VectorRepository defines storage operations for vector embeddings
type VectorRepository interface {
	Save(ctx context.Context, vec *VectorRecord) error
	Search(ctx context.Context, queryVector []float32, limit int) ([]VectorRecord, error)
	DeleteBySource(ctx context.Context, sourceID string) error
}

// GraphRepository defines storage operations for GraphRAG entities and relationships
type GraphRepository interface {
	SaveEntity(ctx context.Context, entity *EntityRecord) error
	SaveRelation(ctx context.Context, relation *RelationRecord) error
	GetEntities(ctx context.Context) ([]EntityRecord, error)
	GetRelations(ctx context.Context) ([]RelationRecord, error)
}

// SettingsRepository defines key-value storage for app settings
type SettingsRepository interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value string) error
}

// Factory interface for initializing repositories based on configured DB engine
type RepositoryFactory interface {
	Notes() NoteRepository
	Documents() DocumentRepository
	Vectors() VectorRepository
	Graph() GraphRepository
	Settings() SettingsRepository
	Close(ctx context.Context) error
}
