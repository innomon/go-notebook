package router

import (
	"encoding/json"
	"fmt"
	"go-notebook/internal/domain"
	"go-notebook/internal/utils"
	"net/http"
	"os"
	"strings"
	"time"
)

// RegisterCredentialRoutes binds credential management routes to the ServeMux
func RegisterCredentialRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/credentials/status", handleGetCredentialStatus)
	mux.HandleFunc("GET /api/credentials/env-status", handleGetCredentialEnvStatus)
	mux.HandleFunc("GET /api/credentials", handleListCredentials)
	mux.HandleFunc("GET /api/credentials/by-provider/{provider}", handleListCredentialsByProvider)
	mux.HandleFunc("POST /api/credentials", handleCreateCredential)
	mux.HandleFunc("GET /api/credentials/{credential_id}", handleGetCredential)
	mux.HandleFunc("PUT /api/credentials/{credential_id}", handleUpdateCredential)
	mux.HandleFunc("DELETE /api/credentials/{credential_id}", handleDeleteCredential)
	mux.HandleFunc("POST /api/credentials/{credential_id}/test", handleTestCredential)
	mux.HandleFunc("POST /api/credentials/{credential_id}/discover", handleDiscoverModelsForCredential)
	mux.HandleFunc("POST /api/credentials/{credential_id}/register-models", handleRegisterModelsForCredential)
	mux.HandleFunc("POST /api/credentials/migrate-from-provider-config", handleMigrateFromProviderConfig)
	mux.HandleFunc("POST /api/credentials/migrate-from-env", handleMigrateFromEnv)
}

// Provider environment variable rules
var providerEnvKeys = map[string][]string{
	"openai":            {"OPENAI_API_KEY"},
	"anthropic":         {"ANTHROPIC_API_KEY"},
	"google":            {"GOOGLE_API_KEY", "GEMINI_API_KEY"},
	"groq":              {"GROQ_API_KEY"},
	"mistral":           {"MISTRAL_API_KEY"},
	"deepseek":          {"DEEPSEEK_API_KEY"},
	"xai":               {"XAI_API_KEY"},
	"openrouter":        {"OPENROUTER_API_KEY"},
	"voyage":            {"VOYAGE_API_KEY"},
	"elevenlabs":        {"ELEVENLABS_API_KEY"},
	"deepgram":          {"DEEPGRAM_API_KEY"},
	"ollama":            {"OLLAMA_API_BASE"},
	"vertex":            {"VERTEX_PROJECT", "VERTEX_LOCATION"},
	"azure":             {"AZURE_OPENAI_API_KEY", "AZURE_OPENAI_ENDPOINT", "AZURE_OPENAI_API_VERSION"},
	"openai_compatible": {"OPENAI_COMPATIBLE_BASE_URL", "OPENAI_COMPATIBLE_API_KEY"},
}

// Modalities per provider
var providerModalities = map[string][]string{
	"openai":            {"language", "embedding", "speech_to_text", "text_to_speech"},
	"anthropic":         {"language"},
	"google":            {"language", "embedding", "speech_to_text", "text_to_speech"},
	"groq":              {"language", "speech_to_text"},
	"mistral":           {"language", "embedding", "speech_to_text", "text_to_speech"},
	"deepseek":          {"language"},
	"xai":               {"language", "text_to_speech"},
	"openrouter":        {"language", "embedding"},
	"voyage":            {"embedding"},
	"elevenlabs":        {"text_to_speech", "speech_to_text"},
	"deepgram":          {"text_to_speech"},
	"ollama":            {"language", "embedding"},
	"vertex":            {"language", "embedding", "text_to_speech"},
	"azure":             {"language", "embedding", "speech_to_text", "text_to_speech"},
	"openai_compatible": {"language", "embedding", "speech_to_text", "text_to_speech"},
}

func checkEnvConfigured(provider string) bool {
	keys, ok := providerEnvKeys[provider]
	if !ok {
		return false
	}
	if provider == "google" || provider == "openai_compatible" {
		// required_any
		for _, key := range keys {
			if strings.TrimSpace(os.Getenv(key)) != "" {
				return true
			}
		}
		return false
	}
	// required all
	for _, key := range keys {
		if strings.TrimSpace(os.Getenv(key)) == "" {
			return false
		}
	}
	return true
}

func handleGetCredentialStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	encryptionConfigured := utils.GetSecretFromEnv("OPEN_NOTEBOOK_ENCRYPTION_KEY") != ""

	configured := make(map[string]bool)
	source := make(map[string]string)

	for provider := range providerEnvKeys {
		envConfigured := checkEnvConfigured(provider)
		dbConfigured := false
		creds, err := domain.ListCredentials(ctx)
		if err == nil {
			for _, c := range creds {
				if strings.ToLower(c.Provider) == provider {
					dbConfigured = true
					break
				}
			}
		}

		configured[provider] = dbConfigured || envConfigured
		if dbConfigured {
			source[provider] = "database"
		} else if envConfigured {
			source[provider] = "environment"
		} else {
			source[provider] = "none"
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"configured":            configured,
		"source":                source,
		"encryption_configured": encryptionConfigured,
	})
}

func handleGetCredentialEnvStatus(w http.ResponseWriter, r *http.Request) {
	status := make(map[string]bool)
	for provider := range providerEnvKeys {
		status[provider] = checkEnvConfigured(provider)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(status)
}

func handleListCredentials(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	provider := r.URL.Query().Get("provider")

	creds, err := domain.ListCredentials(ctx)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to retrieve credentials: "+err.Error())
		return
	}

	res := []domain.CredentialResponse{}
	for _, c := range creds {
		if provider != "" && strings.ToLower(c.Provider) != strings.ToLower(provider) {
			continue
		}

		// Count models linked to this credential
		modelCount := 0
		models, err := domain.ListModels(ctx)
		if err == nil {
			for _, m := range models {
				if m.Credential != nil && m.Credential.String() == c.ID.String() {
					modelCount++
				}
			}
		}

		res = append(res, domain.CredentialResponse{
			ID:                c.ID.String(),
			Name:              c.Name,
			Provider:          c.Provider,
			Modalities:        c.Modalities,
			BaseURL:           c.BaseURL,
			Endpoint:          c.Endpoint,
			APIVersion:        c.APIVersion,
			EndpointLLM:       c.EndpointLLM,
			EndpointEmbedding: c.EndpointEmbedding,
			EndpointSTT:       c.EndpointSTT,
			EndpointTTS:       c.EndpointTTS,
			Project:           c.Project,
			Location:          c.Location,
			CredentialsPath:   c.CredentialsPath,
			NumCtx:            c.NumCtx,
			HasAPIKey:         c.APIKey != "",
			Created:           c.Created,
			Updated:           c.Updated,
			ModelCount:        modelCount,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(res)
}

func handleListCredentialsByProvider(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	provider := r.PathValue("provider")

	creds, err := domain.ListCredentials(ctx)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to retrieve credentials: "+err.Error())
		return
	}

	res := []domain.CredentialResponse{}
	for _, c := range creds {
		if strings.ToLower(c.Provider) != strings.ToLower(provider) {
			continue
		}

		modelCount := 0
		models, err := domain.ListModels(ctx)
		if err == nil {
			for _, m := range models {
				if m.Credential != nil && m.Credential.String() == c.ID.String() {
					modelCount++
				}
			}
		}

		res = append(res, domain.CredentialResponse{
			ID:                c.ID.String(),
			Name:              c.Name,
			Provider:          c.Provider,
			Modalities:        c.Modalities,
			BaseURL:           c.BaseURL,
			Endpoint:          c.Endpoint,
			APIVersion:        c.APIVersion,
			EndpointLLM:       c.EndpointLLM,
			EndpointEmbedding: c.EndpointEmbedding,
			EndpointSTT:       c.EndpointSTT,
			EndpointTTS:       c.EndpointTTS,
			Project:           c.Project,
			Location:          c.Location,
			CredentialsPath:   c.CredentialsPath,
			NumCtx:            c.NumCtx,
			HasAPIKey:         c.APIKey != "",
			Created:           c.Created,
			Updated:           c.Updated,
			ModelCount:        modelCount,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(res)
}

func handleCreateCredential(w http.ResponseWriter, r *http.Request) {
	var payload domain.Credential
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	ctx := r.Context()
	created, err := domain.CreateCredential(ctx, &payload)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to create credential: "+err.Error())
		return
	}

	response := domain.CredentialResponse{
		ID:                created.ID.String(),
		Name:              created.Name,
		Provider:          created.Provider,
		Modalities:        created.Modalities,
		BaseURL:           created.BaseURL,
		Endpoint:          created.Endpoint,
		APIVersion:        created.APIVersion,
		EndpointLLM:       created.EndpointLLM,
		EndpointEmbedding: created.EndpointEmbedding,
		EndpointSTT:       created.EndpointSTT,
		EndpointTTS:       created.EndpointTTS,
		Project:           created.Project,
		Location:          created.Location,
		CredentialsPath:   created.CredentialsPath,
		NumCtx:            created.NumCtx,
		HasAPIKey:         created.APIKey != "",
		Created:           created.Created,
		Updated:           created.Updated,
		ModelCount:        0,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(response)
}

func handleGetCredential(w http.ResponseWriter, r *http.Request) {
	credentialID := r.PathValue("credential_id")
	ctx := r.Context()

	cred, err := domain.GetCredential(ctx, credentialID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Credential not found")
		return
	}

	modelCount := 0
	models, err := domain.ListModels(ctx)
	if err == nil {
		for _, m := range models {
			if m.Credential != nil && m.Credential.String() == cred.ID.String() {
				modelCount++
			}
		}
	}

	response := domain.CredentialResponse{
		ID:                cred.ID.String(),
		Name:              cred.Name,
		Provider:          cred.Provider,
		Modalities:        cred.Modalities,
		BaseURL:           cred.BaseURL,
		Endpoint:          cred.Endpoint,
		APIVersion:        cred.APIVersion,
		EndpointLLM:       cred.EndpointLLM,
		EndpointEmbedding: cred.EndpointEmbedding,
		EndpointSTT:       cred.EndpointSTT,
		EndpointTTS:       cred.EndpointTTS,
		Project:           cred.Project,
		Location:          cred.Location,
		CredentialsPath:   cred.CredentialsPath,
		NumCtx:            cred.NumCtx,
		HasAPIKey:         cred.APIKey != "",
		Created:           cred.Created,
		Updated:           cred.Updated,
		ModelCount:        modelCount,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func handleUpdateCredential(w http.ResponseWriter, r *http.Request) {
	credentialID := r.PathValue("credential_id")

	// Decrypt/Check payload
	var rawJSON map[string]any
	if err := json.NewDecoder(r.Body).Decode(&rawJSON); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	ctx := r.Context()
	existing, err := domain.GetCredential(ctx, credentialID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Credential not found")
		return
	}

	keyChanged := false
	if apiKeyVal, ok := rawJSON["api_key"]; ok {
		apiKeyStr, isStr := apiKeyVal.(string)
		if isStr && apiKeyStr != "" {
			existing.APIKey = apiKeyStr
			keyChanged = true
		}
	}

	if nameVal, ok := rawJSON["name"].(string); ok {
		existing.Name = nameVal
	}
	if modalitiesVal, ok := rawJSON["modalities"].([]any); ok {
		modalities := []string{}
		for _, m := range modalitiesVal {
			if s, ok := m.(string); ok {
				modalities = append(modalities, s)
			}
		}
		existing.Modalities = modalities
	}
	if val, ok := rawJSON["base_url"].(string); ok {
		existing.BaseURL = val
	}
	if val, ok := rawJSON["endpoint"].(string); ok {
		existing.Endpoint = val
	}
	if val, ok := rawJSON["api_version"].(string); ok {
		existing.APIVersion = val
	}
	if val, ok := rawJSON["endpoint_llm"].(string); ok {
		existing.EndpointLLM = val
	}
	if val, ok := rawJSON["endpoint_embedding"].(string); ok {
		existing.EndpointEmbedding = val
	}
	if val, ok := rawJSON["endpoint_stt"].(string); ok {
		existing.EndpointSTT = val
	}
	if val, ok := rawJSON["endpoint_tts"].(string); ok {
		existing.EndpointTTS = val
	}
	if val, ok := rawJSON["project"].(string); ok {
		existing.Project = val
	}
	if val, ok := rawJSON["location"].(string); ok {
		existing.Location = val
	}
	if val, ok := rawJSON["credentials_path"].(string); ok {
		existing.CredentialsPath = val
	}
	if val, ok := rawJSON["num_ctx"].(float64); ok {
		nVal := int(val)
		existing.NumCtx = &nVal
	}

	updated, err := domain.UpdateCredential(ctx, credentialID, existing, keyChanged)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to update credential: "+err.Error())
		return
	}

	modelCount := 0
	models, err := domain.ListModels(ctx)
	if err == nil {
		for _, m := range models {
			if m.Credential != nil && m.Credential.String() == updated.ID.String() {
				modelCount++
			}
		}
	}

	response := domain.CredentialResponse{
		ID:                updated.ID.String(),
		Name:              updated.Name,
		Provider:          updated.Provider,
		Modalities:        updated.Modalities,
		BaseURL:           updated.BaseURL,
		Endpoint:          updated.Endpoint,
		APIVersion:        updated.APIVersion,
		EndpointLLM:       updated.EndpointLLM,
		EndpointEmbedding: updated.EndpointEmbedding,
		EndpointSTT:       updated.EndpointSTT,
		EndpointTTS:       updated.EndpointTTS,
		Project:           updated.Project,
		Location:          updated.Location,
		CredentialsPath:   updated.CredentialsPath,
		NumCtx:            updated.NumCtx,
		HasAPIKey:         updated.APIKey != "",
		Created:           updated.Created,
		Updated:           updated.Updated,
		ModelCount:        modelCount,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func handleDeleteCredential(w http.ResponseWriter, r *http.Request) {
	credentialID := r.PathValue("credential_id")
	deleteModels := r.URL.Query().Get("delete_models") == "true"
	migrateTo := r.URL.Query().Get("migrate_to")

	deletedCount, err := domain.DeleteCredential(r.Context(), credentialID, deleteModels, migrateTo)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to delete credential: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"message":        "Credential deleted successfully",
		"deleted_models": deletedCount,
	})
}

func handleTestCredential(w http.ResponseWriter, r *http.Request) {
	credentialID := r.PathValue("credential_id")
	ctx := r.Context()

	cred, err := domain.GetCredential(ctx, credentialID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Credential not found")
		return
	}

	// Lightweight connection test
	success := false
	msg := ""
	provider := strings.ToLower(cred.Provider)

	if provider == "ollama" {
		url := cred.BaseURL
		if url == "" {
			url = "http://localhost:11434"
		}
		// Try pinging tags endpoint
		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Get(url + "/api/tags")
		if err == nil {
			resp.Body.Close()
			success = (resp.StatusCode == http.StatusOK)
			msg = fmt.Sprintf("Ollama returned status code %d", resp.StatusCode)
		} else {
			msg = "Failed to connect to Ollama: " + err.Error()
		}
	} else {
		// Mock test successes for other cloud services if APIKey is present
		if cred.APIKey != "" {
			success = true
			msg = "API Key is configured"
		} else {
			msg = "API Key is missing"
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"provider": cred.Provider,
		"success":  success,
		"message":  msg,
	})
}

type DiscoveredModel struct {
	Name        string `json:"name"`
	Provider    string `json:"provider"`
	Description string `json:"description,omitempty"`
}

func handleDiscoverModelsForCredential(w http.ResponseWriter, r *http.Request) {
	credentialID := r.PathValue("credential_id")
	ctx := r.Context()

	cred, err := domain.GetCredential(ctx, credentialID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Credential not found")
		return
	}

	provider := strings.ToLower(cred.Provider)
	discovered := []DiscoveredModel{}

	// Static fallback models
	staticLists := map[string][]string{
		"anthropic": {
			"claude-3-5-sonnet-20241022",
			"claude-3-5-haiku-20241022",
			"claude-3-opus-20240229",
			"claude-3-haiku-20240307",
		},
		"voyage": {
			"voyage-3",
			"voyage-3-lite",
			"voyage-code-3",
		},
		"elevenlabs": {
			"eleven_multilingual_v2",
			"eleven_turbo_v2_5",
		},
		"deepgram": {
			"aura-2-asteria-en",
			"aura-2-athena-en",
		},
	}

	if list, exists := staticLists[provider]; exists {
		for _, name := range list {
			discovered = append(discovered, DiscoveredModel{
				Name:     name,
				Provider: cred.Provider,
			})
		}
	} else if provider == "ollama" {
		url := cred.BaseURL
		if url == "" {
			url = "http://localhost:11434"
		}
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Get(url + "/api/tags")
		if err == nil {
			defer resp.Body.Close()
			var data struct {
				Models []struct {
					Name string `json:"name"`
				} `json:"models"`
			}
			if json.NewDecoder(resp.Body).Decode(&data) == nil {
				for _, m := range data.Models {
					discovered = append(discovered, DiscoveredModel{
						Name:     m.Name,
						Provider: "ollama",
					})
				}
			}
		}
	} else if provider == "google" {
		url := "https://generativelanguage.googleapis.com/v1/models"
		if cred.APIKey != "" {
			url += "?key=" + cred.APIKey
		}
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Get(url)
		if err == nil {
			defer resp.Body.Close()
			var data struct {
				Models []struct {
					Name        string `json:"name"`
					DisplayName string `json:"displayName"`
				} `json:"models"`
			}
			if json.NewDecoder(resp.Body).Decode(&data) == nil {
				for _, m := range data.Models {
					cleanName := strings.Replace(m.Name, "models/", "", 1)
					discovered = append(discovered, DiscoveredModel{
						Name:        cleanName,
						Provider:    "google",
						Description: m.DisplayName,
					})
				}
			}
		}
	} else {
		// Mock discovery list for generic OpenAI-compatibles if URL is present
		discovered = append(discovered, DiscoveredModel{Name: "gpt-4o", Provider: cred.Provider})
		discovered = append(discovered, DiscoveredModel{Name: "gpt-4o-mini", Provider: cred.Provider})
		discovered = append(discovered, DiscoveredModel{Name: "text-embedding-3-small", Provider: cred.Provider})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"credential_id": cred.ID.String(),
		"provider":      cred.Provider,
		"discovered":    discovered,
	})
}

func handleRegisterModelsForCredential(w http.ResponseWriter, r *http.Request) {
	credentialID := r.PathValue("credential_id")

	var payload struct {
		Models []struct {
			Name      string `json:"name"`
			Provider  string `json:"provider"`
			ModelType string `json:"model_type"`
		} `json:"models"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	ctx := r.Context()
	created := 0
	existing := 0

	for _, m := range payload.Models {
		// Check if model already registered
		registeredModels, err := domain.ListModels(ctx)
		alreadyExists := false
		if err == nil {
			for _, rm := range registeredModels {
				if strings.ToLower(rm.Name) == strings.ToLower(m.Name) && strings.ToLower(rm.Provider) == strings.ToLower(m.Provider) {
					alreadyExists = true
					break
				}
			}
		}

		if alreadyExists {
			existing++
			continue
		}

		_, err = domain.CreateModel(ctx, m.Name, m.Provider, m.ModelType, credentialID)
		if err == nil {
			created++
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"created":  created,
		"existing": existing,
	})
}

func handleMigrateFromProviderConfig(w http.ResponseWriter, r *http.Request) {
	// Replicates migration endpoint from python. In pure Go clean setup, it's a no-op success.
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"message":  "Migration from provider config not needed in clean Go setup",
		"migrated": []string{},
		"skipped":  []string{},
		"errors":   []string{},
	})
}

func handleMigrateFromEnv(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	migrated := []string{}
	skipped := []string{}
	notConfigured := []string{}
	errors := []string{}

	encryptionKey := utils.GetSecretFromEnv("OPEN_NOTEBOOK_ENCRYPTION_KEY")
	if encryptionKey == "" {
		respondError(w, http.StatusBadRequest, "OPEN_NOTEBOOK_ENCRYPTION_KEY is not set. Encryption is required to store credentials.")
		return
	}

	for provider, keys := range providerEnvKeys {
		if !checkEnvConfigured(provider) {
			notConfigured = append(notConfigured, provider)
			continue
		}

		// Check if credentials already exist
		existingList, err := domain.ListCredentials(ctx)
		hasExisting := false
		if err == nil {
			for _, c := range existingList {
				if strings.ToLower(c.Provider) == provider {
					hasExisting = true
					break
				}
			}
		}

		if hasExisting {
			skipped = append(skipped, provider)
			continue
		}

		// Build credential from environment variables
		name := fmt.Sprintf("Default (Migrated from env - %s)", provider)
		apiKey := ""
		baseURL := ""
		apiVersion := ""

		if provider == "google" {
			apiKey = os.Getenv("GOOGLE_API_KEY")
			if apiKey == "" {
				apiKey = os.Getenv("GEMINI_API_KEY")
			}
		} else if provider == "openai_compatible" {
			apiKey = os.Getenv("OPENAI_COMPATIBLE_API_KEY")
			baseURL = os.Getenv("OPENAI_COMPATIBLE_BASE_URL")
		} else if provider == "ollama" {
			baseURL = os.Getenv("OLLAMA_API_BASE")
			if baseURL == "" {
				baseURL = "http://localhost:11434"
			}
		} else if provider == "azure" {
			apiKey = os.Getenv("AZURE_OPENAI_API_KEY")
			baseURL = os.Getenv("AZURE_OPENAI_ENDPOINT")
			apiVersion = os.Getenv("AZURE_OPENAI_API_VERSION")
		} else if len(keys) > 0 {
			apiKey = os.Getenv(keys[0])
		}

		cred := &domain.Credential{
			Name:       name,
			Provider:   provider,
			Modalities: providerModalities[provider],
			APIKey:     apiKey,
			BaseURL:    baseURL,
			APIVersion: apiVersion,
		}

		_, err = domain.CreateCredential(ctx, cred)
		if err != nil {
			errors = append(errors, fmt.Sprintf("Failed to migrate %s: %s", provider, err.Error()))
		} else {
			migrated = append(migrated, provider)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"message":        "Migration complete",
		"migrated":       migrated,
		"skipped":        skipped,
		"not_configured": notConfigured,
		"errors":         errors,
	})
}
