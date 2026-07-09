package router

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go-notebook/internal/db"
	"go-notebook/internal/domain"
	"go-notebook/pkg/bases"
	"go-notebook/pkg/wasm"

	"github.com/surrealdb/surrealdb.go/pkg/models"
)

type WasmPermissionRecord struct {
	ID             *models.RecordID `json:"id,omitempty"`
	Name           string           `json:"name"`
	ReadOtherNotes bool             `json:"read_other_notes"`
	AccessEnv      bool             `json:"access_env"`
}

type EvaluateRequest struct {
	NotebookID string            `json:"notebook_id,omitempty"`
	Config     *bases.BaseConfig `json:"config"`
}

// RegisterBasesRoutes registers HTTP endpoints for managing WASM plugins and evaluating Bases.
func RegisterBasesRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/bases/plugins", handleGetPlugins)
	mux.HandleFunc("POST /api/bases/plugins/permissions", handlePostPermissions)
	mux.HandleFunc("POST /api/bases/evaluate", handlePostEvaluate)
}

func resolveExtensionsDir() string {
	paths := []string{"extensions/bin", "../extensions/bin", "../../extensions/bin", "../../../extensions/bin"}
	for _, p := range paths {
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			return p
		}
	}
	return "extensions/bin"
}

func handleGetPlugins(w http.ResponseWriter, r *http.Request) {
	extDir := resolveExtensionsDir()
	var plugins []WasmPermissionRecord

	if info, err := os.Stat(extDir); err == nil && info.IsDir() {
		files, err := os.ReadDir(extDir)
		if err == nil {
			for _, file := range files {
				if !file.IsDir() && strings.HasSuffix(strings.ToLower(file.Name()), ".wasm") {
					name := strings.TrimSuffix(file.Name(), filepath.Ext(file.Name()))

					// Query permissions from database, default to all false if not found
					recordID := db.EnsureRecordID("wasm_permission", name)
					rec, err := db.RepoQuery[WasmPermissionRecord](r.Context(), "SELECT * FROM ONLY $id;", map[string]any{"id": recordID})

					if err == nil && rec != nil && rec.ID != nil {
						plugins = append(plugins, *rec)
					} else {
						plugins = append(plugins, WasmPermissionRecord{
							Name:           name,
							ReadOtherNotes: false,
							AccessEnv:      false,
						})
					}
				}
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(plugins)
}

func handlePostPermissions(w http.ResponseWriter, r *http.Request) {
	var req WasmPermissionRecord
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("invalid request: %v", err), http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, "plugin name is required", http.StatusBadRequest)
		return
	}

	data := map[string]any{
		"name":             req.Name,
		"read_other_notes": req.ReadOtherNotes,
		"access_env":       req.AccessEnv,
	}

	_, err := db.RepoUpsert[WasmPermissionRecord](r.Context(), "wasm_permission", req.Name, data, true)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to save permissions: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

func handlePostEvaluate(w http.ResponseWriter, r *http.Request) {
	var req EvaluateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("invalid request: %v", err), http.StatusBadRequest)
		return
	}

	if req.Config == nil {
		http.Error(w, "base configuration is required", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	// 1. Fetch domain Notes
	var dbNotes []domain.Note
	if req.NotebookID != "" {
		// Fetch notes from notebook (including content!)
		recordID := db.EnsureRecordID("notebook", req.NotebookID)
		query := `
			SELECT * from (
				SELECT in as note FROM artifact WHERE out = $notebook_id
				FETCH note
			);
		`
		type ArtifactLink struct {
			Note domain.Note `json:"note"`
		}
		links, err := db.RepoQuery[[]ArtifactLink](ctx, query, map[string]any{"notebook_id": recordID})
		if err == nil && links != nil {
			dbNotes = make([]domain.Note, len(*links))
			for i, l := range *links {
				dbNotes[i] = l.Note
			}
		}
	} else {
		// Fetch all notes in the database
		results, err := db.RepoQuery[[]domain.Note](ctx, "SELECT * FROM note;", nil)
		if err == nil && results != nil {
			dbNotes = *results
		}
	}

	// 2. Parse Notes and inject properties
	var notes []*bases.Note
	for _, dbNote := range dbNotes {
		note, err := bases.ParseNote(dbNote.ID.String(), dbNote.Content)
		if err == nil {
			// Inject system properties if not shadowed
			if _, ok := note.Properties["title"]; !ok {
				note.Properties["title"] = dbNote.Title
			}
			if _, ok := note.Properties["created_at"]; !ok {
				note.Properties["created_at"] = dbNote.Created.Format("2006-01-02")
			}
			if _, ok := note.Properties["updated_at"]; !ok {
				note.Properties["updated_at"] = dbNote.Updated.Format("2006-01-02")
			}
			if _, ok := note.Properties["note_type"]; !ok {
				note.Properties["note_type"] = dbNote.NoteType
			}
			notes = append(notes, note)
		}
	}

	// 3. Initialize WASM Manager and load plugins
	extDir := resolveExtensionsDir()
	log.Printf("[Bases API] Resolved extensions dir: %s", extDir)
	manager, err := wasm.NewManager(ctx)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to init WASM manager: %v", err), http.StatusInternalServerError)
		return
	}
	defer manager.Close(ctx)

	// Keep track of loaded plugin permissions
	pluginPerms := make(map[string]wasm.HostPermissions)

	if info, err := os.Stat(extDir); err == nil && info.IsDir() {
		files, err := os.ReadDir(extDir)
		if err == nil {
			for _, file := range files {
				if !file.IsDir() && strings.HasSuffix(strings.ToLower(file.Name()), ".wasm") {
					path := filepath.Join(extDir, file.Name())
					wasmBytes, err := os.ReadFile(path)
					if err == nil {
						pluginName := strings.TrimSuffix(file.Name(), filepath.Ext(file.Name()))
						err = manager.LoadPlugin(ctx, pluginName, wasmBytes)
						if err == nil {
							log.Printf("[Bases API] Loaded plugin '%s' from %s", pluginName, path)
							// Load permissions for this plugin
							recordID := db.EnsureRecordID("wasm_permission", pluginName)
							rec, err := db.RepoQuery[WasmPermissionRecord](ctx, "SELECT * FROM ONLY $id;", map[string]any{"id": recordID})
							if err == nil && rec != nil && rec.ID != nil {
								pluginPerms[pluginName] = wasm.HostPermissions{
									ReadOtherNotes: rec.ReadOtherNotes,
									AccessEnv:      rec.AccessEnv,
								}
							} else {
								pluginPerms[pluginName] = wasm.HostPermissions{
									ReadOtherNotes: false,
									AccessEnv:      false,
								}
							}
						} else {
							log.Printf("[Bases API] Failed to load plugin '%s': %v", pluginName, err)
						}
					} else {
						log.Printf("[Bases API] Failed to read plugin file %s: %v", path, err)
					}
				}
			}
		} else {
			log.Printf("[Bases API] Failed to read dir %s: %v", extDir, err)
		}
	} else {
		log.Printf("[Bases API] Directory stat failed on %s: %v", extDir, err)
	}

	// 4. Define formula evaluator function
	runFormula := func(funcName string, properties map[string]any) (any, error) {
		payload := struct {
			Properties map[string]any `json:"properties"`
			Args       []string       `json:"args"`
		}{
			Properties: properties,
			Args:       []string{""},
		}

		payloadBytes, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}

		// Retrieve permission settings for this plugin name
		perms := pluginPerms[funcName]

		// Configure execution context with permissions and notes resolver callback
		execCtx := wasm.WithContextPermissions(ctx, perms)
		
		resolver := func(path string) (string, error) {
			// Resolve by record ID (e.g. note:xxxx)
			n, err := domain.GetNote(ctx, path)
			if err == nil {
				return n.Content, nil
			}

			// Fallback: search note by Title in SurrealDB
			results, err := db.RepoQuery[[]domain.Note](ctx, "SELECT * FROM note WHERE title = $title LIMIT 1;", map[string]any{"title": path})
			if err == nil && results != nil && len(*results) > 0 {
				return (*results)[0].Content, nil
			}

			return "", fmt.Errorf("note not found: %s", path)
		}
		
		execCtx = wasm.WithContextNotesResolver(execCtx, resolver)

		resBytes, err := manager.Execute(execCtx, funcName, funcName, payloadBytes)
		if err != nil {
			return nil, err
		}

		var result map[string]any
		if err := json.Unmarshal(resBytes, &result); err != nil {
			return nil, err
		}

		if errVal, ok := result["error"]; ok && errVal != nil {
			return nil, errors.New(fmt.Sprintf("plugin error: %v", errVal))
		}

		for k, v := range result {
			if k != "error" {
				return v, nil
			}
		}

		return nil, errors.New("no result value from plugin")
	}

	// 5. Evaluate Bases Engine
	response, err := bases.Execute(notes, req.Config, runFormula)
	if err != nil {
		http.Error(w, fmt.Sprintf("evaluation failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}
