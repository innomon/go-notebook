package router

import (
	"context"
	"encoding/json"
	"go-notebook/internal/domain"
	"net/http"
	"os"
	"strings"
)

// RegisterModelRoutes binds model registry routes to the ServeMux
func RegisterModelRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/models", handleListModels)
	mux.HandleFunc("POST /api/models", handleCreateModel)
	mux.HandleFunc("DELETE /api/models/{model_id}", handleDeleteModel)
	mux.HandleFunc("GET /api/models/defaults", handleGetDefaultModels)
	mux.HandleFunc("PUT /api/models/defaults", handleUpdateDefaultModels)
	mux.HandleFunc("GET /api/models/providers", handleGetProviderAvailability)
	mux.HandleFunc("GET /api/models/discover/{provider}", handleDiscoverModels)
	mux.HandleFunc("POST /api/models/sync/{provider}", handleSyncModels)
	mux.HandleFunc("POST /api/models/sync", handleSyncAllModels)
	mux.HandleFunc("GET /api/models/count/{provider}", handleGetProviderModelCount)
	mux.HandleFunc("GET /api/models/by-provider/{provider}", handleGetModelsByProvider)
	mux.HandleFunc("POST /api/models/auto-assign", handleAutoAssignDefaults)
	mux.HandleFunc("POST /api/models/test/{model_id}", handleTestModel)
}

func handleListModels(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	models, err := domain.ListModels(ctx)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to retrieve models: "+err.Error())
		return
	}

	responses := make([]domain.ModelResponse, len(models))
	for i, m := range models {
		responses[i] = m.ToResponse()
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(responses)
}

func handleCreateModel(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Name         string `json:"name"`
		Provider     string `json:"provider"`
		Type         string `json:"type"`
		CredentialID string `json:"credential_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	ctx := r.Context()
	model, err := domain.CreateModel(ctx, payload.Name, payload.Provider, payload.Type, payload.CredentialID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to create model: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(model.ToResponse())
}

func handleDeleteModel(w http.ResponseWriter, r *http.Request) {
	modelID := r.PathValue("model_id")
	if modelID == "" {
		respondError(w, http.StatusBadRequest, "Model ID is required")
		return
	}

	ctx := r.Context()
	err := domain.DeleteModel(ctx, modelID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to delete model: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"message": "Model deleted successfully",
	})
}

func handleGetDefaultModels(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defaults, err := domain.GetDefaultModels(ctx)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to fetch default models: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(defaults)
}

func handleUpdateDefaultModels(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 1. Get existing defaults to merge with
	defaults, err := domain.GetDefaultModels(ctx)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to fetch default models: "+err.Error())
		return
	}

	// 2. Decode partial payload into a map
	var payload map[string]any
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	// 3. Update only provided fields
	for k, v := range payload {
		valStr := ""
		if v != nil {
			if s, ok := v.(string); ok {
				valStr = s
			}
		}

		switch k {
		case "default_chat_model":
			defaults.DefaultChatModel = valStr
		case "default_transformation_model":
			defaults.DefaultTransformationModel = valStr
		case "large_context_model":
			defaults.LargeContextModel = valStr
		case "default_text_to_speech_model":
			defaults.DefaultTextToSpeechModel = valStr
		case "default_speech_to_text_model":
			defaults.DefaultSpeechToTextModel = valStr
		case "default_embedding_model":
			defaults.DefaultEmbeddingModel = valStr
		case "default_tools_model":
			defaults.DefaultToolsModel = valStr
		}
	}

	// 4. Update the merged default models record
	err = domain.UpdateDefaultModels(ctx, defaults)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to update default models: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(defaults)
}

func handleGetProviderAvailability(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	available := []string{}
	unavailable := []string{}
	supportedTypes := make(map[string][]string)

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

		if dbConfigured || envConfigured {
			available = append(available, provider)
			supportedTypes[provider] = providerModalities[provider]
		} else {
			unavailable = append(unavailable, provider)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"available":       available,
		"unavailable":     unavailable,
		"supported_types": supportedTypes,
	})
}

func handleDiscoverModels(w http.ResponseWriter, r *http.Request) {
	provider := r.PathValue("provider")
	if provider == "" {
		respondError(w, http.StatusBadRequest, "Provider is required")
		return
	}

	// Discovers without credentialID by finding the first configured credential or using env vars
	ctx := r.Context()
	creds, err := domain.ListCredentials(ctx)
	apiKey := ""
	baseURL := ""

	if err == nil {
		for _, c := range creds {
			if strings.ToLower(c.Provider) == strings.ToLower(provider) {
				apiKey = c.APIKey
				baseURL = c.BaseURL
				break
			}
		}
	}

	// Fallback to env
	if apiKey == "" {
		if keys, ok := providerEnvKeys[strings.ToLower(provider)]; ok && len(keys) > 0 {
			apiKey = os.Getenv(keys[0])
		}
	}
	if baseURL == "" {
		if strings.ToLower(provider) == "ollama" {
			baseURL = os.Getenv("OLLAMA_API_BASE")
			if baseURL == "" {
				baseURL = "http://localhost:11434"
			}
		}
	}

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

	discovered := []DiscoveredModel{}
	pLower := strings.ToLower(provider)

	if list, exists := staticLists[pLower]; exists {
		for _, name := range list {
			discovered = append(discovered, DiscoveredModel{
				Name:     name,
				Provider: provider,
			})
		}
	} else if pLower == "ollama" {
		client := &http.Client{}
		resp, err := client.Get(baseURL + "/api/tags")
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
	} else if pLower == "google" {
		url := "https://generativelanguage.googleapis.com/v1/models"
		if apiKey != "" {
			url += "?key=" + apiKey
		}
		client := &http.Client{}
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
		// Mock generic OpenAI
		discovered = append(discovered, DiscoveredModel{Name: "gpt-4o", Provider: provider})
		discovered = append(discovered, DiscoveredModel{Name: "gpt-4o-mini", Provider: provider})
		discovered = append(discovered, DiscoveredModel{Name: "text-embedding-3-small", Provider: provider})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(discovered)
}

func handleSyncModels(w http.ResponseWriter, r *http.Request) {
	provider := r.PathValue("provider")
	ctx := r.Context()

	discovered, newCount, existing, err := syncProviderInternal(ctx, provider)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to sync models: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"provider":   provider,
		"discovered": discovered,
		"new":        newCount,
		"existing":   existing,
	})
}

func handleSyncAllModels(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	results := make(map[string]any)
	totalDiscovered := 0
	totalNew := 0

	for provider := range providerEnvKeys {
		discovered, newCount, existing, err := syncProviderInternal(ctx, provider)
		if err == nil && discovered > 0 {
			results[provider] = map[string]any{
				"provider":   provider,
				"discovered": discovered,
				"new":        newCount,
				"existing":   existing,
			}
			totalDiscovered += discovered
			totalNew += newCount
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"results":          results,
		"total_discovered": totalDiscovered,
		"total_new":        totalNew,
	})
}

func syncProviderInternal(ctx context.Context, provider string) (discovered int, newCount int, existing int, err error) {
	// 1. Discover models list
	creds, err := domain.ListCredentials(ctx)
	apiKey := ""
	baseURL := ""
	var cred *domain.Credential

	if err == nil {
		for _, c := range creds {
			if strings.ToLower(c.Provider) == strings.ToLower(provider) {
				cred = &c
				apiKey = c.APIKey
				baseURL = c.BaseURL
				break
			}
		}
	}

	if apiKey == "" {
		if keys, ok := providerEnvKeys[strings.ToLower(provider)]; ok && len(keys) > 0 {
			apiKey = os.Getenv(keys[0])
		}
	}
	if baseURL == "" {
		if strings.ToLower(provider) == "ollama" {
			baseURL = os.Getenv("OLLAMA_API_BASE")
			if baseURL == "" {
				baseURL = "http://localhost:11434"
			}
		}
	}

	if apiKey == "" && strings.ToLower(provider) != "ollama" {
		return 0, 0, 0, nil // Provider not configured
	}

	// Curated list
	names := []string{}
	pLower := strings.ToLower(provider)

	staticLists := map[string][]string{
		"anthropic":  {"claude-3-5-sonnet-20241022", "claude-3-opus-20240229"},
		"voyage":     {"voyage-3", "voyage-3-lite"},
		"elevenlabs": {"eleven_multilingual_v2", "eleven_turbo_v2_5"},
		"deepgram":   {"aura-2-asteria-en"},
	}

	if list, exists := staticLists[pLower]; exists {
		names = list
	} else if pLower == "ollama" {
		client := &http.Client{}
		resp, err := client.Get(baseURL + "/api/tags")
		if err == nil {
			defer resp.Body.Close()
			var data struct {
				Models []struct {
					Name string `json:"name"`
				} `json:"models"`
			}
			if json.NewDecoder(resp.Body).Decode(&data) == nil {
				for _, m := range data.Models {
					names = append(names, m.Name)
				}
			}
		}
	} else if pLower == "google" {
		url := "https://generativelanguage.googleapis.com/v1/models"
		if apiKey != "" {
			url += "?key=" + apiKey
		}
		client := &http.Client{}
		resp, err := client.Get(url)
		if err == nil {
			defer resp.Body.Close()
			var data struct {
				Models []struct {
					Name string `json:"name"`
				} `json:"models"`
			}
			if json.NewDecoder(resp.Body).Decode(&data) == nil {
				for _, m := range data.Models {
					cleanName := strings.Replace(m.Name, "models/", "", 1)
					names = append(names, cleanName)
				}
			}
		}
	} else {
		// Generic OpenAI model templates
		names = []string{"gpt-4o", "gpt-4o-mini", "text-embedding-3-small"}
	}

	discovered = len(names)

	// Get registered models
	registered, err := domain.ListModels(ctx)
	if err != nil {
		return 0, 0, 0, err
	}

	for _, name := range names {
		alreadyExists := false
		for _, rm := range registered {
			if strings.ToLower(rm.Name) == strings.ToLower(name) && strings.ToLower(rm.Provider) == strings.ToLower(provider) {
				alreadyExists = true
				break
			}
		}

		if alreadyExists {
			existing++
			continue
		}

		// Classify type based on name
		mType := "language"
		nLower := strings.ToLower(name)
		if strings.Contains(nLower, "embed") {
			mType = "embedding"
		} else if strings.Contains(nLower, "tts") || strings.Contains(nLower, "eleven") || strings.Contains(nLower, "aura") {
			mType = "text_to_speech"
		} else if strings.Contains(nLower, "stt") || strings.Contains(nLower, "whisper") || strings.Contains(nLower, "scribe") {
			mType = "speech_to_text"
		}

		credID := ""
		if cred != nil {
			credID = cred.ID.String()
		}

		_, err = domain.CreateModel(ctx, name, provider, mType, credID)
		if err == nil {
			newCount++
		}
	}

	return discovered, newCount, existing, nil
}

func handleGetProviderModelCount(w http.ResponseWriter, r *http.Request) {
	provider := r.PathValue("provider")
	ctx := r.Context()

	models, err := domain.ListModels(ctx)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to retrieve models: "+err.Error())
		return
	}

	count := 0
	for _, m := range models {
		if strings.ToLower(m.Provider) == strings.ToLower(provider) {
			count++
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"provider":    provider,
		"model_count": count,
	})
}

func handleGetModelsByProvider(w http.ResponseWriter, r *http.Request) {
	provider := r.PathValue("provider")
	ctx := r.Context()

	models, err := domain.ListModelsByProvider(ctx, provider)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to retrieve models: "+err.Error())
		return
	}

	responses := make([]domain.ModelResponse, len(models))
	for i, m := range models {
		responses[i] = m.ToResponse()
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(responses)
}

var providerPriority = []string{
	"openai",
	"anthropic",
	"google",
	"mistral",
	"groq",
	"deepseek",
	"xai",
	"openrouter",
	"ollama",
	"azure",
	"openai_compatible",
	"dashscope",
	"minimax",
}

var modelPreferences = map[string][]string{
	"openai":    {"gpt-4o", "gpt-4", "gpt-3.5-turbo"},
	"anthropic": {"claude-3-5-sonnet", "claude-3-opus", "claude-3-sonnet"},
	"google":    {"gemini-2.0", "gemini-1.5-pro", "gemini-pro"},
	"mistral":   {"mistral-large", "mixtral"},
	"groq":      {"llama-3.3", "llama-3.1", "mixtral"},
	"dashscope": {"qwen-max", "qwen-plus", "qwen-turbo"},
	"ollama":    {"gemma3", "gemma", "llama3.3", "llama3.2", "llama3.1", "llama3", "llama", "mistral", "qwen", "phi3", "phi", "granite"},
}

func handleAutoAssignDefaults(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	defaults, err := domain.GetDefaultModels(ctx)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to load defaults: "+err.Error())
		return
	}

	allModels, err := domain.ListModels(ctx)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to load registered models: "+err.Error())
		return
	}

	if len(allModels) == 0 {
		// Auto-sync all providers if no models are registered yet
		for provider := range providerEnvKeys {
			_, _, _, _ = syncProviderInternal(ctx, provider)
		}
		// Try listing models again
		allModels, err = domain.ListModels(ctx)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "Failed to load registered models after sync: "+err.Error())
			return
		}
	}

	modelsByType := map[string][]domain.Model{
		"language":       {},
		"embedding":      {},
		"text_to_speech": {},
		"speech_to_text": {},
	}
	for _, m := range allModels {
		modelsByType[m.Type] = append(modelsByType[m.Type], m)
	}

	slotConfigs := []struct {
		SlotName  string
		ModelType string
		Current   string
		Assign    func(string)
	}{
		{"default_chat_model", "language", defaults.DefaultChatModel, func(v string) { defaults.DefaultChatModel = v }},
		{"default_transformation_model", "language", defaults.DefaultTransformationModel, func(v string) { defaults.DefaultTransformationModel = v }},
		{"default_tools_model", "language", defaults.DefaultToolsModel, func(v string) { defaults.DefaultToolsModel = v }},
		{"large_context_model", "language", defaults.LargeContextModel, func(v string) { defaults.LargeContextModel = v }},
		{"default_embedding_model", "embedding", defaults.DefaultEmbeddingModel, func(v string) { defaults.DefaultEmbeddingModel = v }},
		{"default_text_to_speech_model", "text_to_speech", defaults.DefaultTextToSpeechModel, func(v string) { defaults.DefaultTextToSpeechModel = v }},
		{"default_speech_to_text_model", "speech_to_text", defaults.DefaultSpeechToTextModel, func(v string) { defaults.DefaultSpeechToTextModel = v }},
	}

	assigned := make(map[string]string)
	skipped := []string{}
	missing := []string{}

	for _, slot := range slotConfigs {
		if slot.Current != "" {
			skipped = append(skipped, slot.SlotName)
			continue
		}

		available := modelsByType[slot.ModelType]
		if len(available) == 0 {
			missing = append(missing, slot.SlotName)
			continue
		}

		// Filter out highly specialized models (function-calling, guardrails, embeddings)
		// for language slots if other options exist
		if slot.ModelType == "language" {
			generalModels := []domain.Model{}
			for _, m := range available {
				nameLower := strings.ToLower(m.Name)
				if !strings.Contains(nameLower, "function") &&
					!strings.Contains(nameLower, "guard") &&
					!strings.Contains(nameLower, "embed") {
					generalModels = append(generalModels, m)
				}
			}
			if len(generalModels) > 0 {
				available = generalModels
			}
		}

		// Find preferred model
		var best *domain.Model
		for _, provider := range providerPriority {
			providerModels := []domain.Model{}
			for _, m := range available {
				if strings.ToLower(m.Provider) == provider {
					providerModels = append(providerModels, m)
				}
			}

			if len(providerModels) > 0 {
				// Search patterns
				prefs := modelPreferences[provider]
				for _, pattern := range prefs {
					for _, pm := range providerModels {
						if strings.Contains(strings.ToLower(pm.Name), strings.ToLower(pattern)) {
							best = &pm
							break
						}
					}
					if best != nil {
						break
					}
				}

				if best == nil {
					best = &providerModels[0]
				}
				break
			}
		}

		if best == nil && len(available) > 0 {
			best = &available[0]
		}

		if best != nil {
			slot.Assign(best.ID.String())
			assigned[slot.SlotName] = best.ID.String()
		}
	}

	if len(assigned) > 0 {
		_ = domain.UpdateDefaultModels(ctx, defaults)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"assigned": assigned,
		"skipped":  skipped,
		"missing":  missing,
	})
}

func handleTestModel(w http.ResponseWriter, r *http.Request) {
	modelID := r.PathValue("model_id")
	ctx := r.Context()

	model, err := domain.GetModel(ctx, modelID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Model not found")
		return
	}

	// Just a simple status test for the model configuration
	success := false
	msg := ""
	if model.Credential != nil {
		cred, err := domain.GetCredential(ctx, model.Credential.String())
		if err == nil && cred.APIKey != "" {
			success = true
			msg = "Model credential verified"
		} else {
			msg = "Model credential API key is missing or corrupted"
		}
	} else {
		// Env vars check
		if checkEnvConfigured(strings.ToLower(model.Provider)) {
			success = true
			msg = "Provider configured via environment variables"
		} else {
			msg = "Model has no credentials or env vars configured"
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"model_id": modelID,
		"success":  success,
		"message":  msg,
	})
}
