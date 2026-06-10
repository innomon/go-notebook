package ai

import (
	"context"
	"errors"
	"fmt"
	"go-notebook/internal/domain"
	"go-notebook/internal/utils"
	"os"
	"strings"
)

// GetClientForModel gets an AIClient for a registered model by model ID
func GetClientForModel(ctx context.Context, modelID string) (AIClient, error) {
	if modelID == "" {
		return nil, errors.New("model ID is empty")
	}

	model, err := domain.GetModel(ctx, modelID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch model: %w", err)
	}

	cfg := ClientConfig{
		Provider: model.Provider,
		Model:    model.Name,
	}

	// 1. Resolve credentials
	if model.Credential != nil && model.Credential.String() != "" {
		cred, err := domain.GetCredential(ctx, model.Credential.String())
		if err == nil && cred != nil {
			cfg.APIKey = cred.APIKey
			cfg.BaseURL = cred.BaseURL
			cfg.APIVersion = cred.APIVersion
			cfg.Project = cred.Project
			cfg.Location = cred.Location
			if cred.NumCtx != nil {
				cfg.NumCtx = *cred.NumCtx
			}

			// Modality-specific endpoint mapping (Azure/etc.)
			switch model.Type {
			case "language":
				if cred.EndpointLLM != "" {
					cfg.BaseURL = cred.EndpointLLM
				}
			case "embedding":
				if cred.EndpointEmbedding != "" {
					cfg.BaseURL = cred.EndpointEmbedding
				}
			case "speech_to_text":
				if cred.EndpointSTT != "" {
					cfg.BaseURL = cred.EndpointSTT
				}
			case "text_to_speech":
				if cred.EndpointTTS != "" {
					cfg.BaseURL = cred.EndpointTTS
				}
			}
		}
	}

	// 2. Fall back to environment variable mapping if API key / endpoint is not set
	if cfg.APIKey == "" {
		cfg.APIKey = resolveAPIKeyFromEnv(model.Provider)
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = resolveBaseURLFromEnv(model.Provider)
	}

	return NewClient(cfg), nil
}

// GetClientForDefaultModel resolves the default model for a specific task and returns its AIClient
func GetClientForDefaultModel(ctx context.Context, task string) (AIClient, error) {
	defaults, err := domain.GetDefaultModels(ctx)
	if err != nil {
		return nil, err
	}

	modelID := ""
	switch task {
	case "chat":
		modelID = defaults.DefaultChatModel
	case "transformation":
		modelID = defaults.DefaultTransformationModel
		if modelID == "" {
			modelID = defaults.DefaultChatModel
		}
	case "large_context":
		modelID = defaults.LargeContextModel
		if modelID == "" {
			modelID = defaults.DefaultChatModel
		}
	case "text_to_speech":
		modelID = defaults.DefaultTextToSpeechModel
	case "speech_to_text":
		modelID = defaults.DefaultSpeechToTextModel
	case "embedding":
		modelID = defaults.DefaultEmbeddingModel
	case "tools":
		modelID = defaults.DefaultToolsModel
		if modelID == "" {
			modelID = defaults.DefaultChatModel
		}
	default:
		return nil, fmt.Errorf("unknown task: %s", task)
	}

	if modelID == "" {
		return nil, fmt.Errorf("no default model configured for task: %s", task)
	}

	return GetClientForModel(ctx, modelID)
}

func resolveAPIKeyFromEnv(provider string) string {
	provider = strings.ToLower(provider)
	switch provider {
	case "openai":
		return utils.GetSecretFromEnv("OPENAI_API_KEY")
	case "anthropic":
		return utils.GetSecretFromEnv("ANTHROPIC_API_KEY")
	case "google", "gemini":
		return utils.GetSecretFromEnv("GOOGLE_API_KEY")
	case "groq":
		return utils.GetSecretFromEnv("GROQ_API_KEY")
	case "mistral":
		return utils.GetSecretFromEnv("MISTRAL_API_KEY")
	case "deepseek":
		return utils.GetSecretFromEnv("DEEPSEEK_API_KEY")
	case "xai":
		return utils.GetSecretFromEnv("XAI_API_KEY")
	case "openrouter":
		return utils.GetSecretFromEnv("OPENROUTER_API_KEY")
	case "voyage":
		return utils.GetSecretFromEnv("VOYAGE_API_KEY")
	case "elevenlabs":
		return utils.GetSecretFromEnv("ELEVENLABS_API_KEY")
	case "deepgram":
		return utils.GetSecretFromEnv("DEEPGRAM_API_KEY")
	case "azure":
		return utils.GetSecretFromEnv("AZURE_OPENAI_API_KEY")
	case "openai_compatible", "openai-compatible":
		return utils.GetSecretFromEnv("OPENAI_COMPATIBLE_API_KEY")
	}
	return ""
}

func resolveBaseURLFromEnv(provider string) string {
	provider = strings.ToLower(provider)
	switch provider {
	case "ollama":
		if base := os.Getenv("OLLAMA_API_BASE"); base != "" {
			return base
		}
		return "http://localhost:11434"
	case "azure":
		return os.Getenv("AZURE_OPENAI_ENDPOINT")
	case "openai_compatible", "openai-compatible":
		return os.Getenv("OPENAI_COMPATIBLE_BASE_URL")
	}
	return ""
}
