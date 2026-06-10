package middleware

import (
	"net/http"
	"os"
	"strings"
)

// getCORSOrigins parses CORS_ORIGINS environment variable
func getCORSOrigins() []string {
	originsRaw := os.Getenv("CORS_ORIGINS")
	if originsRaw == "" || originsRaw == "*" {
		return []string{"*"}
	}
	parts := strings.Split(originsRaw, ",")
	origins := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			origins = append(origins, trimmed)
		}
	}
	return origins
}

// CORS is a middleware that applies CORS headers to all responses
func CORS(next http.Handler) http.Handler {
	allowedOrigins := getCORSOrigins()
	isWildcard := len(allowedOrigins) == 1 && allowedOrigins[0] == "*"

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		// Determine allowed origin
		var finalOrigin string
		if isWildcard {
			if origin != "" {
				finalOrigin = origin // Reflect origin for wildcard to support credentials
			} else {
				finalOrigin = "*"
			}
		} else {
			for _, allowed := range allowedOrigins {
				if allowed == origin {
					finalOrigin = origin
					break
				}
			}
		}

		if finalOrigin != "" {
			w.Header().Set("Access-Control-Allow-Origin", finalOrigin)
			w.Header().Set("Vary", "Origin")
		}

		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, PATCH, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")

		// Handle preflight OPTIONS request
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
