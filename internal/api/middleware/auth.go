package middleware

import (
	"encoding/json"
	"go-notebook/internal/utils"
	"net/http"
	"strings"
)

// PasswordAuth is a middleware that enforces password authentication for non-exempt paths
func PasswordAuth(next http.Handler) http.Handler {
	// Excluded paths
	excludedPaths := map[string]bool{
		"/":                true,
		"/health":          true,
		"/docs":            true,
		"/openapi.json":    true,
		"/redoc":           true,
		"/api/auth/status": true,
		"/api/config":      true,
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		password := utils.GetSecretFromEnv("OPEN_NOTEBOOK_PASSWORD")

		// Skip authentication if no password is set
		if password == "" {
			next.ServeHTTP(w, r)
			return
		}

		// Skip authentication for excluded paths
		if excludedPaths[r.URL.Path] {
			next.ServeHTTP(w, r)
			return
		}

		// Skip authentication for CORS preflight requests (OPTIONS)
		if r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}

		// Check authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			respondUnauthorized(w, "Missing authorization header")
			return
		}

		// Expected format: "Bearer {password}"
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			respondUnauthorized(w, "Invalid authorization header format")
			return
		}

		if parts[1] != password {
			respondUnauthorized(w, "Invalid password")
			return
		}

		next.ServeHTTP(w, r)
	})
}

func respondUnauthorized(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("WWW-Authenticate", "Bearer")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"detail": msg,
	})
}
