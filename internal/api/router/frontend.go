package router

import (
	"encoding/json"
	"go-notebook/internal/utils"
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"
)

// RegisterFrontendRoutes binds frontend static asset routes to the ServeMux
func RegisterFrontendRoutes(mux *http.ServeMux, frontendFS fs.FS) {
	// Serve dynamic runtime configuration at /config
	mux.HandleFunc("GET /config", handleFrontendConfig)

	// Create http.FileServer for the embedded static files
	fileServer := http.FileServer(http.FS(frontendFS))

	// Catch-all handler for everything else (including static files, folders, and SPA fallbacks)
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// Trim leading slash
		cleaned := strings.TrimPrefix(path, "/")

		// If requesting root, serve index.html (Next.js index page redirects to /notebooks)
		if cleaned == "" {
			fileServer.ServeHTTP(w, r)
			return
		}

		// Check if it is a request for a static file that exists in the filesystem.
		if fileExists(frontendFS, cleaned) {
			fileServer.ServeHTTP(w, r)
			return
		}

		// Check if it's a dynamic RAG notebook route: /notebooks/[id]
		if strings.HasPrefix(path, "/notebooks/") && !hasFileExtension(path) {
			r.URL.Path = "/notebooks/default.html"
			fileServer.ServeHTTP(w, r)
			return
		}

		// Check if it's a dynamic RAG source route: /sources/[id]
		if strings.HasPrefix(path, "/sources/") && !hasFileExtension(path) {
			r.URL.Path = "/sources/default.html"
			fileServer.ServeHTTP(w, r)
			return
		}

		// Check if appending ".html" resolves to a valid file (e.g. /settings -> /settings.html)
		if fileExists(frontendFS, cleaned+".html") {
			r.URL.Path = path + ".html"
			fileServer.ServeHTTP(w, r)
			return
		}

		// Check if it's a directory with an index.html (e.g. /settings/api-keys -> /settings/api-keys/index.html)
		if !hasFileExtension(path) && fileExists(frontendFS, filepath.Join(cleaned, "index.html")) {
			r.URL.Path = filepath.Join(path, "index.html")
			fileServer.ServeHTTP(w, r)
			return
		}

		// If nothing matched, serve the Next.js 404 page
		if fileExists(frontendFS, "404.html") {
			r.URL.Path = "/404.html"
			fileServer.ServeHTTP(w, r)
			return
		}

		// Final fallback
		http.NotFound(w, r)
	})
}

// fileExists checks if a file exists in the filesystem and is not a directory
func fileExists(f fs.FS, path string) bool {
	info, err := fs.Stat(f, path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func hasFileExtension(path string) bool {
	base := filepath.Base(path)
	return strings.Contains(base, ".")
}

func handleFrontendConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Get API_URL from env or auto-detect
	apiUrl := utils.GetSecretFromEnv("API_URL")
	if apiUrl == "" {
		proto := "http"
		if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
			proto = "https"
		}
		host := r.Host
		if host == "" {
			host = "localhost:5055"
		}
		apiUrl = proto + "://" + host
	}

	_ = json.NewEncoder(w).Encode(map[string]string{
		"apiUrl": apiUrl,
	})
}
