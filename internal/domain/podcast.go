package domain

import (
	"context"
	"errors"
	"go-notebook/internal/db"

	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// Speaker represents a single speaker configuration within a profile
type Speaker struct {
	Name        string `json:"name"`
	VoiceID     string `json:"voice_id,omitempty"`
	Backstory   string `json:"backstory,omitempty"`
	Personality string `json:"personality,omitempty"`
	VoiceModel  string `json:"voice_model,omitempty"` // Per-speaker model override
}

// SpeakerProfile represents a set of speaker configurations for a podcast format
type SpeakerProfile struct {
	ID          *models.RecordID `json:"id,omitempty"`
	Name        string           `json:"name"`
	Description string           `json:"description,omitempty"`
	VoiceModel  *models.RecordID `json:"voice_model,omitempty"` // Default model record ID for TTS
	Speakers    []Speaker        `json:"speakers"`
	Created     string           `json:"created,omitempty"`
	Updated     string           `json:"updated,omitempty"`
}

// SpeakerProfileResponse represents a speaker profile serialized for the REST API
type SpeakerProfileResponse struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	VoiceModel  string    `json:"voice_model,omitempty"`
	Speakers    []Speaker `json:"speakers"`
	Created     string    `json:"created"`
	Updated     string    `json:"updated"`
}

// EpisodeProfile represents the layout and LLM parameters for generating a podcast episode
type EpisodeProfile struct {
	ID             *models.RecordID `json:"id,omitempty"`
	Name           string           `json:"name"`
	Description    string           `json:"description,omitempty"`
	SpeakerConfig  string           `json:"speaker_config"` // Reference to speaker profile name
	OutlineLLM     *models.RecordID `json:"outline_llm,omitempty"`
	TranscriptLLM  *models.RecordID `json:"transcript_llm,omitempty"`
	Language       string           `json:"language,omitempty"`
	DefaultBriefing string           `json:"default_briefing"`
	NumSegments    int              `json:"num_segments"`
	Created        string           `json:"created,omitempty"`
	Updated        string           `json:"updated,omitempty"`
}

// EpisodeProfileResponse represents an episode profile serialized for the REST API
type EpisodeProfileResponse struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Description    string `json:"description,omitempty"`
	SpeakerConfig  string `json:"speaker_config"`
	OutlineLLM     string `json:"outline_llm,omitempty"`
	TranscriptLLM  string `json:"transcript_llm,omitempty"`
	Language       string `json:"language,omitempty"`
	DefaultBriefing string `json:"default_briefing"`
	NumSegments    int    `json:"num_segments"`
	Created        string `json:"created"`
	Updated        string `json:"updated"`
}

// PodcastEpisode represents a generated podcast episode (mapped to the 'episode' table)
type PodcastEpisode struct {
	ID             *models.RecordID `json:"id,omitempty"`
	Name           string           `json:"name"`
	EpisodeProfile map[string]any   `json:"episode_profile"` // Stored snapshot
	SpeakerProfile map[string]any   `json:"speaker_profile"` // Stored snapshot
	Briefing       string           `json:"briefing"`
	Content        string           `json:"content"`
	AudioFile      string           `json:"audio_file,omitempty"`
	Transcript     map[string]any   `json:"transcript,omitempty"`
	Outline        map[string]any   `json:"outline,omitempty"`
	Command        *models.RecordID `json:"command,omitempty"`
	Created        string           `json:"created,omitempty"`
	Updated        string           `json:"updated,omitempty"`
}

// PodcastEpisodeResponse represents a podcast episode for the REST API
type PodcastEpisodeResponse struct {
	ID             string         `json:"id"`
	Name           string         `json:"name"`
	EpisodeProfile map[string]any `json:"episode_profile"`
	SpeakerProfile map[string]any `json:"speaker_profile"`
	Briefing       string         `json:"briefing"`
	Content        string         `json:"content"`
	AudioFile      string         `json:"audio_file,omitempty"`
	Transcript     map[string]any `json:"transcript,omitempty"`
	Outline        map[string]any `json:"outline,omitempty"`
	CommandID      string         `json:"command_id,omitempty"`
	Created        string         `json:"created"`
	Updated        string         `json:"updated"`
	Status         string         `json:"status,omitempty"`
	ErrorMessage   string         `json:"error_message,omitempty"`
}

// --- Speaker Profile CRUD ---

func GetSpeakerProfile(ctx context.Context, id string) (*SpeakerProfile, error) {
	recordID := db.EnsureRecordIDString("speaker_profile", id)
	return db.RepoQuery[SpeakerProfile](ctx, "SELECT * FROM ONLY $id;", map[string]any{"id": recordID})
}

func GetSpeakerProfileByName(ctx context.Context, name string) (*SpeakerProfile, error) {
	results, err := db.RepoQuery[[]SpeakerProfile](ctx, "SELECT * FROM speaker_profile WHERE name = $name LIMIT 1;", map[string]any{"name": name})
	if err != nil {
		return nil, err
	}
	if results == nil || len(*results) == 0 {
		return nil, errors.New("speaker profile not found by name")
	}
	return &(*results)[0], nil
}

func ListSpeakerProfiles(ctx context.Context) ([]SpeakerProfile, error) {
	results, err := db.RepoQuery[[]SpeakerProfile](ctx, "SELECT * FROM speaker_profile ORDER BY name ASC;", nil)
	if err != nil {
		return nil, err
	}
	if results == nil {
		return []SpeakerProfile{}, nil
	}
	return *results, nil
}

func CreateSpeakerProfile(ctx context.Context, name, description, voiceModelID string, speakers []Speaker) (*SpeakerProfile, error) {
	if name == "" {
		return nil, errors.New("name is required")
	}
	if len(speakers) < 1 || len(speakers) > 4 {
		return nil, errors.New("must have between 1 and 4 speakers")
	}

	data := map[string]any{
		"name":        name,
		"description": description,
		"speakers":    speakers,
	}

	if voiceModelID != "" {
		data["voice_model"] = db.EnsureRecordIDString("model", voiceModelID)
	}

	return db.RepoCreate[SpeakerProfile](ctx, "speaker_profile", data)
}

func UpdateSpeakerProfile(ctx context.Context, id string, name, description *string, voiceModelID *string, speakers []Speaker) (*SpeakerProfile, error) {
	data := map[string]any{}
	if name != nil {
		if *name == "" {
			return nil, errors.New("name cannot be empty")
		}
		data["name"] = *name
	}
	if description != nil {
		data["description"] = *description
	}
	if voiceModelID != nil {
		if *voiceModelID == "" {
			data["voice_model"] = nil
		} else {
			data["voice_model"] = db.EnsureRecordIDString("model", *voiceModelID)
		}
	}
	if speakers != nil {
		if len(speakers) < 1 || len(speakers) > 4 {
			return nil, errors.New("must have between 1 and 4 speakers")
		}
		data["speakers"] = speakers
	}

	return db.RepoUpdate[SpeakerProfile](ctx, "speaker_profile", id, data)
}

func DeleteSpeakerProfile(ctx context.Context, id string) error {
	recordID := db.EnsureRecordIDString("speaker_profile", id)
	return db.RepoDelete(ctx, recordID)
}

// --- Episode Profile CRUD ---

func GetEpisodeProfile(ctx context.Context, id string) (*EpisodeProfile, error) {
	recordID := db.EnsureRecordIDString("episode_profile", id)
	return db.RepoQuery[EpisodeProfile](ctx, "SELECT * FROM ONLY $id;", map[string]any{"id": recordID})
}

func GetEpisodeProfileByName(ctx context.Context, name string) (*EpisodeProfile, error) {
	results, err := db.RepoQuery[[]EpisodeProfile](ctx, "SELECT * FROM episode_profile WHERE name = $name LIMIT 1;", map[string]any{"name": name})
	if err != nil {
		return nil, err
	}
	if results == nil || len(*results) == 0 {
		return nil, errors.New("episode profile not found by name")
	}
	return &(*results)[0], nil
}

func ListEpisodeProfiles(ctx context.Context) ([]EpisodeProfile, error) {
	results, err := db.RepoQuery[[]EpisodeProfile](ctx, "SELECT * FROM episode_profile ORDER BY name ASC;", nil)
	if err != nil {
		return nil, err
	}
	if results == nil {
		return []EpisodeProfile{}, nil
	}
	return *results, nil
}

func CreateEpisodeProfile(ctx context.Context, ep *EpisodeProfile) (*EpisodeProfile, error) {
	if ep.Name == "" || ep.SpeakerConfig == "" {
		return nil, errors.New("name and speaker_config are required")
	}
	if ep.NumSegments < 3 || ep.NumSegments > 20 {
		return nil, errors.New("num_segments must be between 3 and 20")
	}

	data := map[string]any{
		"name":             ep.Name,
		"description":      ep.Description,
		"speaker_config":   ep.SpeakerConfig,
		"language":         ep.Language,
		"default_briefing": ep.DefaultBriefing,
		"num_segments":     ep.NumSegments,
	}

	if ep.OutlineLLM != nil {
		data["outline_llm"] = ep.OutlineLLM
	}
	if ep.TranscriptLLM != nil {
		data["transcript_llm"] = ep.TranscriptLLM
	}

	return db.RepoCreate[EpisodeProfile](ctx, "episode_profile", data)
}

func UpdateEpisodeProfile(ctx context.Context, id string, ep *EpisodeProfile) (*EpisodeProfile, error) {
	data := map[string]any{
		"name":             ep.Name,
		"description":      ep.Description,
		"speaker_config":   ep.SpeakerConfig,
		"language":         ep.Language,
		"default_briefing": ep.DefaultBriefing,
		"num_segments":     ep.NumSegments,
	}

	if ep.OutlineLLM != nil {
		data["outline_llm"] = ep.OutlineLLM
	} else {
		data["outline_llm"] = nil
	}

	if ep.TranscriptLLM != nil {
		data["transcript_llm"] = ep.TranscriptLLM
	} else {
		data["transcript_llm"] = nil
	}

	return db.RepoUpdate[EpisodeProfile](ctx, "episode_profile", id, data)
}

func DeleteEpisodeProfile(ctx context.Context, id string) error {
	recordID := db.EnsureRecordIDString("episode_profile", id)
	return db.RepoDelete(ctx, recordID)
}

// --- Podcast Episode CRUD ---

func GetPodcastEpisode(ctx context.Context, id string) (*PodcastEpisode, error) {
	recordID := db.EnsureRecordIDString("episode", id)
	return db.RepoQuery[PodcastEpisode](ctx, "SELECT * FROM ONLY $id;", map[string]any{"id": recordID})
}

func ListPodcastEpisodes(ctx context.Context) ([]PodcastEpisode, error) {
	results, err := db.RepoQuery[[]PodcastEpisode](ctx, "SELECT * FROM episode ORDER BY created DESC;", nil)
	if err != nil {
		return nil, err
	}
	if results == nil {
		return []PodcastEpisode{}, nil
	}
	return *results, nil
}

func DeletePodcastEpisode(ctx context.Context, id string) error {
	recordID := db.EnsureRecordIDString("episode", id)
	return db.RepoDelete(ctx, recordID)
}
