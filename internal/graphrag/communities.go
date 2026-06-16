package graphrag

import (
	"context"
	"fmt"
	"go-notebook/internal/ai"
	"go-notebook/internal/db"
	"go-notebook/internal/domain"
	"log"
	"math/rand"
	"strings"
)

// RunCommunityDetection runs label propagation to find communities and generates summaries
func RunCommunityDetection(ctx context.Context, chatClient ai.AIClient, embedClient ai.AIClient, notebookID string) error {
	notebookRecordID := db.EnsureRecordID("notebook", notebookID)

	// 1. Fetch all entities for this notebook
	entityQuery := "SELECT name FROM RAGEntity WHERE notebook = $nb;"
	entities, err := db.RepoQuery[[]domain.RAGEntity](ctx, entityQuery, map[string]any{"nb": notebookRecordID})
	if err != nil {
		return fmt.Errorf("failed to fetch entities for community detection: %w", err)
	}
	if len(*entities) == 0 {
		return nil // No entities to cluster
	}

	// 2. Fetch all relationship edges for this notebook
	edgesQuery := "SELECT in.name AS source, out.name AS target, weight FROM co_occurs WHERE notebook = $nb;"
	type edgeResult struct {
		Source string  `json:"source"`
		Target string  `json:"target"`
		Weight float64 `json:"weight"`
	}
	edges, err := db.RepoQuery[[]edgeResult](ctx, edgesQuery, map[string]any{"nb": notebookRecordID})
	if err != nil {
		return fmt.Errorf("failed to fetch edges for community detection: %w", err)
	}

	// 3. Build Adjacency List
	nodes := make([]string, 0, len(*entities))
	nodeSet := make(map[string]bool)
	adj := make(map[string]map[string]float64)

	for _, ent := range *entities {
		name := ent.Name
		nodes = append(nodes, name)
		nodeSet[name] = true
		adj[name] = make(map[string]float64)
	}

	for _, edge := range *edges {
		s, t, w := edge.Source, edge.Target, edge.Weight
		// Guard against dangling references
		if nodeSet[s] && nodeSet[t] {
			adj[s][t] += w
			adj[t][s] += w
		}
	}

	// 4. Label Propagation Algorithm (LPA)
	labels := make(map[string]int)
	for i, node := range nodes {
		labels[node] = i // Initial labels are unique
	}

	maxIterations := 15
	for iter := 0; iter < maxIterations; iter++ {
		// Shuffle nodes to prevent propagation bias
		rand.Shuffle(len(nodes), func(i, j int) {
			nodes[i], nodes[j] = nodes[j], nodes[i]
		})

		changed := false
		for _, u := range nodes {
			neighbors := adj[u]
			if len(neighbors) == 0 {
				continue
			}

			// Aggregate label weights from neighbors
			labelWeights := make(map[int]float64)
			for v, weight := range neighbors {
				lbl := labels[v]
				labelWeights[lbl] += weight
			}

			// Find label with highest accumulated weight
			bestLabel := labels[u]
			maxWeight := -1.0
			for lbl, w := range labelWeights {
				if w > maxWeight {
					maxWeight = w
					bestLabel = lbl
				}
			}

			if labels[u] != bestLabel {
				labels[u] = bestLabel
				changed = true
			}
		}

		if !changed {
			break // Converged
		}
	}

	// 5. Group entities into community arrays
	communityGroups := make(map[int][]string)
	for node, label := range labels {
		communityGroups[label] = append(communityGroups[label], node)
	}

	// 6. Delete old communities before generating new ones
	_, err = db.RepoQuery[any](ctx, "DELETE RAGCommunity WHERE notebook = $nb;", map[string]any{"nb": notebookRecordID})
	if err != nil {
		return fmt.Errorf("failed to clear old communities: %w", err)
	}

	// 7. Generate summaries and vector embeddings for each community
	commIndex := 0
	for _, members := range communityGroups {
		if len(members) == 0 {
			continue
		}

		// Filter out single-node communities to focus summaries on actual entity clusters
		if len(members) < 2 {
			continue
		}

		summary, err := generateCommunitySummary(ctx, chatClient, members, notebookID)
		if err != nil {
			log.Printf("[GraphRAG] Warning: failed to generate summary for community %d: %v", commIndex, err)
			summary = fmt.Sprintf("This community links entities: %s.", strings.Join(members, ", "))
		}

		// Generate summary vector embedding
		var embedding []float64
		if embedClient != nil {
			emb32, err := embedClient.EmbedText(ctx, summary)
			if err == nil {
				embedding = make([]float64, len(emb32))
				for idx, val := range emb32 {
					embedding[idx] = float64(val)
				}
			} else {
				log.Printf("[GraphRAG] Warning: failed to embed community summary: %v", err)
			}
		}

		// Save community to database
		commData := map[string]any{
			"community_id": commIndex,
			"notebook":     notebookRecordID,
			"summary":      summary,
			"entities":     members,
		}
		if len(embedding) > 0 {
			commData["embedding"] = embedding
		}

		_, err = db.RepoCreate[domain.RAGCommunity](ctx, "RAGCommunity", commData)
		if err != nil {
			log.Printf("[GraphRAG] Error: failed to save community record %d: %v", commIndex, err)
		}
		commIndex++
	}

	log.Printf("[GraphRAG] Completed community detection. Created %d communities.", commIndex)
	return nil
}

func generateCommunitySummary(ctx context.Context, chatClient ai.AIClient, entities []string, notebookID string) (string, error) {
	notebookRecordID := db.EnsureRecordID("notebook", notebookID)

	// Fetch up to 5 relevant text snippets from notebook source_embeddings that mention these entities
	limit := 5
	entityFilters := make([]string, 0, len(entities))
	for _, ent := range entities {
		// Escape single quotes in entity name to prevent syntax errors
		escaped := strings.ReplaceAll(ent, "'", "\\'")
		entityFilters = append(entityFilters, fmt.Sprintf("content CONTAINS '%s'", escaped))
	}

	snippetsQuery := fmt.Sprintf(`
		SELECT content FROM source_embedding 
		WHERE source.id IN (SELECT out FROM reference WHERE in = $nb)
		  AND (%s)
		LIMIT $limit;
	`, strings.Join(entityFilters, " OR "))

	type snippetResult struct {
		Content string `json:"content"`
	}

	results, err := db.RepoQuery[[]snippetResult](ctx, snippetsQuery, map[string]any{
		"nb":    notebookRecordID,
		"limit": limit,
	})

	var contextParts []string
	if err == nil && results != nil {
		for _, res := range *results {
			contextParts = append(contextParts, res.Content)
		}
	}

	contextText := strings.Join(contextParts, "\n\n")
	if contextText == "" {
		contextText = "(No direct excerpts available)"
	}

	// Truncate entities list if too long for the prompt
	displayEntities := entities
	if len(displayEntities) > 15 {
		displayEntities = displayEntities[:15]
	}

	systemPrompt := `You are an expert analyst summarizing a cluster of related information from a knowledge graph.`
	userPrompt := fmt.Sprintf(`Core Entities in cluster: %s

Relevant Excerpts:
%s

Write a concise 3-5 sentence summary explaining the central topic, entity relationships, key events, and broader significance. Be specific, objective, and do not hallucinate outside the provided texts. Do not mention code details.`,
		strings.Join(displayEntities, ", "), contextText)

	return chatClient.GenerateText(ctx, systemPrompt, userPrompt)
}
