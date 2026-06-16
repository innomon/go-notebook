package router

import (
	"encoding/json"
	"go-notebook/internal/api/middleware"
	"go-notebook/internal/db"
	"go-notebook/internal/utils"
	"io/fs"
	"net/http"
)

// NewRouter sets up all routes using the native Go 1.22+ ServeMux and applies middlewares
func NewRouter(frontendFS fs.FS) http.Handler {
	mux := http.NewServeMux()

	// Base/health endpoints (exempt from auth)
	mux.HandleFunc("GET /health", handleHealth)
	mux.HandleFunc("GET /api/config", handleConfig)
	mux.HandleFunc("GET /api/auth/status", handleAuthStatus)

	// Register domain routes
	RegisterNotebookRoutes(mux)
	RegisterNoteRoutes(mux)
	RegisterSettingsRoutes(mux)
	RegisterTransformationRoutes(mux)
	RegisterLanguagesRoutes(mux)
	RegisterSourceRoutes(mux)
	RegisterInsightRoutes(mux)
	RegisterChatRoutes(mux)
	RegisterCredentialRoutes(mux)
	RegisterModelRoutes(mux)
	RegisterPodcastRoutes(mux)
	RegisterGraphRAGRoutes(mux)

	// Register embedded frontend file server routes
	RegisterFrontendRoutes(mux, frontendFS)

	// Chain middlewares: CORS must be applied first (outermost) to handle OPTIONS preflights,
	// followed by Password Authentication.
	var handler http.Handler = mux
	handler = middleware.PasswordAuth(handler)
	handler = middleware.CORS(handler)

	return handler
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status": "healthy",
	})
}

func handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	password := utils.GetSecretFromEnv("OPEN_NOTEBOOK_PASSWORD")
	authEnabled := password != ""

	message := "Authentication is disabled"
	if authEnabled {
		message = "Authentication is required"
	}

	_ = json.NewEncoder(w).Encode(map[string]any{
		"auth_enabled": authEnabled,
		"message":      message,
	})
}

func handleConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Get database status
	dbStatus, err := db.CheckDatabaseHealth(r.Context())
	if err != nil {
		dbStatus = "offline"
	}

	// Replicate config structure expected by the frontend
	_ = json.NewEncoder(w).Encode(map[string]any{
		"version":       "1.9.0", // Match original python pyproject version
		"latestVersion": "1.9.0",
		"hasUpdate":     false,
		"dbStatus":      dbStatus,
	})
}
