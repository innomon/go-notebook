package domain

import (
	"context"
	"go-notebook/internal/db"
)

// ContentSettings represents processing and embedding strategy singletons
type ContentSettings struct {
	DefaultContentProcessingEngineDoc string   `json:"default_content_processing_engine_doc"`
	DefaultContentProcessingEngineURL string   `json:"default_content_processing_engine_url"`
	DefaultEmbeddingOption            string   `json:"default_embedding_option"`
	AutoDeleteFiles                   string   `json:"auto_delete_files"`
	YoutubePreferredLanguages         []string `json:"youtube_preferred_languages"`
}

// DefaultPrompts represents the instruction templates for LLM transformations
type DefaultPrompts struct {
	TransformationInstructions string `json:"transformation_instructions"`
}

// GetContentSettings fetches the ContentSettings singleton, initializing defaults if absent
func GetContentSettings(ctx context.Context) (*ContentSettings, error) {
	recordID := "open_notebook:content_settings"

	results, err := db.RepoQuery[ContentSettings](ctx, "SELECT * FROM ONLY $id;", map[string]any{"id": recordID})
	if err != nil {
		// Fallback to query errors or empty table
		return nil, err
	}

	if results == nil {
		// Initialize with default values
		defaults := &ContentSettings{
			DefaultContentProcessingEngineDoc: "auto",
			DefaultContentProcessingEngineURL: "auto",
			DefaultEmbeddingOption:            "ask",
			AutoDeleteFiles:                   "yes",
			YoutubePreferredLanguages:         []string{"en", "pt", "es", "de", "nl", "en-GB", "fr", "hi", "ja"},
		}
		// Save it
		err = UpdateContentSettings(ctx, defaults)
		if err != nil {
			return nil, err
		}
		return defaults, nil
	}

	return results, nil
}

// UpdateContentSettings upserts the ContentSettings singleton record
func UpdateContentSettings(ctx context.Context, settings *ContentSettings) error {
	recordID := "open_notebook:content_settings"
	data := map[string]any{
		"default_content_processing_engine_doc": settings.DefaultContentProcessingEngineDoc,
		"default_content_processing_engine_url": settings.DefaultContentProcessingEngineURL,
		"default_embedding_option":            settings.DefaultEmbeddingOption,
		"auto_delete_files":                   settings.AutoDeleteFiles,
		"youtube_preferred_languages":         settings.YoutubePreferredLanguages,
	}

	_, err := db.RepoUpsert[ContentSettings](ctx, "content_settings", recordID, data, true)
	return err
}

// GetDefaultPrompts fetches the DefaultPrompts singleton
func GetDefaultPrompts(ctx context.Context) (*DefaultPrompts, error) {
	recordID := "open_notebook:default_prompts"

	results, err := db.RepoQuery[DefaultPrompts](ctx, "SELECT * FROM ONLY $id;", map[string]any{"id": recordID})
	if err != nil {
		return nil, err
	}

	if results == nil {
		defaults := &DefaultPrompts{
			TransformationInstructions: "",
		}
		err = UpdateDefaultPrompts(ctx, defaults)
		if err != nil {
			return nil, err
		}
		return defaults, nil
	}

	return results, nil
}

// UpdateDefaultPrompts upserts the DefaultPrompts singleton record
func UpdateDefaultPrompts(ctx context.Context, prompts *DefaultPrompts) error {
	recordID := "open_notebook:default_prompts"
	data := map[string]any{
		"transformation_instructions": prompts.TransformationInstructions,
	}

	_, err := db.RepoUpsert[DefaultPrompts](ctx, "default_prompts", recordID, data, true)
	return err
}
