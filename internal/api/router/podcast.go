package router

import (
	"encoding/json"
	"fmt"
	"go-notebook/internal/db"
	"go-notebook/internal/domain"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// RegisterPodcastRoutes binds podcast, speaker, and episode profile routes to the ServeMux
func RegisterPodcastRoutes(mux *http.ServeMux) {
	// Speaker Profiles
	mux.HandleFunc("GET /api/speaker-profiles", handleListSpeakerProfiles)
	mux.HandleFunc("POST /api/speaker-profiles", handleCreateSpeakerProfile)
	mux.HandleFunc("GET /api/speaker-profiles/{id}", handleGetSpeakerProfile)
	mux.HandleFunc("PUT /api/speaker-profiles/{id}", handleUpdateSpeakerProfile)
	mux.HandleFunc("DELETE /api/speaker-profiles/{id}", handleDeleteSpeakerProfile)

	// Episode Profiles
	mux.HandleFunc("GET /api/episode-profiles", handleListEpisodeProfiles)
	mux.HandleFunc("POST /api/episode-profiles", handleCreateEpisodeProfile)
	mux.HandleFunc("GET /api/episode-profiles/{id}", handleGetEpisodeProfile)
	mux.HandleFunc("PUT /api/episode-profiles/{id}", handleUpdateEpisodeProfile)
	mux.HandleFunc("DELETE /api/episode-profiles/{id}", handleDeleteEpisodeProfile)

	// Podcasts
	mux.HandleFunc("POST /api/podcasts/generate", handleGeneratePodcast)
	mux.HandleFunc("GET /api/podcasts/episodes", handleListPodcastEpisodes)
	mux.HandleFunc("GET /api/podcasts/episodes/{episode_id}", handleGetPodcastEpisode)
	mux.HandleFunc("DELETE /api/podcasts/episodes/{episode_id}", handleDeletePodcastEpisode)
	mux.HandleFunc("GET /api/podcasts/episodes/{episode_id}/audio", handleStreamPodcastEpisodeAudio)
	mux.HandleFunc("GET /api/podcasts/jobs/{job_id}", handleGetPodcastJobStatus)
}

// Speaker Profiles

func handleListSpeakerProfiles(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	profiles, err := domain.ListSpeakerProfiles(ctx)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to retrieve speaker profiles: "+err.Error())
		return
	}

	response := make([]domain.SpeakerProfileResponse, len(profiles))
	for i, p := range profiles {
		vmStr := ""
		if p.VoiceModel != nil {
			vmStr = p.VoiceModel.String()
		}
		response[i] = domain.SpeakerProfileResponse{
			ID:          p.ID.String(),
			Name:        p.Name,
			Description: p.Description,
			VoiceModel:  vmStr,
			Speakers:    p.Speakers,
			Created:     p.Created,
			Updated:     p.Updated,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func handleCreateSpeakerProfile(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Name         string           `json:"name"`
		Description  string           `json:"description"`
		VoiceModelID string           `json:"voice_model"`
		Speakers     []domain.Speaker `json:"speakers"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	profile, err := domain.CreateSpeakerProfile(r.Context(), payload.Name, payload.Description, payload.VoiceModelID, payload.Speakers)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	vmStr := ""
	if profile.VoiceModel != nil {
		vmStr = profile.VoiceModel.String()
	}

	response := domain.SpeakerProfileResponse{
		ID:          profile.ID.String(),
		Name:        profile.Name,
		Description: profile.Description,
		VoiceModel:  vmStr,
		Speakers:    profile.Speakers,
		Created:     profile.Created,
		Updated:     profile.Updated,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(response)
}

func handleGetSpeakerProfile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	profile, err := domain.GetSpeakerProfile(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusNotFound, "Speaker profile not found")
		return
	}

	vmStr := ""
	if profile.VoiceModel != nil {
		vmStr = profile.VoiceModel.String()
	}

	response := domain.SpeakerProfileResponse{
		ID:          profile.ID.String(),
		Name:        profile.Name,
		Description: profile.Description,
		VoiceModel:  vmStr,
		Speakers:    profile.Speakers,
		Created:     profile.Created,
		Updated:     profile.Updated,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func handleUpdateSpeakerProfile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var payload struct {
		Name         *string          `json:"name"`
		Description  *string          `json:"description"`
		VoiceModelID *string          `json:"voice_model"`
		Speakers     []domain.Speaker `json:"speakers"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	profile, err := domain.UpdateSpeakerProfile(r.Context(), id, payload.Name, payload.Description, payload.VoiceModelID, payload.Speakers)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	vmStr := ""
	if profile.VoiceModel != nil {
		vmStr = profile.VoiceModel.String()
	}

	response := domain.SpeakerProfileResponse{
		ID:          profile.ID.String(),
		Name:        profile.Name,
		Description: profile.Description,
		VoiceModel:  vmStr,
		Speakers:    profile.Speakers,
		Created:     profile.Created,
		Updated:     profile.Updated,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func handleDeleteSpeakerProfile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := domain.DeleteSpeakerProfile(r.Context(), id); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to delete speaker profile: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"message": "Speaker profile deleted successfully"})
}

// Episode Profiles

func handleListEpisodeProfiles(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	profiles, err := domain.ListEpisodeProfiles(ctx)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to retrieve episode profiles: "+err.Error())
		return
	}

	response := make([]domain.EpisodeProfileResponse, len(profiles))
	for i, p := range profiles {
		oStr := ""
		if p.OutlineLLM != nil {
			oStr = p.OutlineLLM.String()
		}
		tStr := ""
		if p.TranscriptLLM != nil {
			tStr = p.TranscriptLLM.String()
		}
		response[i] = domain.EpisodeProfileResponse{
			ID:              p.ID.String(),
			Name:            p.Name,
			Description:     p.Description,
			SpeakerConfig:   p.SpeakerConfig,
			OutlineLLM:      oStr,
			TranscriptLLM:   tStr,
			Language:        p.Language,
			DefaultBriefing: p.DefaultBriefing,
			NumSegments:     p.NumSegments,
			Created:         p.Created,
			Updated:         p.Updated,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func handleCreateEpisodeProfile(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Name            string `json:"name"`
		Description     string `json:"description"`
		SpeakerConfig   string `json:"speaker_config"`
		OutlineLLM      string `json:"outline_llm"`
		TranscriptLLM   string `json:"transcript_llm"`
		Language        string `json:"language"`
		DefaultBriefing string `json:"default_briefing"`
		NumSegments     int    `json:"num_segments"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	ep := &domain.EpisodeProfile{
		Name:            payload.Name,
		Description:     payload.Description,
		SpeakerConfig:   payload.SpeakerConfig,
		Language:        payload.Language,
		DefaultBriefing: payload.DefaultBriefing,
		NumSegments:     payload.NumSegments,
	}
	if payload.OutlineLLM != "" {
		ep.OutlineLLM = db.EnsureRecordID("model", payload.OutlineLLM)
	}
	if payload.TranscriptLLM != "" {
		ep.TranscriptLLM = db.EnsureRecordID("model", payload.TranscriptLLM)
	}

	profile, err := domain.CreateEpisodeProfile(r.Context(), ep)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	oStr := ""
	if profile.OutlineLLM != nil {
		oStr = profile.OutlineLLM.String()
	}
	tStr := ""
	if profile.TranscriptLLM != nil {
		tStr = profile.TranscriptLLM.String()
	}

	response := domain.EpisodeProfileResponse{
		ID:              profile.ID.String(),
		Name:            profile.Name,
		Description:     profile.Description,
		SpeakerConfig:   profile.SpeakerConfig,
		OutlineLLM:      oStr,
		TranscriptLLM:   tStr,
		Language:        profile.Language,
		DefaultBriefing: profile.DefaultBriefing,
		NumSegments:     profile.NumSegments,
		Created:         profile.Created,
		Updated:         profile.Updated,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(response)
}

func handleGetEpisodeProfile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	profile, err := domain.GetEpisodeProfile(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusNotFound, "Episode profile not found")
		return
	}

	oStr := ""
	if profile.OutlineLLM != nil {
		oStr = profile.OutlineLLM.String()
	}
	tStr := ""
	if profile.TranscriptLLM != nil {
		tStr = profile.TranscriptLLM.String()
	}

	response := domain.EpisodeProfileResponse{
		ID:              profile.ID.String(),
		Name:            profile.Name,
		Description:     profile.Description,
		SpeakerConfig:   profile.SpeakerConfig,
		OutlineLLM:      oStr,
		TranscriptLLM:   tStr,
		Language:        profile.Language,
		DefaultBriefing: profile.DefaultBriefing,
		NumSegments:     profile.NumSegments,
		Created:         profile.Created,
		Updated:         profile.Updated,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func handleUpdateEpisodeProfile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var payload struct {
		Name            string `json:"name"`
		Description     string `json:"description"`
		SpeakerConfig   string `json:"speaker_config"`
		OutlineLLM      string `json:"outline_llm"`
		TranscriptLLM   string `json:"transcript_llm"`
		Language        string `json:"language"`
		DefaultBriefing string `json:"default_briefing"`
		NumSegments     int    `json:"num_segments"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	ep := &domain.EpisodeProfile{
		Name:            payload.Name,
		Description:     payload.Description,
		SpeakerConfig:   payload.SpeakerConfig,
		Language:        payload.Language,
		DefaultBriefing: payload.DefaultBriefing,
		NumSegments:     payload.NumSegments,
	}
	if payload.OutlineLLM != "" {
		ep.OutlineLLM = db.EnsureRecordID("model", payload.OutlineLLM)
	}
	if payload.TranscriptLLM != "" {
		ep.TranscriptLLM = db.EnsureRecordID("model", payload.TranscriptLLM)
	}

	profile, err := domain.UpdateEpisodeProfile(r.Context(), id, ep)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	oStr := ""
	if profile.OutlineLLM != nil {
		oStr = profile.OutlineLLM.String()
	}
	tStr := ""
	if profile.TranscriptLLM != nil {
		tStr = profile.TranscriptLLM.String()
	}

	response := domain.EpisodeProfileResponse{
		ID:              profile.ID.String(),
		Name:            profile.Name,
		Description:     profile.Description,
		SpeakerConfig:   profile.SpeakerConfig,
		OutlineLLM:      oStr,
		TranscriptLLM:   tStr,
		Language:        profile.Language,
		DefaultBriefing: profile.DefaultBriefing,
		NumSegments:     profile.NumSegments,
		Created:         profile.Created,
		Updated:         profile.Updated,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func handleDeleteEpisodeProfile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := domain.DeleteEpisodeProfile(r.Context(), id); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to delete episode profile: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"message": "Episode profile deleted successfully"})
}

// Podcasts / Episodes

func handleGeneratePodcast(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		EpisodeProfile string  `json:"episode_profile"`
		SpeakerProfile string  `json:"speaker_profile"`
		EpisodeName    string  `json:"episode_name"`
		Content        string  `json:"content"`
		BriefingSuffix *string `json:"briefing_suffix,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	if payload.EpisodeProfile == "" || payload.SpeakerProfile == "" || payload.EpisodeName == "" || payload.Content == "" {
		respondError(w, http.StatusBadRequest, "episode_profile, speaker_profile, episode_name, and content are required")
		return
	}

	ctx := r.Context()
	nowStr := time.Now().UTC().Format(time.RFC3339)

	inputData := map[string]any{
		"episode_profile": payload.EpisodeProfile,
		"speaker_profile": payload.SpeakerProfile,
		"episode_name":    payload.EpisodeName,
		"content":         payload.Content,
	}
	if payload.BriefingSuffix != nil {
		inputData["briefing_suffix"] = *payload.BriefingSuffix
	}

	jobData := map[string]any{
		"app":            "open_notebook",
		"command":        "generate_podcast",
		"status":         "pending",
		"created":        nowStr,
		"updated":        nowStr,
		"retry_attempts": 0,
		"input":          inputData,
	}

	type CommandRecord struct {
		ID *models.RecordID `json:"id"`
	}

	res, err := db.RepoCreate[CommandRecord](ctx, "command", jobData)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to submit podcast command: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"job_id":          res.ID.String(),
		"status":          "submitted",
		"message":         fmt.Sprintf("Podcast generation started for episode '%s'", payload.EpisodeName),
		"episode_profile": payload.EpisodeProfile,
		"episode_name":    payload.EpisodeName,
	})
}

func handleListPodcastEpisodes(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	episodes, err := domain.ListPodcastEpisodes(ctx)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to list podcast episodes: "+err.Error())
		return
	}

	var response []map[string]any
	for _, ep := range episodes {
		// Skip incomplete episodes without command or audio
		if ep.Command == nil && ep.AudioFile == "" {
			continue
		}

		jobStatus := "unknown"
		var errMsg *string
		if ep.Command != nil {
			cmd, err := domain.GetCommandJob(ctx, ep.Command.String())
			if err == nil && cmd != nil {
				jobStatus = cmd.Status
				errMsg = cmd.ErrorMessage
			}
		} else if ep.AudioFile != "" {
			jobStatus = "completed"
		}

		audioURL := ""
		if ep.AudioFile != "" {
			audioURL = fmt.Sprintf("/api/podcasts/episodes/%s/audio", ep.ID.String())
		}

		epMap := map[string]any{
			"id":              ep.ID.String(),
			"name":            ep.Name,
			"episode_profile": ep.EpisodeProfile,
			"speaker_profile": ep.SpeakerProfile,
			"briefing":        ep.Briefing,
			"content":         ep.Content,
			"audio_file":      ep.AudioFile,
			"audio_url":       audioURL,
			"transcript":      ep.Transcript,
			"outline":         ep.Outline,
			"created":         ep.Created,
			"updated":         ep.Updated,
			"job_status":      jobStatus,
		}
		if errMsg != nil {
			epMap["error_message"] = *errMsg
		}
		response = append(response, epMap)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func handleGetPodcastEpisode(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("episode_id")
	ctx := r.Context()
	ep, err := domain.GetPodcastEpisode(ctx, id)
	if err != nil {
		respondError(w, http.StatusNotFound, "Podcast episode not found")
		return
	}

	jobStatus := "unknown"
	var errMsg *string
	if ep.Command != nil {
		cmd, err := domain.GetCommandJob(ctx, ep.Command.String())
		if err == nil && cmd != nil {
			jobStatus = cmd.Status
			errMsg = cmd.ErrorMessage
		}
	} else if ep.AudioFile != "" {
		jobStatus = "completed"
	}

	audioURL := ""
	if ep.AudioFile != "" {
		audioURL = fmt.Sprintf("/api/podcasts/episodes/%s/audio", ep.ID.String())
	}

	epMap := map[string]any{
		"id":              ep.ID.String(),
		"name":            ep.Name,
		"episode_profile": ep.EpisodeProfile,
		"speaker_profile": ep.SpeakerProfile,
		"briefing":        ep.Briefing,
		"content":         ep.Content,
		"audio_file":      ep.AudioFile,
		"audio_url":       audioURL,
		"transcript":      ep.Transcript,
		"outline":         ep.Outline,
		"created":         ep.Created,
		"updated":         ep.Updated,
		"job_status":      jobStatus,
	}
	if errMsg != nil {
		epMap["error_message"] = *errMsg
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(epMap)
}

func handleDeletePodcastEpisode(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("episode_id")
	ctx := r.Context()
	episode, err := domain.GetPodcastEpisode(ctx, id)
	if err != nil {
		respondError(w, http.StatusNotFound, "Podcast episode not found")
		return
	}

	if episode.AudioFile != "" {
		audioPath := episode.AudioFile
		if strings.HasPrefix(audioPath, "file://") {
			audioPath = strings.TrimPrefix(audioPath, "file://")
		}
		_ = os.Remove(audioPath)
	}

	if err := domain.DeletePodcastEpisode(ctx, id); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to delete podcast episode: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"message":    "Episode deleted successfully",
		"episode_id": id,
	})
}

func handleStreamPodcastEpisodeAudio(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("episode_id")
	ctx := r.Context()
	episode, err := domain.GetPodcastEpisode(ctx, id)
	if err != nil {
		respondError(w, http.StatusNotFound, "Podcast episode not found")
		return
	}

	if episode.AudioFile == "" {
		respondError(w, http.StatusNotFound, "Episode has no audio file")
		return
	}

	audioPath := episode.AudioFile
	if strings.HasPrefix(audioPath, "file://") {
		audioPath = strings.TrimPrefix(audioPath, "file://")
	}

	http.ServeFile(w, r, audioPath)
}

func handleGetPodcastJobStatus(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("job_id")
	ctx := r.Context()
	cmd, err := domain.GetCommandJob(ctx, jobID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Job not found")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"job_id":        jobID,
		"status":        cmd.Status,
		"error_message": cmd.ErrorMessage,
		"result":        cmd.Result,
	})
}
