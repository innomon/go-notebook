package domain

import (
	"context"
	"errors"
	"fmt"
	"go-notebook/internal/db"
	"go-notebook/internal/utils"
	"time"

	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// Credential represents a provider account config with encrypted API key
type Credential struct {
	ID                *models.RecordID `json:"id,omitempty"`
	Name              string           `json:"name"`
	Provider          string           `json:"provider"`
	Modalities        []string         `json:"modalities"`
	APIKey            string           `json:"api_key,omitempty"`
	BaseURL           string           `json:"base_url,omitempty"`
	Endpoint          string           `json:"endpoint,omitempty"`
	APIVersion        string           `json:"api_version,omitempty"`
	EndpointLLM       string           `json:"endpoint_llm,omitempty"`
	EndpointEmbedding string           `json:"endpoint_embedding,omitempty"`
	EndpointSTT       string           `json:"endpoint_stt,omitempty"`
	EndpointTTS       string           `json:"endpoint_tts,omitempty"`
	Project           string           `json:"project,omitempty"`
	Location          string           `json:"location,omitempty"`
	CredentialsPath   string           `json:"credentials_path,omitempty"`
	NumCtx            *int             `json:"num_ctx,omitempty"`
	Created           time.Time        `json:"created,omitempty"`
	Updated           time.Time        `json:"updated,omitempty"`
}

// CredentialResponse represents a Credential for API serialization
type CredentialResponse struct {
	ID                string    `json:"id"`
	Name              string    `json:"name"`
	Provider          string    `json:"provider"`
	Modalities        []string  `json:"modalities"`
	BaseURL           string    `json:"base_url,omitempty"`
	Endpoint          string    `json:"endpoint,omitempty"`
	APIVersion        string    `json:"api_version,omitempty"`
	EndpointLLM       string    `json:"endpoint_llm,omitempty"`
	EndpointEmbedding string    `json:"endpoint_embedding,omitempty"`
	EndpointSTT       string    `json:"endpoint_stt,omitempty"`
	EndpointTTS       string    `json:"endpoint_tts,omitempty"`
	Project           string    `json:"project,omitempty"`
	Location          string    `json:"location,omitempty"`
	CredentialsPath   string    `json:"credentials_path,omitempty"`
	NumCtx            *int      `json:"num_ctx,omitempty"`
	HasAPIKey         bool      `json:"has_api_key"`
	Created           time.Time `json:"created"`
	Updated           time.Time `json:"updated"`
	ModelCount        int       `json:"model_count"`
	DecryptionError   string    `json:"decryption_error,omitempty"`
}

// Model represents a registered LLM/STT/TTS model
type Model struct {
	ID         *models.RecordID `json:"id,omitempty"`
	Name       string           `json:"name"`
	Provider   string           `json:"provider"`
	Type       string           `json:"type"` // "language", "embedding", "speech_to_text", "text_to_speech"
	Credential *models.RecordID `json:"credential,omitempty"`
	Created    time.Time        `json:"created,omitempty"`
	Updated    time.Time        `json:"updated,omitempty"`
}

// DefaultModels represents the default chosen models for various tasks
type DefaultModels struct {
	DefaultChatModel           string `json:"default_chat_model,omitempty"`
	DefaultTransformationModel string `json:"default_transformation_model,omitempty"`
	LargeContextModel          string `json:"large_context_model,omitempty"`
	DefaultTextToSpeechModel   string `json:"default_text_to_speech_model,omitempty"`
	DefaultSpeechToTextModel   string `json:"default_speech_to_text_model,omitempty"`
	DefaultEmbeddingModel      string `json:"default_embedding_model,omitempty"`
	DefaultToolsModel          string `json:"default_tools_model,omitempty"`
}

// GetCredential retrieves a credential and decrypts its API key
func GetCredential(ctx context.Context, id string) (*Credential, error) {
	recordID := db.EnsureRecordID("credential", id)
	cred, err := db.RepoQuery[Credential](ctx, "SELECT * FROM ONLY $id;", map[string]any{"id": recordID})
	if err != nil {
		return nil, err
	}
	if cred == nil || cred.ID == nil {
		return nil, errors.New("credential not found")
	}
	if cred.APIKey != "" {
		decrypted, err := utils.DecryptValue(cred.APIKey)
		if err == nil {
			cred.APIKey = decrypted
		} else {
			// Do not leak encrypted ciphertext if decryption fails
			cred.APIKey = ""
		}
	}
	return cred, nil
}

// ListCredentials lists all credentials, decrypting keys
func ListCredentials(ctx context.Context) ([]Credential, error) {
	results, err := db.RepoQuery[[]Credential](ctx, "SELECT * FROM credential ORDER BY created ASC;", nil)
	if err != nil {
		return nil, err
	}
	if results == nil {
		return []Credential{}, nil
	}

	creds := *results
	for i := range creds {
		if creds[i].APIKey != "" {
			decrypted, err := utils.DecryptValue(creds[i].APIKey)
			if err == nil {
				creds[i].APIKey = decrypted
			} else {
				creds[i].APIKey = ""
			}
		}
	}
	return creds, nil
}

// CreateCredential encrypts the API key and creates a credential
func CreateCredential(ctx context.Context, c *Credential) (*Credential, error) {
	if c.Name == "" || c.Provider == "" {
		return nil, errors.New("name and provider are required")
	}

	encryptedKey := ""
	if c.APIKey != "" {
		enc, err := utils.EncryptValue(c.APIKey)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt API key: %w", err)
		}
		encryptedKey = enc
	}

	data := map[string]any{
		"name":               c.Name,
		"provider":           c.Provider,
		"modalities":         c.Modalities,
		"api_key":            encryptedKey,
		"base_url":           c.BaseURL,
		"endpoint":           c.Endpoint,
		"api_version":        c.APIVersion,
		"endpoint_llm":       c.EndpointLLM,
		"endpoint_embedding": c.EndpointEmbedding,
		"endpoint_stt":       c.EndpointSTT,
		"endpoint_tts":       c.EndpointTTS,
		"project":            c.Project,
		"location":           c.Location,
		"credentials_path":   c.CredentialsPath,
		"num_ctx":            c.NumCtx,
	}

	created, err := db.RepoCreate[Credential](ctx, "credential", data)
	if err != nil {
		return nil, err
	}

	// Restore original plain-text APIKey
	created.APIKey = c.APIKey
	return created, nil
}

// UpdateCredential updates a credential encrypting the API key if changed
func UpdateCredential(ctx context.Context, id string, c *Credential, keyChanged bool) (*Credential, error) {
	data := map[string]any{
		"name":               c.Name,
		"modalities":         c.Modalities,
		"base_url":           c.BaseURL,
		"endpoint":           c.Endpoint,
		"api_version":        c.APIVersion,
		"endpoint_llm":       c.EndpointLLM,
		"endpoint_embedding": c.EndpointEmbedding,
		"endpoint_stt":       c.EndpointSTT,
		"endpoint_tts":       c.EndpointTTS,
		"project":            c.Project,
		"location":           c.Location,
		"credentials_path":   c.CredentialsPath,
		"num_ctx":            c.NumCtx,
	}

	if keyChanged {
		if c.APIKey != "" {
			enc, err := utils.EncryptValue(c.APIKey)
			if err != nil {
				return nil, fmt.Errorf("failed to encrypt API key: %w", err)
			}
			data["api_key"] = enc
		} else {
			data["api_key"] = ""
		}
	}

	updated, err := db.RepoUpdate[Credential](ctx, "credential", id, data)
	if err != nil {
		return nil, err
	}

	updated.APIKey = c.APIKey
	return updated, nil
}

// DeleteCredential deletes a credential and unlinks/deletes linked models
func DeleteCredential(ctx context.Context, id string, deleteModels bool, migrateTo string) (deletedModels int, err error) {
	recordID := db.EnsureRecordID("credential", id)

	type ModelID struct {
		ID *models.RecordID `json:"id"`
	}

	// Find models linked to this credential
	modelsList, err := db.RepoQuery[[]ModelID](ctx, "SELECT id FROM model WHERE credential = $id;", map[string]any{"id": recordID})
	if err == nil && modelsList != nil {
		for _, m := range *modelsList {
			if m.ID != nil {
				if deleteModels {
					if err := db.RepoDelete(ctx, m.ID.String()); err == nil {
						deletedModels++
					}
				} else if migrateTo != "" {
					newCred := db.EnsureRecordIDString("credential", migrateTo)
					_, _ = db.RepoUpdate[Model](ctx, "model", m.ID.String(), map[string]any{
						"credential": newCred,
					})
				} else {
					// Unlink
					_, _ = db.RepoUpdate[Model](ctx, "model", m.ID.String(), map[string]any{
						"credential": nil,
					})
				}
			}
		}
	}

	// Delete credential record itself
	err = db.RepoDelete(ctx, recordID.String())
	return deletedModels, err
}

// GetModel retrieves a model by ID
func GetModel(ctx context.Context, id string) (*Model, error) {
	recordID := db.EnsureRecordID("model", id)
	results, err := db.RepoQuery[Model](ctx, "SELECT * FROM ONLY $id;", map[string]any{"id": recordID})
	if err != nil {
		return nil, err
	}
	if results == nil || results.ID == nil {
		return nil, errors.New("model not found")
	}
	return results, nil
}

// ListModels retrieves all models
func ListModels(ctx context.Context) ([]Model, error) {
	results, err := db.RepoQuery[[]Model](ctx, "SELECT * FROM model ORDER BY provider ASC, name ASC;", nil)
	if err != nil {
		return nil, err
	}
	if results == nil {
		return []Model{}, nil
	}
	return *results, nil
}

// ListModelsByProvider retrieves models for a provider
func ListModelsByProvider(ctx context.Context, provider string) ([]Model, error) {
	results, err := db.RepoQuery[[]Model](ctx, "SELECT * FROM model WHERE string::lowercase(provider) = string::lowercase($provider) ORDER BY name ASC;", map[string]any{"provider": provider})
	if err != nil {
		return nil, err
	}
	if results == nil {
		return []Model{}, nil
	}
	return *results, nil
}

// CreateModel registers a new model
func CreateModel(ctx context.Context, name, provider, mType, credID string) (*Model, error) {
	if name == "" || provider == "" || mType == "" {
		return nil, errors.New("name, provider and type are required")
	}

	data := map[string]any{
		"name":     name,
		"provider": provider,
		"type":     mType,
	}

	if credID != "" {
		data["credential"] = db.EnsureRecordID("credential", credID)
	}

	return db.RepoCreate[Model](ctx, "model", data)
}

// DeleteModel deletes a model
func DeleteModel(ctx context.Context, id string) error {
	recordID := db.EnsureRecordID("model", id)
	return db.RepoDelete(ctx, recordID.String())
}

// GetDefaultModels gets DefaultModels singleton
func GetDefaultModels(ctx context.Context) (*DefaultModels, error) {
	recordID := db.EnsureRecordID("open_notebook", "default_models")
	results, err := db.RepoQuery[[]DefaultModels](ctx, "SELECT * FROM open_notebook WHERE id = $id;", map[string]any{"id": recordID})
	if err != nil {
		return nil, err
	}
	if results == nil || len(*results) == 0 {
		// Initialize empty
		defaults := &DefaultModels{}
		err = UpdateDefaultModels(ctx, defaults)
		if err != nil {
			return nil, err
		}
		return defaults, nil
	}
	return &(*results)[0], nil
}

// UpdateDefaultModels updates DefaultModels singleton
func UpdateDefaultModels(ctx context.Context, dm *DefaultModels) error {
	recordID := "open_notebook:default_models"
	data := map[string]any{}
	if dm.DefaultChatModel != "" {
		data["default_chat_model"] = db.EnsureRecordIDString("model", dm.DefaultChatModel)
	} else {
		data["default_chat_model"] = nil
	}
	if dm.DefaultTransformationModel != "" {
		data["default_transformation_model"] = db.EnsureRecordIDString("model", dm.DefaultTransformationModel)
	} else {
		data["default_transformation_model"] = nil
	}
	if dm.LargeContextModel != "" {
		data["large_context_model"] = db.EnsureRecordIDString("model", dm.LargeContextModel)
	} else {
		data["large_context_model"] = nil
	}
	if dm.DefaultTextToSpeechModel != "" {
		data["default_text_to_speech_model"] = db.EnsureRecordIDString("model", dm.DefaultTextToSpeechModel)
	} else {
		data["default_text_to_speech_model"] = nil
	}
	if dm.DefaultSpeechToTextModel != "" {
		data["default_speech_to_text_model"] = db.EnsureRecordIDString("model", dm.DefaultSpeechToTextModel)
	} else {
		data["default_speech_to_text_model"] = nil
	}
	if dm.DefaultEmbeddingModel != "" {
		data["default_embedding_model"] = db.EnsureRecordIDString("model", dm.DefaultEmbeddingModel)
	} else {
		data["default_embedding_model"] = nil
	}
	if dm.DefaultToolsModel != "" {
		data["default_tools_model"] = db.EnsureRecordIDString("model", dm.DefaultToolsModel)
	} else {
		data["default_tools_model"] = nil
	}

	_, err := db.RepoUpsert[DefaultModels](ctx, "default_models", recordID, data, true)
	return err
}
