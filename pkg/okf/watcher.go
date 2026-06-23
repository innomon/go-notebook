package okf

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"go-notebook/internal/db"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// WatcherInstance wraps an active fsnotify.Watcher and tracks access times.
type WatcherInstance struct {
	watcher    *fsnotify.Watcher
	indexer    *WorkspaceIndexer
	lastAccess time.Time
	mu         sync.Mutex
	done       chan struct{}
}

// WatcherManager coordinates a pool of active workspace watcher instances.
type WatcherManager struct {
	mu        sync.Mutex
	instances map[string]*WatcherInstance
	timeout   time.Duration
	done      chan struct{}
}

// NewWatcherManager initializes a new WatcherManager with inactivity reaping.
func NewWatcherManager() *WatcherManager {
	wm := &WatcherManager{
		instances: make(map[string]*WatcherInstance),
		timeout:   5 * time.Minute,
		done:      make(chan struct{}),
	}
	go wm.reaperLoop()
	return wm
}

// Close terminates all active watchers and background loops.
func (wm *WatcherManager) Close() {
	close(wm.done)
	wm.mu.Lock()
	defer wm.mu.Unlock()
	for _, inst := range wm.instances {
		close(inst.done)
		_ = inst.watcher.Close()
	}
	wm.instances = make(map[string]*WatcherInstance)
}

// Watch registers or updates an active watch for a target workspace directory.
func (wm *WatcherManager) Watch(ctx context.Context, path string, indexer *WorkspaceIndexer) error {
	cleanPath := filepath.Clean(path)
	wm.mu.Lock()
	inst, exists := wm.instances[cleanPath]
	if exists {
		inst.mu.Lock()
		inst.lastAccess = time.Now()
		inst.mu.Unlock()
		wm.mu.Unlock()
		return nil
	}
	wm.mu.Unlock()

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create fsnotify watcher: %w", err)
	}

	inst = &WatcherInstance{
		watcher:    watcher,
		indexer:    indexer,
		lastAccess: time.Now(),
		done:       make(chan struct{}),
	}

	// Add watchers recursively
	err = filepath.WalkDir(cleanPath, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			name := d.Name()
			if name == ".git" || name == "node_modules" || name == ".obsidian" || name == ".gemini" {
				return filepath.SkipDir
			}
			return watcher.Add(p)
		}
		return nil
	})
	if err != nil {
		watcher.Close()
		return fmt.Errorf("failed to add recursive watcher directories: %w", err)
	}

	wm.mu.Lock()
	wm.instances[cleanPath] = inst
	wm.mu.Unlock()

	go inst.listenEvents(cleanPath)
	return nil
}

// Touch keeps a watcher instance alive by refreshing its access timestamp.
func (wm *WatcherManager) Touch(path string) {
	cleanPath := filepath.Clean(path)
	wm.mu.Lock()
	defer wm.mu.Unlock()
	if inst, exists := wm.instances[cleanPath]; exists {
		inst.mu.Lock()
		inst.lastAccess = time.Now()
		inst.mu.Unlock()
	}
}

func (inst *WatcherInstance) listenEvents(root string) {
	for {
		select {
		case <-inst.done:
			return
		case err, ok := <-inst.watcher.Errors:
			if !ok {
				return
			}
			log.Printf("[OKF Watcher] error: %v", err)
		case event, ok := <-inst.watcher.Events:
			if !ok {
				return
			}

			// Keep instance timestamp active on activity
			inst.mu.Lock()
			inst.lastAccess = time.Now()
			inst.mu.Unlock()

			cleanEventPath := filepath.Clean(event.Name)

			// 1. Directory creation recursion handling
			if event.Has(fsnotify.Create) {
				if info, statErr := os.Stat(cleanEventPath); statErr == nil && info.IsDir() {
					name := info.Name()
					if name != ".git" && name != "node_modules" && name != ".obsidian" && name != ".gemini" {
						_ = inst.watcher.Add(cleanEventPath)
					}
					continue
				}
			}

			// 2. Markdown file event updates
			if strings.HasSuffix(strings.ToLower(cleanEventPath), ".md") {
				relPath, relErr := filepath.Rel(root, cleanEventPath)
				if relErr != nil {
					continue
				}
				relPath = filepath.ToSlash(relPath)

				nodeKey := GetNodeID(root, relPath)
				nodeRecordID := db.EnsureRecordID("okf_node", nodeKey)

				// Handle deletion/rename-removal
				if event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename) {
					log.Printf("[OKF Watcher] File removed: %s", relPath)
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					_ = db.RepoDelete(ctx, nodeRecordID.String())
					cancel()
					continue
				}

				// Handle writes and creations
				if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
					log.Printf("[OKF Watcher] File modified/created: %s", relPath)
					content, readErr := os.ReadFile(cleanEventPath)
					if readErr != nil {
						continue
					}

					h := sha256.New()
					h.Write(content)
					contentHash := hex.EncodeToString(h.Sum(nil))

					ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

					// Parse OKF properties
					meta, body, parseErr := ParseDocument(strings.NewReader(string(content)))
					if parseErr != nil {
						// If document became invalid, delete it from the database graph
						_ = db.RepoDelete(ctx, nodeRecordID.String())
						cancel()
						continue
					}

					// Upsert the node record
					nodeData := map[string]any{
						"workspace_path": root,
						"file_path":      relPath,
						"metadata":      *meta,
						"hash":          contentHash,
						"updated":       time.Now().UTC(),
					}
					_, upsertErr := db.RepoUpsert[OKFNodeRecord](ctx, "okf_node", nodeKey, nodeData, true)
					if upsertErr != nil {
						cancel()
						continue
					}

					// Refresh outgoing link relations
					_, _ = db.RepoQuery[any](ctx, "DELETE okf_link WHERE in = $node;", map[string]any{"node": nodeRecordID})

					links := ExtractLinks(body)
					for _, l := range links {
						targetRel := filepath.Join(filepath.Dir(relPath), l)
						targetRel = filepath.Clean(targetRel)
						targetRel = filepath.ToSlash(targetRel)

						targetDiskPath := filepath.Join(root, targetRel)
						if targetInfo, targetErr := os.Stat(targetDiskPath); targetErr == nil && !targetInfo.IsDir() {
							targetKey := GetNodeID(root, targetRel)
							targetRecordID := db.EnsureRecordID("okf_node", targetKey)
							linkData := map[string]any{
								"workspace_path": root,
							}
							_ = db.RepoRelate(ctx, nodeRecordID.String(), "okf_link", targetRecordID.String(), linkData)
						}
					}
					cancel()
				}
			}
		}
	}
}

func (wm *WatcherManager) reaperLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-wm.done:
			return
		case <-ticker.C:
			wm.mu.Lock()
			now := time.Now()
			for path, inst := range wm.instances {
				inst.mu.Lock()
				if now.Sub(inst.lastAccess) > wm.timeout {
					log.Printf("[OKF Watcher] Closing idle watcher for %s due to inactivity", path)
					close(inst.done)
					_ = inst.watcher.Close()
					delete(wm.instances, path)
				}
				inst.mu.Unlock()
			}
			wm.mu.Unlock()
		}
	}
}
