package graphrag

import (
	"context"
	"crypto/sha256"
	"fmt"
	"go-notebook/internal/ai"
	"go-notebook/internal/db"
	"go-notebook/internal/domain"
	"log"
	"sort"
	"strings"

	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// Pipeline orchestrates GraphRAG ingestion and query tasks
type Pipeline struct {
	chatClient  ai.AIClient
	embedClient ai.AIClient
}

// NewPipeline initializes a new GraphRAG pipeline using default models
func NewPipeline(ctx context.Context) (*Pipeline, error) {
	chatClient, err := ai.GetClientForDefaultModel(ctx, "chat")
	if err != nil {
		return nil, fmt.Errorf("failed to get default chat model: %w", err)
	}

	embedClient, err := ai.GetClientForDefaultModel(ctx, "embedding")
	if err != nil {
		// Log warning and proceed without embedding client (embeddings will be skipped)
		log.Printf("[GraphRAG] Warning: default embedding model not available: %v. Global search vector matches will be skipped.", err)
	}

	return &Pipeline{
		chatClient:  chatClient,
		embedClient: embedClient,
	}, nil
}

// BuildGraph extracts entities and builds communities for all sources in the notebook
func (p *Pipeline) BuildGraph(ctx context.Context, notebookID string) error {
	notebookRecordID := db.EnsureRecordID("notebook", notebookID)

	log.Printf("[GraphRAG] Starting stateful incremental graph build for notebook %s...", notebookID)

	// 1. Fetch all sources linked to the notebook
	sourcesQuery := "SELECT id, title, full_text, hash, last_graph_hash FROM source WHERE id IN (SELECT VALUE in FROM reference WHERE out = $nb);"
	sources, err := db.RepoQuery[[]domain.Source](ctx, sourcesQuery, map[string]any{"nb": notebookRecordID})
	if err != nil {
		return fmt.Errorf("failed to query sources: %w", err)
	}

	graphChanged := false

	// 2. Identify and clear lineage of orphaned sources (deleted/unlinked from notebook) in Go
	type sourcesResult struct {
		Sources []*models.RecordID `json:"sources"`
	}
	entSources, err := db.RepoQuery[[]sourcesResult](ctx, "SELECT sources FROM RAGEntity WHERE notebook = $nb;", map[string]any{"nb": notebookRecordID})
	if err != nil {
		return fmt.Errorf("failed to query entity sources: %w", err)
	}
	edgeSources, err := db.RepoQuery[[]sourcesResult](ctx, "SELECT sources FROM co_occurs WHERE notebook = $nb;", map[string]any{"nb": notebookRecordID})
	if err != nil {
		return fmt.Errorf("failed to query edge sources: %w", err)
	}

	indexedMap := make(map[string]*models.RecordID)
	if entSources != nil {
		for _, r := range *entSources {
			for _, src := range r.Sources {
				if src != nil {
					indexedMap[src.String()] = src
				}
			}
		}
	}
	if edgeSources != nil {
		for _, r := range *edgeSources {
			for _, src := range r.Sources {
				if src != nil {
					indexedMap[src.String()] = src
				}
			}
		}
	}

	activeMap := make(map[string]bool)
	for _, src := range *sources {
		if src.ID != nil {
			activeMap[src.ID.String()] = true
		}
	}

	for srcStr, srcRecID := range indexedMap {
		if !activeMap[srcStr] {
			log.Printf("[GraphRAG] Cleaning up lineage for deleted/orphaned source: %s", srcStr)
			if clearErr := domain.ClearSourceGraphLineage(ctx, notebookID, srcRecID.String()); clearErr != nil {
				log.Printf("[GraphRAG] Error cleaning up orphaned source lineage %s: %v", srcStr, clearErr)
			} else {
				graphChanged = true
			}
		}
	}

	if len(*sources) == 0 {
		log.Printf("[GraphRAG] No active sources found in notebook %s.", notebookID)
		if graphChanged {
			log.Println("[GraphRAG] Graph lineage cleared for deleted sources. Re-running community detection...")
			if err := RunCommunityDetection(ctx, p.chatClient, p.embedClient, notebookID); err != nil {
				return fmt.Errorf("failed community clustering: %w", err)
			}
		}
		return nil
	}

	// 3. Process each source document incrementally
	for _, src := range *sources {
		text := src.FullText
		if strings.TrimSpace(text) == "" {
			continue
		}

		// Compute current hash
		hashSum := sha256.Sum256([]byte(text))
		currentHash := fmt.Sprintf("%x", hashSum)

		// Compare current hash with last indexed graph hash
		if src.LastGraphHash != "" && src.LastGraphHash == currentHash {
			log.Printf("[GraphRAG] Skipping unchanged source %q (hash: %s)", src.Title, currentHash)
			continue
		}

		log.Printf("[GraphRAG] Processing new or modified source %q...", src.Title)
		graphChanged = true

		// Clear old lineage for this source first
		if src.LastGraphHash != "" {
			log.Printf("[GraphRAG] Clearing old lineage for source %s...", src.ID.String())
			if clearErr := domain.ClearSourceGraphLineage(ctx, notebookID, src.ID.String()); clearErr != nil {
				log.Printf("[GraphRAG] Error clearing old source lineage: %v", clearErr)
			}
		}

		blocks := ParseText(text)
		cleaned := CleanText(blocks)
		chunks := ChunkBlocks(cleaned, 500, 80, src.Title)

		for _, chunk := range chunks {
			err := IngestChunkGraph(ctx, p.chatClient, notebookID, src.ID.String(), chunk.Text)
			if err != nil {
				log.Printf("[GraphRAG] Error ingesting chunk graph: %v", err)
				// Continue processing other chunks rather than failing the whole pipeline
			}
		}

		// Update database with both hash (current document state) and last_graph_hash (indexed state)
		_, err = db.RepoUpdate[any](ctx, "source", src.ID.String(), map[string]any{
			"hash":            currentHash,
			"last_graph_hash": currentHash,
		})
		if err != nil {
			log.Printf("[GraphRAG] Error updating source hashes for %s: %v", src.ID.String(), err)
		}
	}

	// 4. Run community clustering and summarization only if the graph representation has changed
	if graphChanged {
		log.Println("[GraphRAG] Graph changed. Running community detection and generating summaries...")
		if err := RunCommunityDetection(ctx, p.chatClient, p.embedClient, notebookID); err != nil {
			return fmt.Errorf("failed community clustering: %w", err)
		}
	} else {
		log.Println("[GraphRAG] No graph changes detected. Skipping community detection.")
	}

	log.Printf("[GraphRAG] Graph build completed successfully for notebook %s.", notebookID)
	return nil
}

// QueryResult contains the formatted context and retrieval metadata
type QueryResult struct {
	Context   string   `json:"context"`
	Mode      string   `json:"mode"`
	Sources   []string `json:"sources"`
	Entities  []string `json:"entities"`
	TotalHits int      `json:"total_hits"`
}

// SearchHit represents a candidate chunk returned from dense/sparse searches
type SearchHit struct {
	Content    string
	Source     string
	Similarity float64
	Score      float64 // BM25 or merged RRF score
}

// Query executes local, global, or hybrid retrieval over the notebook graph
func (p *Pipeline) Query(ctx context.Context, notebookID string, query string, mode string) (*QueryResult, error) {
	switch strings.ToLower(mode) {
	case "local":
		return p.localQuery(ctx, notebookID, query)
	case "global":
		return p.globalQuery(ctx, notebookID, query)
	default: // hybrid
		return p.hybridQuery(ctx, notebookID, query)
	}
}

func (p *Pipeline) localQuery(ctx context.Context, notebookID string, query string) (*QueryResult, error) {
	notebookRecordID := db.EnsureRecordID("notebook", notebookID)

	// 1. Dense (vector) search against chunks
	var denseHits []SearchHit
	if p.embedClient != nil {
		emb32, err := p.embedClient.EmbedText(ctx, query)
		if err == nil {
			emb64 := make([]float64, len(emb32))
			for idx, val := range emb32 {
				emb64[idx] = float64(val)
			}

			vectorQuery := `
				SELECT source.title AS source_title, content, vector::similarity::cosine(embedding, $query_vector) AS similarity
				FROM source_embedding
				WHERE source IN (SELECT in FROM reference WHERE out = $nb)
				ORDER BY similarity DESC
				LIMIT 20;
			`
			type vectorResult struct {
				SourceTitle string  `json:"source_title"`
				Content     string  `json:"content"`
				Similarity  float64 `json:"similarity"`
			}
			res, err := db.RepoQuery[[]vectorResult](ctx, vectorQuery, map[string]any{
				"nb":           notebookRecordID,
				"query_vector": emb64,
			})
			if err == nil && res != nil {
				for _, r := range *res {
					denseHits = append(denseHits, SearchHit{
						Content:    r.Content,
						Source:     r.SourceTitle,
						Similarity: r.Similarity,
					})
				}
			}
		}
	}

	// 2. Sparse (BM25 fulltext) search against chunks
	var sparseHits []SearchHit
	fulltextQuery := `
		SELECT source.title AS source_title, content, search::score(1) AS score
		FROM source_embedding
		WHERE content @1@ $query
		  AND source IN (SELECT in FROM reference WHERE out = $nb)
		ORDER BY score DESC
		LIMIT 20;
	`
	type ftResult struct {
		SourceTitle string  `json:"source_title"`
		Content     string  `json:"content"`
		Score       float64 `json:"score"`
	}
	res, err := db.RepoQuery[[]ftResult](ctx, fulltextQuery, map[string]any{
		"nb":    notebookRecordID,
		"query": query,
	})
	if err == nil && res != nil {
		for _, r := range *res {
			sparseHits = append(sparseHits, SearchHit{
				Content:    r.Content,
				Source:     r.SourceTitle,
				Similarity: 0,
				Score:      r.Score,
			})
		}
	}

	// 3. Reciprocal Rank Fusion (RRF)
	fused := ReciprocalRankFusion(denseHits, sparseHits)

	// 4. Graph expansion (1st degree neighbors of matching entities)
	// Extract query entities using standard LLM call
	seedEntities := p.extractQueryEntities(ctx, query)

	// Extract entities from top 5 chunks
	topChunksCount := 5
	if len(fused) < topChunksCount {
		topChunksCount = len(fused)
	}
	for i := 0; i < topChunksCount; i++ {
		chunkEntities := p.extractQueryEntities(ctx, fused[i].Content)
		seedEntities = append(seedEntities, chunkEntities...)
	}

	// Deduplicate seeds
	seedMap := make(map[string]bool)
	var uniqueSeeds []string
	for _, ent := range seedEntities {
		entClean := strings.ToLower(strings.TrimSpace(ent))
		if entClean != "" && !seedMap[entClean] {
			seedMap[entClean] = true
			uniqueSeeds = append(uniqueSeeds, entClean)
		}
	}

	// Find neighbors in SurrealDB
	var neighbors []string
	if len(uniqueSeeds) > 0 {
		neighborsQuery := `
			SELECT in.name AS source, out.name AS target FROM co_occurs
			WHERE notebook = $nb
			  AND (in.name INSIDE $seeds OR out.name INSIDE $seeds);
		`
		type edgeResult struct {
			Source string `json:"source"`
			Target string `json:"target"`
		}
		edges, err := db.RepoQuery[[]edgeResult](ctx, neighborsQuery, map[string]any{
			"nb":    notebookRecordID,
			"seeds": uniqueSeeds,
		})
		if err == nil && edges != nil {
			neighborMap := make(map[string]bool)
			for _, edge := range *edges {
				s, t := edge.Source, edge.Target
				if !seedMap[s] && !neighborMap[s] {
					neighborMap[s] = true
					neighbors = append(neighbors, s)
				}
				if !seedMap[t] && !neighborMap[t] {
					neighborMap[t] = true
					neighbors = append(neighbors, t)
				}
			}
		}
	}

	// Fetch additional chunks related to neighbors
	var expandedHits []SearchHit
	if len(neighbors) > 0 {
		filters := make([]string, 0, len(neighbors))
		for _, n := range neighbors {
			escaped := strings.ReplaceAll(n, "'", "\\'")
			filters = append(filters, fmt.Sprintf("content CONTAINS '%s'", escaped))
		}
		snippetsQuery := fmt.Sprintf(`
			SELECT source.title AS source_title, content FROM source_embedding 
			WHERE source IN (SELECT in FROM reference WHERE out = $nb)
			  AND (%s)
			LIMIT 15;
		`, strings.Join(filters, " OR "))
		type snResult struct {
			SourceTitle string `json:"source_title"`
			Content     string `json:"content"`
		}
		results, err := db.RepoQuery[[]snResult](ctx, snippetsQuery, map[string]any{
			"nb": notebookRecordID,
		})
		if err == nil && results != nil {
			for _, r := range *results {
				expandedHits = append(expandedHits, SearchHit{
					Content:    r.Content,
					Source:     r.SourceTitle,
					Similarity: 0.4, // Boost proxy
				})
			}
		}
	}

	// Merge expanded hits with fused results
	finalHits := mergeSearchHits(fused, expandedHits)

	// Format final local context
	var sb strings.Builder
	var sources []string
	sourceSeen := make(map[string]bool)

	sb.WriteString("### Document Excerpts (Local Context):\n\n")
	for idx, hit := range finalHits {
		if idx >= 6 { // Limit context size
			break
		}
		sb.WriteString(fmt.Sprintf("[Source: %s]\n%s\n\n", hit.Source, hit.Content))
		if !sourceSeen[hit.Source] {
			sourceSeen[hit.Source] = true
			sources = append(sources, hit.Source)
		}
	}

	allEntities := append(uniqueSeeds, neighbors...)

	return &QueryResult{
		Context:   sb.String(),
		Mode:      "local",
		Sources:   sources,
		Entities:  allEntities,
		TotalHits: len(finalHits),
	}, nil
}

func (p *Pipeline) globalQuery(ctx context.Context, notebookID string, query string) (*QueryResult, error) {
	notebookRecordID := db.EnsureRecordID("notebook", notebookID)

	if p.embedClient == nil {
		return &QueryResult{
			Context:   "Global search is currently unavailable because the embedding service is offline.",
			Mode:      "global_fallback",
			TotalHits: 0,
		}, nil
	}

	// 1. Embed user query
	emb32, err := p.embedClient.EmbedText(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to embed search query: %w", err)
	}
	emb64 := make([]float64, len(emb32))
	for idx, val := range emb32 {
		emb64[idx] = float64(val)
	}

	// 2. Query community summaries matching embedding
	commQuery := `
		SELECT community_id, summary, entities, vector::similarity::cosine(embedding, $query_vector) AS similarity
		FROM RAGCommunity
		WHERE notebook = $nb AND embedding != NONE
		ORDER BY similarity DESC
		LIMIT 3;
	`
	type commResult struct {
		CommunityID int      `json:"community_id"`
		Summary     string   `json:"summary"`
		Entities    []string `json:"entities"`
		Similarity  float64  `json:"similarity"`
	}
	results, err := db.RepoQuery[[]commResult](ctx, commQuery, map[string]any{
		"nb":           notebookRecordID,
		"query_vector": emb64,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to query matching communities: %w", err)
	}

	var sb strings.Builder
	sb.WriteString("### Thematic Community Summaries (Global Context):\n\n")

	var entities []string
	entitySeen := make(map[string]bool)

	for _, res := range *results {
		sb.WriteString(fmt.Sprintf("• [Community %d (Similarity: %.2f)] %s\n\n",
			res.CommunityID, res.Similarity, res.Summary))

		for _, ent := range res.Entities {
			if !entitySeen[ent] {
				entitySeen[ent] = true
				entities = append(entities, ent)
			}
		}
	}

	return &QueryResult{
		Context:   sb.String(),
		Mode:      "global",
		Sources:   []string{},
		Entities:  entities,
		TotalHits: len(*results),
	}, nil
}

func (p *Pipeline) hybridQuery(ctx context.Context, notebookID string, query string) (*QueryResult, error) {
	// Execute both local and global queries
	local, err := p.localQuery(ctx, notebookID, query)
	if err != nil {
		return nil, err
	}

	global, err := p.globalQuery(ctx, notebookID, query)
	if err != nil {
		return nil, err
	}

	// Combine Context
	var sb strings.Builder
	sb.WriteString(global.Context)
	sb.WriteString("\n\n---\n\n")
	sb.WriteString(local.Context)

	// Combine Entities
	entitySeen := make(map[string]bool)
	var entities []string
	for _, e := range local.Entities {
		if !entitySeen[e] {
			entitySeen[e] = true
			entities = append(entities, e)
		}
	}
	for _, e := range global.Entities {
		if !entitySeen[e] {
			entitySeen[e] = true
			entities = append(entities, e)
		}
	}

	return &QueryResult{
		Context:   sb.String(),
		Mode:      "hybrid",
		Sources:   local.Sources,
		Entities:  entities,
		TotalHits: local.TotalHits + global.TotalHits,
	}, nil
}

// ReciprocalRankFusion combines dense and sparse hits into a single list
func ReciprocalRankFusion(dense, sparse []SearchHit) []SearchHit {
	k := 60.0
	scores := make(map[string]float64)
	hitsMap := make(map[string]SearchHit)

	for rank, hit := range dense {
		scores[hit.Content] += 0.6 / (k + float64(rank+1))
		hitsMap[hit.Content] = hit
	}

	for rank, hit := range sparse {
		scores[hit.Content] += 0.4 / (k + float64(rank+1))
		hitsMap[hit.Content] = hit
	}

	var results []SearchHit
	for content, score := range scores {
		hit := hitsMap[content]
		hit.Score = score
		results = append(results, hit)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	return results
}

func (p *Pipeline) extractQueryEntities(ctx context.Context, text string) []string {
	systemPrompt := "Extract named entities (canonical lowercase names, e.g., company names, people, locations, events, specific concepts) mentioned in the text. Output them as a raw comma-separated list."
	resp, err := p.chatClient.GenerateText(ctx, systemPrompt, text)
	if err != nil {
		return nil
	}

	parts := strings.Split(resp, ",")
	var entities []string
	for _, p := range parts {
		clean := strings.ToLower(strings.TrimSpace(p))
		if len(clean) > 2 {
			entities = append(entities, clean)
		}
	}
	return entities
}

func mergeSearchHits(a, b []SearchHit) []SearchHit {
	seen := make(map[string]bool)
	var merged []SearchHit

	for _, h := range a {
		if !seen[h.Content] {
			seen[h.Content] = true
			merged = append(merged, h)
		}
	}

	for _, h := range b {
		if !seen[h.Content] {
			seen[h.Content] = true
			merged = append(merged, h)
		}
	}

	return merged
}
