package domain

import (
	"context"
	"fmt"
	"go-notebook/internal/db"
	"time"

	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// RAGEntity represents a node in the knowledge graph
type RAGEntity struct {
	ID       *models.RecordID    `json:"id,omitempty"`
	Name     string              `json:"name"`
	Count    int                 `json:"count"`
	Sources  []*models.RecordID  `json:"sources,omitempty"`
	Notebook *models.RecordID    `json:"notebook"`
	Created  time.Time           `json:"created,omitempty"`
}

// RAGCommunity represents a thematic cluster of entities
type RAGCommunity struct {
	ID          *models.RecordID `json:"id,omitempty"`
	CommunityID int              `json:"community_id"`
	Notebook    *models.RecordID `json:"notebook"`
	Summary     string           `json:"summary"`
	Entities    []string         `json:"entities"`
	Embedding   *[]float64       `json:"embedding,omitempty"`
	Created     time.Time        `json:"created,omitempty"`
}

// EntityConnection represents a co-occurrence link between entities
type EntityConnection struct {
	Source string  `json:"source"`
	Target string  `json:"target"`
	Weight float64 `json:"weight"`
}

// CommunityInfo represents serialized community metadata for the REST API
type CommunityInfo struct {
	ID          int      `json:"id"`
	Summary     string   `json:"summary"`
	TopEntities []string `json:"top_entities"`
	Size        int      `json:"size"`
	NumChunks   int      `json:"num_chunks"`
}

// GraphDataResponse represents the complete graph visualization payload
type GraphDataResponse struct {
	Connections []EntityConnection `json:"connections"`
	Communities []CommunityInfo    `json:"communities"`
	TopNodes    []string           `json:"top_nodes"`
}

// ClearNotebookGraph deletes all entities, relations, and communities for a given notebook
func ClearNotebookGraph(ctx context.Context, notebookID string) error {
	notebookRecordID := db.EnsureRecordID("notebook", notebookID)

	// Delete communities
	_, err := db.RepoQuery[any](ctx, "DELETE RAGCommunity WHERE notebook = $nb;", map[string]any{"nb": notebookRecordID})
	if err != nil {
		return fmt.Errorf("failed to delete community records: %w", err)
	}

	// Delete relationship edges
	_, err = db.RepoQuery[any](ctx, "DELETE co_occurs WHERE notebook = $nb;", map[string]any{"nb": notebookRecordID})
	if err != nil {
		return fmt.Errorf("failed to delete relationship edges: %w", err)
	}

	// Delete entities
	_, err = db.RepoQuery[any](ctx, "DELETE RAGEntity WHERE notebook = $nb;", map[string]any{"nb": notebookRecordID})
	if err != nil {
		return fmt.Errorf("failed to delete entities: %w", err)
	}

	return nil
}

// CreateOrUpdateEntity creates an entity or appends a source to its list
func CreateOrUpdateEntity(ctx context.Context, notebookID, sourceID, name string) (*RAGEntity, error) {
	notebookRecordID := db.EnsureRecordID("notebook", notebookID)
	sourceRecordID := db.EnsureRecordID("source", sourceID)

	query := `
		LET $existing = (SELECT id, sources FROM RAGEntity WHERE notebook = $nb AND name = $name)[0];
		IF $existing.id != NONE {
			UPDATE $existing.id SET 
				sources = array::distinct(array::add(sources, $source)),
				count = count(array::distinct(array::add(sources, $source)));
		} ELSE {
			CREATE RAGEntity CONTENT {
				notebook: $nb,
				name: $name,
				sources: [$source],
				count: 1
			};
		};
	`
	_, err := db.RepoQuery[[]RAGEntity](ctx, query, map[string]any{
		"nb":     notebookRecordID,
		"source": sourceRecordID,
		"name":   name,
	})
	if err != nil {
		return nil, err
	}

	res, err := db.RepoQuery[[]RAGEntity](ctx, "SELECT * FROM RAGEntity WHERE notebook = $nb AND name = $name LIMIT 1;", map[string]any{
		"nb":   notebookRecordID,
		"name": name,
	})
	if err != nil || len(*res) == 0 {
		return nil, fmt.Errorf("failed to retrieve upserted entity %q: %v", name, err)
	}
	return &(*res)[0], nil
}

// RelateEntities creates or appends a source to a co-occurs relation between two entities
func RelateEntities(ctx context.Context, notebookID, sourceID string, entA, entB *RAGEntity) error {
	notebookRecordID := db.EnsureRecordID("notebook", notebookID)
	sourceRecordID := db.EnsureRecordID("source", sourceID)

	fromID := entA.ID.String()
	toID := entB.ID.String()
	if entA.Name > entB.Name {
		fromID, toID = toID, fromID
	}

	query := `
		LET $existing = (SELECT id, sources FROM co_occurs WHERE in = $from AND out = $to)[0];
		IF $existing.id != NONE {
			UPDATE $existing.id SET 
				sources = array::distinct(array::add(sources, $source)),
				weight = count(array::distinct(array::add(sources, $source)));
		} ELSE {
			RELATE $from->co_occurs->$to CONTENT {
				sources: [$source],
				weight: 1,
				notebook: $nb
			};
		};
	`
	_, err := db.RepoQuery[any](ctx, query, map[string]any{
		"from":   db.EnsureRecordID("RAGEntity", fromID),
		"to":     db.EnsureRecordID("RAGEntity", toID),
		"nb":     notebookRecordID,
		"source": sourceRecordID,
	})
	return err
}

// GetNotebookGraphData retrieves the top N most connected entities and their co-occurrence edges
func GetNotebookGraphData(ctx context.Context, notebookID string, maxNodes int) (*GraphDataResponse, error) {
	notebookRecordID := db.EnsureRecordID("notebook", notebookID)

	// Fetch top entities by count
	entitiesQuery := `
		SELECT name, count FROM RAGEntity 
		WHERE notebook = $nb 
		ORDER BY count DESC 
		LIMIT $limit;
	`
	entities, err := db.RepoQuery[[]RAGEntity](ctx, entitiesQuery, map[string]any{
		"nb":    notebookRecordID,
		"limit": maxNodes,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch top entities: %w", err)
	}

	topNodes := make([]string, 0, len(*entities))
	entityNames := make([]string, 0, len(*entities))
	for _, ent := range *entities {
		topNodes = append(topNodes, ent.Name)
		entityNames = append(entityNames, ent.Name)
	}

	// Fetch relations between these top entities
	relationsQuery := `
		SELECT in.name AS source, out.name AS target, weight 
		FROM co_occurs 
		WHERE notebook = $nb 
		  AND in.name INSIDE $names 
		  AND out.name INSIDE $names;
	`
	type relationResult struct {
		Source string  `json:"source"`
		Target string  `json:"target"`
		Weight float64 `json:"weight"`
	}
	relations, err := db.RepoQuery[[]relationResult](ctx, relationsQuery, map[string]any{
		"nb":    notebookRecordID,
		"names": entityNames,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch co-occurrence relationships: %w", err)
	}

	connections := make([]EntityConnection, 0, len(*relations))
	for _, rel := range *relations {
		connections = append(connections, EntityConnection{
			Source: rel.Source,
			Target: rel.Target,
			Weight: rel.Weight,
		})
	}

	// Fetch community summaries
	communitiesQuery := `
		SELECT community_id, summary, entities FROM RAGCommunity 
		WHERE notebook = $nb;
	`
	communities, err := db.RepoQuery[[]RAGCommunity](ctx, communitiesQuery, map[string]any{
		"nb": notebookRecordID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch community summaries: %w", err)
	}

	commInfos := make([]CommunityInfo, 0, len(*communities))
	for _, comm := range *communities {
		commInfos = append(commInfos, CommunityInfo{
			ID:          comm.CommunityID,
			Summary:     comm.Summary,
			TopEntities: comm.Entities,
			Size:        len(comm.Entities),
			NumChunks:   0, // Chunk counting can be expanded if linked to source_embeddings
		})
	}

	return &GraphDataResponse{
		Connections: connections,
		Communities: commInfos,
		TopNodes:    topNodes,
	}, nil
}
