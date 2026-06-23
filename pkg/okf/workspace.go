package okf

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"go-notebook/internal/db"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// OKFNodeRecord represents a persisted OKF document node in SurrealDB.
type OKFNodeRecord struct {
	ID            *models.RecordID `json:"id,omitempty"`
	WorkspacePath string           `json:"workspace_path"`
	FilePath      string           `json:"file_path"`
	Metadata      Metadata         `json:"metadata"`
	Hash          string           `json:"hash"`
	Updated       time.Time        `json:"updated,omitempty"`
}

// WorkspaceIndexer scans and indexes an OKF workspace folder into SurrealDB.
type WorkspaceIndexer struct {
	RootPath string
	mu       sync.Mutex
}

// NewWorkspaceIndexer creates a new indexer targeting a specific root path.
func NewWorkspaceIndexer(rootPath string) *WorkspaceIndexer {
	return &WorkspaceIndexer{
		RootPath: filepath.Clean(rootPath),
	}
}

// GetNodeID computes a unique hex string for a given node path under a workspace.
func GetNodeID(workspacePath, filePath string) string {
	h := sha256.New()
	h.Write([]byte(filepath.Clean(workspacePath) + "::" + filepath.Clean(filePath)))
	return hex.EncodeToString(h.Sum(nil))
}

// Index runs a tree-walk of the workspace path, parsing new or updated notes
// and syncing them to SurrealDB.
func (idx *WorkspaceIndexer) Index(ctx context.Context) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	// Verify directory exists
	info, err := os.Stat(idx.RootPath)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("invalid workspace directory: %s", idx.RootPath)
	}

	var mdFiles []string
	err = filepath.WalkDir(idx.RootPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if name == ".git" || name == "node_modules" || name == ".obsidian" || name == ".gemini" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(strings.ToLower(d.Name()), ".md") {
			mdFiles = append(mdFiles, path)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to walk workspace path: %w", err)
	}

	foundRelPaths := make(map[string]bool)

	for _, path := range mdFiles {
		relPath, err := filepath.Rel(idx.RootPath, path)
		if err != nil {
			continue
		}
		// Standardize slashes to forward slashes for cross-platform consistency
		relPath = filepath.ToSlash(relPath)

		// Read content
		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		// Calculate hash
		h := sha256.New()
		h.Write(content)
		contentHash := hex.EncodeToString(h.Sum(nil))

		nodeKey := getNodeID(idx.RootPath, relPath)
		nodeRecordID := db.EnsureRecordID("okf_node", nodeKey)

		// Query database for existing hash
		type HashCheck struct {
			ID   *models.RecordID `json:"id"`
			Hash string           `json:"hash"`
		}
		existing, err := db.RepoQuery[[]HashCheck](ctx, "SELECT id, hash FROM okf_node WHERE id = $id;", map[string]any{"id": nodeRecordID})
		if err == nil && existing != nil && len(*existing) > 0 && (*existing)[0].Hash == contentHash {
			// Hash matches, skip processing
			foundRelPaths[relPath] = true
			continue
		}

		// Process mismatch or new node
		meta, body, parseErr := ParseDocument(strings.NewReader(string(content)))
		if parseErr != nil {
			// If file became invalid, ensure it is deleted from the graph database
			_ = db.RepoDelete(ctx, nodeRecordID.String())
			continue
		}

		// Save the valid node
		nodeData := map[string]any{
			"workspace_path": idx.RootPath,
			"file_path":      relPath,
			"metadata":      *meta,
			"hash":          contentHash,
			"updated":       time.Now().UTC(),
		}
		_, err = db.RepoUpsert[OKFNodeRecord](ctx, "okf_node", nodeKey, nodeData, true)
		if err != nil {
			continue
		}

		foundRelPaths[relPath] = true

		// Clear existing outbound links
		_, _ = db.RepoQuery[any](ctx, "DELETE okf_link WHERE in = $node;", map[string]any{"node": nodeRecordID})

		// Extract and resolve outbound links
		links := ExtractLinks(body)
		for _, l := range links {
			// Resolve strictly relative to file's directory
			targetRel := filepath.Join(filepath.Dir(relPath), l)
			targetRel = filepath.Clean(targetRel)
			targetRel = filepath.ToSlash(targetRel)

			// Check if target file exists on disk
			targetDiskPath := filepath.Join(idx.RootPath, targetRel)
			if targetInfo, targetErr := os.Stat(targetDiskPath); targetErr == nil && !targetInfo.IsDir() {
				// Relate nodes
				targetKey := getNodeID(idx.RootPath, targetRel)
				targetRecordID := db.EnsureRecordID("okf_node", targetKey)
				linkData := map[string]any{
					"workspace_path": idx.RootPath,
				}
				_ = db.RepoRelate(ctx, nodeRecordID.String(), "okf_link", targetRecordID.String(), linkData)
			}
		}
	}

	// Purge nodes no longer on disk
	type NodeItem struct {
		ID       *models.RecordID `json:"id"`
		FilePath string           `json:"file_path"`
	}
	dbNodes, err := db.RepoQuery[[]NodeItem](ctx, "SELECT id, file_path FROM okf_node WHERE workspace_path = $ws;", map[string]any{"ws": idx.RootPath})
	if err == nil && dbNodes != nil {
		for _, item := range *dbNodes {
			if !foundRelPaths[item.FilePath] {
				_ = db.RepoDelete(ctx, item.ID.String())
			}
		}
	}

	return nil
}

// GetGraph retrieves the collection of BundleNodes from the database.
func (idx *WorkspaceIndexer) GetGraph(ctx context.Context) ([]BundleNode, error) {
	type NodeQueryResult struct {
		ID       *models.RecordID `json:"id"`
		Metadata Metadata         `json:"metadata"`
		FilePath string           `json:"file_path"`
		Out      []string         `json:"out"`
	}

	// Query to fetch all nodes and their outgoing relation targets
	query := `
		SELECT 
			id,
			metadata,
			file_path,
			(SELECT ->okf_link->okf_node.file_path as paths FROM $this).paths[0] AS out
		FROM okf_node
		WHERE workspace_path = $ws;
	`
	results, err := db.RepoQuery[[]NodeQueryResult](ctx, query, map[string]any{"ws": idx.RootPath})
	if err != nil {
		return nil, fmt.Errorf("failed to select graph nodes: %w", err)
	}

	if results == nil {
		return []BundleNode{}, nil
	}

	nodes := make([]BundleNode, len(*results))
	for i, item := range *results {
		outbound := []string{}
		for _, path := range item.Out {
			if path != "" {
				outbound = append(outbound, path)
			}
		}
		nodes[i] = BundleNode{
			ID:            item.FilePath,
			Metadata:      item.Metadata,
			OutboundLinks: outbound,
		}
	}

	return nodes, nil
}

func getNodeID(workspacePath, filePath string) string {
	return GetNodeID(workspacePath, filePath)
}
