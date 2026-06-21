package domain

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"go-notebook/internal/db"
	"io"
	"regexp"
	"sort"
	"strings"

	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// CoOccursRecord represents a serialized co_occurs edge record in SurrealDB
type CoOccursRecord struct {
	ID       *models.RecordID   `json:"id,omitempty"`
	In       *models.RecordID   `json:"in"`
	Out      *models.RecordID   `json:"out"`
	Sources  []*models.RecordID `json:"sources"`
	Weight   int                `json:"weight"`
	Notebook *models.RecordID   `json:"notebook"`
}

// ExportGraph represents the serialized GraphRAG database tables
type ExportGraph struct {
	Sources     []Source         `json:"sources"`
	Entities    []RAGEntity      `json:"entities"`
	CoOccurs    []CoOccursRecord `json:"co_occurs"`
	Communities []RAGCommunity   `json:"communities"`
}

// ExportMetadata represents notebook details exported to metadata.json
type ExportMetadata struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type entityRegex struct {
	name string
	re   *regexp.Regexp
}

// SanitizeFilename replaces characters that are invalid in filenames
func SanitizeFilename(filename string) string {
	invalid := regexp.MustCompile(`[\\/:*?"<>|]`)
	sanitized := invalid.ReplaceAllString(filename, "_")
	sanitized = strings.TrimSpace(sanitized)
	if sanitized == "" {
		sanitized = "untitled"
	}
	return sanitized
}

func isWordChar(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_'
}

func getEntityRegexPattern(entity string) string {
	quoted := regexp.QuoteMeta(entity)
	var sb strings.Builder
	sb.WriteString("(?i)")
	if len(entity) > 0 {
		first := entity[0]
		if isWordChar(first) {
			sb.WriteString(`\b`)
		}
	}
	sb.WriteString(quoted)
	if len(entity) > 0 {
		last := entity[len(entity)-1]
		if isWordChar(last) {
			sb.WriteString(`\b`)
		}
	}
	return sb.String()
}

func splitByBrackets(s string) []string {
	var parts []string
	current := s
	for {
		startIdx := strings.Index(current, "[[")
		if startIdx == -1 {
			parts = append(parts, current)
			break
		}
		parts = append(parts, current[:startIdx])
		current = current[startIdx+2:]

		endIdx := strings.Index(current, "]]")
		if endIdx == -1 {
			parts = append(parts, "[["+current)
			break
		}
		parts = append(parts, current[:endIdx])
		current = current[endIdx+2:]
	}
	return parts
}

func joinByBrackets(parts []string) string {
	var sb strings.Builder
	for i, part := range parts {
		if i > 0 {
			if i%2 == 1 {
				sb.WriteString("[[")
			} else {
				sb.WriteString("]]")
			}
		}
		sb.WriteString(part)
	}
	return sb.String()
}

func wrapEntitiesInText(text string, regexes []entityRegex) string {
	currentText := text
	for _, er := range regexes {
		parts := splitByBrackets(currentText)
		for i := 0; i < len(parts); i += 2 {
			parts[i] = er.re.ReplaceAllStringFunc(parts[i], func(match string) string {
				return "[[" + match + "]]"
			})
		}
		currentText = joinByBrackets(parts)
	}
	return currentText
}

// ExportNotebookToZip serializes a notebook and writes it to w as a zip file
func ExportNotebookToZip(ctx context.Context, notebookID string, w io.Writer) error {
	notebook, err := GetNotebook(ctx, notebookID)
	if err != nil {
		return fmt.Errorf("failed to fetch notebook: %w", err)
	}

	nbRecordID := db.EnsureRecordID("notebook", notebookID)

	// Fetch all sources with full_text
	type SourceLink struct {
		Source Source `json:"source"`
	}
	sourceQuery := "SELECT in as source FROM reference WHERE out = $nb FETCH source;"
	sourceLinks, err := db.RepoQuery[[]SourceLink](ctx, sourceQuery, map[string]any{"nb": nbRecordID})
	if err != nil {
		return fmt.Errorf("failed to fetch sources for export: %w", err)
	}

	sourcesList := []Source{}
	if sourceLinks != nil {
		for _, link := range *sourceLinks {
			sourcesList = append(sourcesList, link.Source)
		}
	}

	// Fetch all notes with content
	type NoteLink struct {
		Note Note `json:"note"`
	}
	noteQuery := "SELECT in as note FROM artifact WHERE out = $nb FETCH note;"
	noteLinks, err := db.RepoQuery[[]NoteLink](ctx, noteQuery, map[string]any{"nb": nbRecordID})
	if err != nil {
		return fmt.Errorf("failed to fetch notes for export: %w", err)
	}

	notesList := []Note{}
	if noteLinks != nil {
		for _, link := range *noteLinks {
			notesList = append(notesList, link.Note)
		}
	}

	// Fetch RAGEntities
	entities, err := db.RepoQuery[[]RAGEntity](ctx, "SELECT * FROM RAGEntity WHERE notebook = $nb;", map[string]any{"nb": nbRecordID})
	if err != nil {
		return fmt.Errorf("failed to fetch RAGEntity records: %w", err)
	}
	var entitiesList []RAGEntity
	if entities != nil {
		entitiesList = *entities
	}

	// Fetch co_occurs edges
	edges, err := db.RepoQuery[[]CoOccursRecord](ctx, "SELECT * FROM co_occurs WHERE notebook = $nb;", map[string]any{"nb": nbRecordID})
	if err != nil {
		return fmt.Errorf("failed to fetch co_occurs records: %w", err)
	}
	var edgesList []CoOccursRecord
	if edges != nil {
		edgesList = *edges
	}

	// Fetch RAGCommunities
	communities, err := db.RepoQuery[[]RAGCommunity](ctx, "SELECT * FROM RAGCommunity WHERE notebook = $nb;", map[string]any{"nb": nbRecordID})
	if err != nil {
		return fmt.Errorf("failed to fetch RAGCommunity records: %w", err)
	}
	var communitiesList []RAGCommunity
	if communities != nil {
		communitiesList = *communities
	}

	// Prepare entity regexes for wiki-linking
	entityNames := make([]string, len(entitiesList))
	for i, ent := range entitiesList {
		entityNames[i] = ent.Name
	}
	// Sort by length descending to prevent substring issues
	sort.Slice(entityNames, func(i, j int) bool {
		return len(entityNames[i]) > len(entityNames[j])
	})

	regexes := make([]entityRegex, 0, len(entityNames))
	for _, name := range entityNames {
		pattern := getEntityRegexPattern(name)
		re, err := regexp.Compile(pattern)
		if err == nil {
			regexes = append(regexes, entityRegex{name: name, re: re})
		}
	}

	// Source maps for mapping source RecordID to filename
	sourceMap := make(map[string]string) // ID -> Sanitized Title
	for _, s := range sourcesList {
		sourceMap[s.ID.String()] = SanitizeFilename(s.Title)
	}

	// Initialize zip writer
	archive := zip.NewWriter(w)
	defer archive.Close()

	// Write metadata.json
	metaFile, err := archive.Create("metadata.json")
	if err != nil {
		return fmt.Errorf("failed to create metadata.json in zip: %w", err)
	}
	metadata := ExportMetadata{
		Name:        notebook.Name,
		Description: notebook.Description,
	}
	metaDataBytes, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}
	if _, err := metaFile.Write(metaDataBytes); err != nil {
		return fmt.Errorf("failed to write metadata.json to zip: %w", err)
	}

	// Write sources/
	for _, s := range sourcesList {
		filename := fmt.Sprintf("sources/%s.md", SanitizeFilename(s.Title))
		f, err := archive.Create(filename)
		if err != nil {
			return fmt.Errorf("failed to create source file %s in zip: %w", filename, err)
		}
		processedText := wrapEntitiesInText(s.FullText, regexes)
		if _, err := f.Write([]byte(processedText)); err != nil {
			return fmt.Errorf("failed to write source %s to zip: %w", filename, err)
		}
	}

	// Write notes/
	for _, n := range notesList {
		filename := fmt.Sprintf("notes/%s.md", SanitizeFilename(n.Title))
		f, err := archive.Create(filename)
		if err != nil {
			return fmt.Errorf("failed to create note file %s in zip: %w", filename, err)
		}
		processedText := wrapEntitiesInText(n.Content, regexes)
		if _, err := f.Write([]byte(processedText)); err != nil {
			return fmt.Errorf("failed to write note %s to zip: %w", filename, err)
		}
	}

	// Write entities/
	for _, ent := range entitiesList {
		filename := fmt.Sprintf("entities/%s.md", SanitizeFilename(ent.Name))
		f, err := archive.Create(filename)
		if err != nil {
			return fmt.Errorf("failed to create entity file %s in zip: %w", filename, err)
		}

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("# Entity: %s\n\n", ent.Name))
		sb.WriteString(fmt.Sprintf("- **Mention Count**: %d\n", ent.Count))
		sb.WriteString("- **Type**: Entity\n\n")

		sb.WriteString("## Backlinks to Sources\n")
		if len(ent.Sources) > 0 {
			for _, srcID := range ent.Sources {
				if title, ok := sourceMap[srcID.String()]; ok {
					sb.WriteString(fmt.Sprintf("- [[%s]]\n", title))
				} else {
					sb.WriteString(fmt.Sprintf("- [[%s]]\n", srcID.ID))
				}
			}
		} else {
			sb.WriteString("*No source backlinks.*\n")
		}

		if _, err := f.Write([]byte(sb.String())); err != nil {
			return fmt.Errorf("failed to write entity %s to zip: %w", filename, err)
		}
	}

	// Write graph.json
	graphFile, err := archive.Create("graph.json")
	if err != nil {
		return fmt.Errorf("failed to create graph.json in zip: %w", err)
	}
	graph := ExportGraph{
		Sources:     sourcesList,
		Entities:    entitiesList,
		CoOccurs:    edgesList,
		Communities: communitiesList,
	}
	graphBytes, err := json.MarshalIndent(graph, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal graph: %w", err)
	}
	if _, err := graphFile.Write(graphBytes); err != nil {
		return fmt.Errorf("failed to write graph.json to zip: %w", err)
	}

	return nil
}

// ImportMergeNotebookFromZip merges notes, sources, and graph records from a zipped vault into an existing notebook.
func ImportMergeNotebookFromZip(ctx context.Context, notebookID string, r io.ReaderAt, size int64) error {
	// Verify notebook exists
	if _, err := GetNotebook(ctx, notebookID); err != nil {
		return fmt.Errorf("target notebook not found: %w", err)
	}

	zr, err := zip.NewReader(r, size)
	if err != nil {
		return fmt.Errorf("failed to read zip archive: %w", err)
	}

	// 1. Read files and locate graph.json
	var graphJSON []byte
	sourcesInZip := make(map[string][]byte) // sanitized filename -> content
	notesInZip := make(map[string][]byte)   // sanitized filename -> content

	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("failed to open zip file %s: %w", f.Name, err)
		}
		var buf bytes.Buffer
		_, err = io.Copy(&buf, rc)
		rc.Close()
		if err != nil {
			return fmt.Errorf("failed to read zip file %s: %w", f.Name, err)
		}

		if f.Name == "graph.json" {
			graphJSON = buf.Bytes()
		} else if strings.HasPrefix(f.Name, "sources/") && strings.HasSuffix(f.Name, ".md") {
			name := strings.TrimSuffix(strings.TrimPrefix(f.Name, "sources/"), ".md")
			sourcesInZip[name] = buf.Bytes()
		} else if strings.HasPrefix(f.Name, "notes/") && strings.HasSuffix(f.Name, ".md") {
			name := strings.TrimSuffix(strings.TrimPrefix(f.Name, "notes/"), ".md")
			notesInZip[name] = buf.Bytes()
		}
	}

	// 2. Fetch current sources of notebook to build conflict mapping
	existingSourcesList, err := GetNotebookSources(ctx, notebookID)
	if err != nil {
		return fmt.Errorf("failed to retrieve current notebook sources: %w", err)
	}

	hashToSource := make(map[string]Source)
	titleToSource := make(map[string]Source)
	for _, s := range existingSourcesList {
		if s.Hash != "" {
			hashToSource[s.Hash] = s
		}
		titleToSource[s.Title] = s
	}

	// Map of original source ID (from graph.json) -> resolved DB RecordID
	sourceIDMapping := make(map[string]*models.RecordID)

	// Keep track of imported/created source metadata to help build the graph if graph.json doesn't contain source ID mapping.
	// But first, let's parse graph.json if it exists.
	var graph ExportGraph
	if len(graphJSON) > 0 {
		_ = json.Unmarshal(graphJSON, &graph)
	}

	// Map to associate original Source record details from graph.json
	originalSourcesMap := make(map[string]Source)
	for _, s := range graph.Sources {
		originalSourcesMap[s.ID.String()] = s
	}

	// 3. Process Sources
	// We want to process markdown files under sources/
	for sanitizedName, contentBytes := range sourcesInZip {
		content := string(contentBytes)
		hasher := sha256.New()
		hasher.Write(contentBytes)
		hashStr := hex.EncodeToString(hasher.Sum(nil))

		// Check if hash matches an existing source in the notebook
		if existingSrc, exists := hashToSource[hashStr]; exists {
			// Skip duplicate file, map the matching source IDs
			// Find if there was an original Source ID matching this title/hash
			var origID string
			for id, origS := range originalSourcesMap {
				if SanitizeFilename(origS.Title) == sanitizedName {
					origID = id
					break
				}
			}
			if origID != "" {
				sourceIDMapping[origID] = existingSrc.ID
			}
			continue
		}

		// Reconstruct title
		originalTitle := sanitizedName
		// If we have original sources from graph.json, get exact original title
		var origID string
		for id, origS := range originalSourcesMap {
			if SanitizeFilename(origS.Title) == sanitizedName {
				originalTitle = origS.Title
				origID = id
				break
			}
		}

		// Resolve title conflict if hashes didn't match but name exists
		finalTitle := originalTitle
		if _, exists := titleToSource[finalTitle]; exists {
			suffix := 1
			for {
				candidateTitle := fmt.Sprintf("%s_%d", originalTitle, suffix)
				if _, exists := titleToSource[candidateTitle]; !exists {
					finalTitle = candidateTitle
					break
				}
				suffix++
			}
		}

		// Create source directly in database (avoid queueing process_source since text is ready)
		data := map[string]any{
			"title":     finalTitle,
			"full_text": content,
			"hash":      hashStr,
			"topics":    []string{},
		}
		newSource, err := db.RepoCreate[Source](ctx, "source", data)
		if err != nil {
			return fmt.Errorf("failed to create source %q: %w", finalTitle, err)
		}

		// Link source to notebook
		if err := LinkSourceToNotebook(ctx, newSource.ID.String(), notebookID); err != nil {
			return fmt.Errorf("failed to link source %q: %w", finalTitle, err)
		}

		// Add to mappings
		hashToSource[hashStr] = *newSource
		titleToSource[finalTitle] = *newSource
		if origID != "" {
			sourceIDMapping[origID] = newSource.ID
		}
	}

	// 4. Process Notes
	for sanitizedName, contentBytes := range notesInZip {
		content := string(contentBytes)
		originalTitle := sanitizedName

		// Create Note
		_, _, err := CreateNote(ctx, originalTitle, content, "human", notebookID)
		if err != nil {
			return fmt.Errorf("failed to create note %q: %w", originalTitle, err)
		}
	}

	// 5. Restore Graph elements if graph.json exists
	if len(graphJSON) > 0 {
		// Maps of imported Entity RecordID string -> created RAGEntity object
		entityMapping := make(map[string]*RAGEntity)

		// Restore RAGEntities
		for _, ent := range graph.Entities {
			// Resolve sources list
			var resolvedSources []*models.RecordID
			for _, srcID := range ent.Sources {
				if mappedID, ok := sourceIDMapping[srcID.String()]; ok {
					resolvedSources = append(resolvedSources, mappedID)
				}
			}

			// Create or update the entity in target notebook directly
			query := `
				LET $existing = (SELECT id FROM RAGEntity WHERE notebook = $nb AND name = $name)[0];
				IF $existing.id != NONE {
					UPDATE $existing.id SET 
						sources = array::union(sources, $sources),
						count = count(array::union(sources, $sources));
				} ELSE {
					CREATE RAGEntity CONTENT {
						notebook: $nb,
						name: $name,
						sources: $sources,
						count: count($sources)
					};
				};
			`
			_, err = db.RepoQuery[any](ctx, query, map[string]any{
				"nb":      db.EnsureRecordID("notebook", notebookID),
				"name":    ent.Name,
				"sources": resolvedSources,
			})
			if err != nil {
				return fmt.Errorf("failed to restore entity %q: %w", ent.Name, err)
			}

			// Fetch the created/updated entity to put in entityMapping
			refetched, err := db.RepoQuery[[]RAGEntity](ctx, "SELECT * FROM RAGEntity WHERE notebook = $nb AND name = $name LIMIT 1;", map[string]any{
				"nb":   db.EnsureRecordID("notebook", notebookID),
				"name": ent.Name,
			})
			if err != nil || len(*refetched) == 0 {
				return fmt.Errorf("failed to refetch restored entity %q: %w", ent.Name, err)
			}
			dbEntity := &(*refetched)[0]
			entityMapping[ent.ID.String()] = dbEntity
		}

		// Restore co_occurs relationships
		for _, edge := range graph.CoOccurs {
			entA := entityMapping[edge.In.String()]
			entB := entityMapping[edge.Out.String()]
			if entA == nil || entB == nil {
				continue
			}

			// Resolve sources
			var resolvedSources []*models.RecordID
			for _, srcID := range edge.Sources {
				if mappedID, ok := sourceIDMapping[srcID.String()]; ok {
					resolvedSources = append(resolvedSources, mappedID)
				}
			}

			if len(resolvedSources) == 0 {
				continue
			}

			// Create relation directly in SurrealDB
			fromID := entA.ID.String()
			toID := entB.ID.String()
			if entA.Name > entB.Name {
				fromID, toID = toID, fromID
			}

			query := `
				LET $existing = (SELECT id, sources FROM co_occurs WHERE in = $from AND out = $to)[0];
				IF $existing.id != NONE {
					UPDATE $existing.id SET 
						sources = array::union(sources, $sources),
						weight = count(array::union(sources, $sources));
				} ELSE {
					RELATE $from->co_occurs->$to CONTENT {
						sources: $sources,
						weight: count($sources),
						notebook: $nb
					};
				};
			`
			_, err = db.RepoQuery[any](ctx, query, map[string]any{
				"from":    db.EnsureRecordID("RAGEntity", fromID),
				"to":      db.EnsureRecordID("RAGEntity", toID),
				"nb":      db.EnsureRecordID("notebook", notebookID),
				"sources": resolvedSources,
			})
			if err != nil {
				return fmt.Errorf("failed to restore co_occurs connection between %q and %q: %w", entA.Name, entB.Name, err)
			}
		}

		// Restore RAGCommunities
		for _, comm := range graph.Communities {
			query := `
				CREATE RAGCommunity CONTENT {
					community_id: $comm_id,
					notebook: $nb,
					summary: $summary,
					entities: $entities,
					created: time::now()
				};
			`
			_, err = db.RepoQuery[any](ctx, query, map[string]any{
				"comm_id":  comm.CommunityID,
				"nb":       db.EnsureRecordID("notebook", notebookID),
				"summary":  comm.Summary,
				"entities": comm.Entities,
			})
			if err != nil {
				return fmt.Errorf("failed to restore RAGCommunity %d: %w", comm.CommunityID, err)
			}
		}
	}

	return nil
}

// ImportNewNotebookFromZip creates a new notebook and imports notes, sources, and graph records from a zipped vault.
func ImportNewNotebookFromZip(ctx context.Context, r io.ReaderAt, size int64) (*Notebook, error) {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return nil, fmt.Errorf("failed to read zip archive: %w", err)
	}

	// Find metadata.json to determine name and description
	var metadata ExportMetadata
	metadata.Name = "Imported Notebook"
	metadata.Description = "Imported from Obsidian Vault zip"

	for _, f := range zr.File {
		if f.Name == "metadata.json" {
			rc, err := f.Open()
			if err != nil {
				break
			}
			_ = json.NewDecoder(rc).Decode(&metadata)
			rc.Close()
			break
		}
	}

	// Resolve conflicts with duplicate notebook names
	notebooks, err := ListNotebooks(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list notebooks: %w", err)
	}
	notebookNameMap := make(map[string]bool)
	for _, nb := range notebooks {
		notebookNameMap[nb.Name] = true
	}

	finalName := metadata.Name
	if notebookNameMap[finalName] {
		suffix := 1
		for {
			candidateName := fmt.Sprintf("%s (%d)", metadata.Name, suffix)
			if !notebookNameMap[candidateName] {
				finalName = candidateName
				break
			}
			suffix++
		}
	}

	// Create Notebook
	notebook, err := CreateNotebook(ctx, finalName, metadata.Description)
	if err != nil {
		return nil, fmt.Errorf("failed to create imported notebook: %w", err)
	}

	notebookIDStr := notebook.ID.String()

	// Merge vault contents into the newly created notebook
	err = ImportMergeNotebookFromZip(ctx, notebookIDStr, r, size)
	if err != nil {
		// Clean up the created notebook on failure
		_, _, _, _ = DeleteNotebook(ctx, notebookIDStr, true)
		return nil, err
	}

	return notebook, nil
}
