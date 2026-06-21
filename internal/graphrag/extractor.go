package graphrag

import (
	"context"
	"encoding/json"
	"fmt"
	"go-notebook/internal/ai"
	"go-notebook/internal/domain"
	"log"
	"regexp"
	"strings"
)

// ExtractedEntity matches the LLM response entity format
type ExtractedEntity struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// ExtractedRelationship matches the LLM response relationship format
type ExtractedRelationship struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

// ExtractedGraph matches the entire JSON output format of the LLM response
type ExtractedGraph struct {
	Entities      []ExtractedEntity      `json:"entities"`
	Relationships []ExtractedRelationship `json:"relationships"`
}

const EntityExtractionSystemPrompt = `You are an AI assistant tasked with extracting key named entities and their co-occurrence relationships from the provided text to construct a Knowledge Graph.

Format your output EXACTLY as a JSON object with two fields:
1. "entities": a list of objects, each containing:
   - "name": the canonical lowercase name of the entity (e.g. "microsoft", "john doe")
   - "type": the entity type (e.g. "ORGANIZATION", "PERSON", "LOCATION", "PRODUCT", "EVENT", "CONCEPT")
2. "relationships": a list of objects, each containing:
   - "source": the name of the first entity
   - "target": the name of the second entity

CRITICAL RULES:
- Output ONLY valid JSON.
- Do NOT include markdown code blocks (like ` + "```json" + `) or any other conversational text.
- Keep entity names concise, lowercase, and normalized.
- Only extract prominent entities and real relationships clearly described in the text.`

// ExtractEntitiesAndRelations calls the LLM to extract entities/relations from a chunk
func ExtractEntitiesAndRelations(ctx context.Context, aiClient ai.AIClient, chunkText string) (*ExtractedGraph, error) {
	prompt := fmt.Sprintf("Analyze the following text chunk:\n\n%s", chunkText)

	response, err := aiClient.GenerateText(ctx, EntityExtractionSystemPrompt, prompt)
	if err != nil {
		return nil, err
	}

	// Clean Markdown code block formatting if present
	cleanJSON := cleanJSONResponse(response)

	var graph ExtractedGraph
	if err := json.Unmarshal([]byte(cleanJSON), &graph); err != nil {
		return nil, fmt.Errorf("failed to parse LLM extraction JSON: %w\nResponse was:\n%s", err, cleanJSON)
	}

	return &graph, nil
}

// IngestChunkGraph extracts entities/relations and persists them in SurrealDB
func IngestChunkGraph(ctx context.Context, aiClient ai.AIClient, notebookID string, sourceID string, chunkText string) error {
	graph, err := ExtractEntitiesAndRelations(ctx, aiClient, chunkText)
	if err != nil {
		return err
	}

	// Store entities and build name-to-record mapping
	entityMap := make(map[string]*domain.RAGEntity)
	for _, extEnt := range graph.Entities {
		name := strings.ToLower(strings.TrimSpace(extEnt.Name))
		if len(name) < 2 {
			continue
		}

		ent, err := domain.CreateOrUpdateEntity(ctx, notebookID, sourceID, name)
		if err != nil {
			log.Printf("[GraphRAG] Warning: failed to save entity %q: %v", name, err)
			continue
		}
		entityMap[name] = ent
	}

	// Store relationships (edges)
	for _, rel := range graph.Relationships {
		src := strings.ToLower(strings.TrimSpace(rel.Source))
		tgt := strings.ToLower(strings.TrimSpace(rel.Target))

		entA, existsA := entityMap[src]
		entB, existsB := entityMap[tgt]

		if existsA && existsB && src != tgt {
			err := domain.RelateEntities(ctx, notebookID, sourceID, entA, entB)
			if err != nil {
				log.Printf("[GraphRAG] Warning: failed to relate %q and %q: %v", src, tgt, err)
			}
		}
	}

	return nil
}

func cleanJSONResponse(resp string) string {
	resp = strings.TrimSpace(resp)

	// Remove leading markdown blocks: ```json or ```
	re := regexp.MustCompile("(?s)^```(?:json)?(.*?)```$")
	if matches := re.FindStringSubmatch(resp); len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}

	return resp
}
