package router

import (
	_ "embed"
	"net/http"
)

//go:embed languages.json
var languagesJSON []byte

// RegisterLanguagesRoutes binds the languages endpoint to the ServeMux
func RegisterLanguagesRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/languages", handleListLanguages)
}

func handleListLanguages(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(languagesJSON)
}
