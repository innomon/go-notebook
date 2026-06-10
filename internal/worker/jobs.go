package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go-notebook/internal/ai"
	"go-notebook/internal/db"
	"go-notebook/internal/domain"
	"go-notebook/internal/extractor"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/flosch/pongo2/v6"
	"github.com/gofrs/uuid"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// ExecuteJob dispatches a CommandJob to its respective handler function
func ExecuteJob(ctx context.Context, job *domain.CommandJob) (map[string]any, error) {
	log.Printf("[Worker] Executing job %s (%s)", job.ID.String(), job.Command)

	switch job.Command {
	case "process_source":
		return handleProcessSource(ctx, job)
	case "embed_source":
		return handleEmbedSource(ctx, job)
	case "embed_note":
		return handleEmbedNote(ctx, job)
	case "embed_insight":
		return handleEmbedInsight(ctx, job)
	case "generate_podcast":
		return handleGeneratePodcast(ctx, job)
	case "rebuild_embeddings":
		return handleRebuildEmbeddings(ctx, job)
	default:
		return nil, fmt.Errorf("unknown command: %s", job.Command)
	}
}

// 1. process_source job
func handleProcessSource(ctx context.Context, job *domain.CommandJob) (map[string]any, error) {
	sourceID, _ := job.Input["source_id"].(string)
	if sourceID == "" {
		return nil, errors.New("missing source_id in input")
	}

	contentState, _ := job.Input["content_state"].(map[string]any)
	notebookIDsVal, _ := job.Input["notebook_ids"].([]any)
	transformationsVal, _ := job.Input["transformations"].([]any)
	embed, _ := job.Input["embed"].(bool)

	// Fetch the Source
	source, err := domain.GetSource(ctx, sourceID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch source: %w", err)
	}

	// Update source with command reference
	_, _ = db.RepoUpdate[any](ctx, "source", sourceID, map[string]any{
		"command": job.ID,
	})

	var extractedText string
	var extractedTitle string

	// Extract Content based on type
	if url, ok := contentState["url"].(string); ok && url != "" {
		log.Printf("[Worker] Extracting text from URL: %s", url)
		title, text, err := extractor.ExtractTextFromURL(url)
		if err != nil {
			return nil, fmt.Errorf("failed to extract text from URL: %w", err)
		}
		extractedText = text
		extractedTitle = title
	} else if filePath, ok := contentState["file_path"].(string); ok && filePath != "" {
		log.Printf("[Worker] Extracting text from file path: %s", filePath)

		if strings.HasSuffix(strings.ToLower(filePath), ".pdf") {
			text, err := extractor.ExtractTextFromPDF(filePath)
			if err != nil {
				return nil, fmt.Errorf("failed to extract text from PDF: %w", err)
			}
			extractedText = text
		} else if isAudioVideoFile(filePath) {
			log.Printf("[Worker] Audio/Video file detected, transcribing: %s", filePath)
			sttClient, err := ai.GetClientForDefaultModel(ctx, "speech_to_text")
			if err != nil {
				return nil, fmt.Errorf("speech-to-text model not configured: %w", err)
			}
			text, err := sttClient.TranscribeAudio(ctx, filePath)
			if err != nil {
				return nil, fmt.Errorf("failed to transcribe audio: %w", err)
			}
			extractedText = text
		} else {
			// Read as standard text
			b, err := os.ReadFile(filePath)
			if err != nil {
				return nil, fmt.Errorf("failed to read text file: %w", err)
			}
			extractedText = string(b)
		}

		extractedTitle = filepath.Base(filePath)

		// Delete source file if requested
		if deleteSource, _ := contentState["delete_source"].(bool); deleteSource {
			_ = os.Remove(filePath)
		}
	} else if content, ok := contentState["content"].(string); ok {
		extractedText = content
		extractedTitle = "Text Source"
	} else {
		return nil, errors.New("invalid content_state input")
	}

	if strings.TrimSpace(extractedText) == "" {
		return nil, errors.New("extracted content is empty")
	}

	// Update the Source record
	updateData := map[string]any{
		"full_text": extractedText,
	}
	if source.Title == "" || source.Title == "Processing..." {
		updateData["title"] = extractedTitle
	}
	_, err = db.RepoUpdate[any](ctx, "source", sourceID, updateData)
	if err != nil {
		return nil, fmt.Errorf("failed to update source record: %w", err)
	}

	// Run transformations to generate insights
	insightsCreated := 0
	for _, transIDVal := range transformationsVal {
		transID, _ := transIDVal.(string)
		if transID == "" {
			continue
		}

		trans, err := domain.GetTransformation(ctx, transID)
		if err != nil {
			log.Printf("[Worker] Warning: failed to fetch transformation %s: %v", transID, err)
			continue
		}

		log.Printf("[Worker] Running transformation: %s", trans.Name)

		// Render templates with pongo2
		tpl, err := pongo2.FromString(trans.Prompt)
		if err != nil {
			log.Printf("[Worker] Warning: failed to parse template: %v", err)
			continue
		}

		systemPrompt, err := tpl.Execute(pongo2.Context{
			"input_text":     extractedText,
			"source":         source,
			"transformation": trans,
		})
		if err != nil {
			log.Printf("[Worker] Warning: failed to execute template: %v", err)
			continue
		}

		llmClient, err := ai.GetClientForDefaultModel(ctx, "transformation")
		if err != nil {
			return nil, fmt.Errorf("failed to get LLM client for transformations: %w", err)
		}

		rawResponse, err := llmClient.GenerateText(ctx, systemPrompt, extractedText)
		if err != nil {
			return nil, fmt.Errorf("LLM generation failed: %w", err)
		}

		cleanedResponse := cleanJSONResponse(rawResponse)

		// Save Insight
		insightData := map[string]any{
			"source":       db.EnsureRecordIDString("source", sourceID),
			"insight_type": trans.Title,
			"content":      cleanedResponse,
		}

		insightRecord, err := db.RepoCreate[domain.SourceInsight](ctx, "source_insight", insightData)
		if err != nil {
			log.Printf("[Worker] Warning: failed to save insight: %v", err)
			continue
		}
		insightsCreated++

		// Queue embed_insight job
		embedInsightJob := map[string]any{
			"app":            "open_notebook",
			"command":        "embed_insight",
			"status":         "pending",
			"created":        time.Now().UTC().Format(time.RFC3339),
			"updated":        time.Now().UTC().Format(time.RFC3339),
			"retry_attempts": 0,
			"input": map[string]any{
				"insight_id": insightRecord.ID.String(),
			},
		}
		_, _ = db.RepoCreate[any](ctx, "command", embedInsightJob)
	}

	// Trigger async embedding if requested
	if embed {
		embedSourceJob := map[string]any{
			"app":            "open_notebook",
			"command":        "embed_source",
			"status":         "pending",
			"created":        time.Now().UTC().Format(time.RFC3339),
			"updated":        time.Now().UTC().Format(time.RFC3339),
			"retry_attempts": 0,
			"input": map[string]any{
				"source_id": sourceID,
			},
		}
		_, _ = db.RepoCreate[any](ctx, "command", embedSourceJob)
	}

	var nbIDs []string
	for _, val := range notebookIDsVal {
		if s, ok := val.(string); ok {
			nbIDs = append(nbIDs, s)
		}
	}

	return map[string]any{
		"success":          true,
		"source_id":        sourceID,
		"insights_created": insightsCreated,
		"notebook_ids":     nbIDs,
	}, nil
}

// 2. embed_source job
func handleEmbedSource(ctx context.Context, job *domain.CommandJob) (map[string]any, error) {
	sourceID, _ := job.Input["source_id"].(string)
	if sourceID == "" {
		return nil, errors.New("missing source_id in input")
	}

	source, err := domain.GetSource(ctx, sourceID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch source: %w", err)
	}

	if strings.TrimSpace(source.FullText) == "" {
		return nil, errors.New("source text is empty, nothing to embed")
	}

	// Delete existing embeddings for this source
	_, err = db.RepoQuery[any](ctx, "DELETE source_embedding WHERE source = $id;", map[string]any{"id": db.EnsureRecordIDString("source", sourceID)})
	if err != nil {
		return nil, fmt.Errorf("failed to delete old embeddings: %w", err)
	}

	// Chunk text
	chunks := extractor.ChunkText(source.FullText, 1500, 225)
	if len(chunks) == 0 {
		return nil, errors.New("no chunks created from source text")
	}

	embedClient, err := ai.GetClientForDefaultModel(ctx, "embedding")
	if err != nil {
		return nil, fmt.Errorf("embedding model not configured: %w", err)
	}

	log.Printf("[Worker] Generating embeddings for %d chunks of source %s", len(chunks), sourceID)

	for idx, chunk := range chunks {
		embedding, err := embedClient.EmbedText(ctx, chunk)
		if err != nil {
			return nil, fmt.Errorf("failed to generate chunk embedding: %w", err)
		}

		data := map[string]any{
			"source":    db.EnsureRecordID("source", sourceID),
			"order":     idx,
			"content":   chunk,
			"embedding": embedding,
		}

		_, err = db.RepoCreate[any](ctx, "source_embedding", data)
		if err != nil {
			return nil, fmt.Errorf("failed to save chunk embedding to database: %w", err)
		}
	}

	return map[string]any{
		"success":        true,
		"source_id":      sourceID,
		"chunks_created": len(chunks),
	}, nil
}

// 3. embed_note job
func handleEmbedNote(ctx context.Context, job *domain.CommandJob) (map[string]any, error) {
	noteID, _ := job.Input["note_id"].(string)
	if noteID == "" {
		return nil, errors.New("missing note_id in input")
	}

	note, err := domain.GetNote(ctx, noteID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch note: %w", err)
	}

	if strings.TrimSpace(note.Content) == "" {
		return nil, errors.New("note content is empty, nothing to embed")
	}

	embedClient, err := ai.GetClientForDefaultModel(ctx, "embedding")
	if err != nil {
		return nil, fmt.Errorf("embedding model not configured: %w", err)
	}

	embedding, err := embedTextWithPooling(ctx, embedClient, note.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to embed note: %w", err)
	}

	_, err = db.RepoQuery[any](ctx, "UPDATE ONLY $id SET embedding = $embedding;", map[string]any{
		"id":        db.EnsureRecordIDString("note", noteID),
		"embedding": embedding,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to save note embedding: %w", err)
	}

	return map[string]any{
		"success": true,
		"note_id": noteID,
	}, nil
}

// 4. embed_insight job
func handleEmbedInsight(ctx context.Context, job *domain.CommandJob) (map[string]any, error) {
	insightID, _ := job.Input["insight_id"].(string)
	if insightID == "" {
		return nil, errors.New("missing insight_id in input")
	}

	insight, err := domain.GetSourceInsight(ctx, insightID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch insight: %w", err)
	}

	if strings.TrimSpace(insight.Content) == "" {
		return nil, errors.New("insight content is empty, nothing to embed")
	}

	embedClient, err := ai.GetClientForDefaultModel(ctx, "embedding")
	if err != nil {
		return nil, fmt.Errorf("embedding model not configured: %w", err)
	}

	embedding, err := embedTextWithPooling(ctx, embedClient, insight.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to embed insight: %w", err)
	}

	_, err = db.RepoQuery[any](ctx, "UPDATE ONLY $id SET embedding = $embedding;", map[string]any{
		"id":        db.EnsureRecordIDString("source_insight", insightID),
		"embedding": embedding,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to save insight embedding: %w", err)
	}

	return map[string]any{
		"success":    true,
		"insight_id": insightID,
	}, nil
}

// 5. generate_podcast job
func handleGeneratePodcast(ctx context.Context, job *domain.CommandJob) (map[string]any, error) {
	epProfileName, _ := job.Input["episode_profile"].(string)
	spProfileName, _ := job.Input["speaker_profile"].(string)
	episodeName, _ := job.Input["episode_name"].(string)
	content, _ := job.Input["content"].(string)
	briefingSuffix, _ := job.Input["briefing_suffix"].(string)

	if epProfileName == "" || spProfileName == "" || episodeName == "" || content == "" {
		return nil, errors.New("missing required arguments for generate_podcast")
	}

	log.Printf("[Worker] Starting podcast generation: %s", episodeName)

	// Load Profiles
	epProfile, err := domain.GetEpisodeProfileByName(ctx, epProfileName)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch episode profile: %w", err)
	}

	spProfile, err := domain.GetSpeakerProfileByName(ctx, spProfileName)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch speaker profile: %w", err)
	}

	// 1. Resolve LLM Clients
	var outlineClient ai.AIClient
	if epProfile.OutlineLLM != nil {
		outlineClient, err = ai.GetClientForModel(ctx, epProfile.OutlineLLM.String())
	} else {
		outlineClient, err = ai.GetClientForDefaultModel(ctx, "chat")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get outline LLM client: %w", err)
	}

	var transcriptClient ai.AIClient
	if epProfile.TranscriptLLM != nil {
		transcriptClient, err = ai.GetClientForModel(ctx, epProfile.TranscriptLLM.String())
	} else {
		transcriptClient, err = ai.GetClientForDefaultModel(ctx, "chat")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get transcript LLM client: %w", err)
	}

	// Create Episode Profile / Speaker Profile snapshot dump maps
	epProfileDump := map[string]any{
		"name":             epProfile.Name,
		"description":      epProfile.Description,
		"speaker_config":   epProfile.SpeakerConfig,
		"language":         epProfile.Language,
		"default_briefing": epProfile.DefaultBriefing,
		"num_segments":     epProfile.NumSegments,
	}

	speakersList := make([]map[string]any, len(spProfile.Speakers))
	for i, sp := range spProfile.Speakers {
		speakersList[i] = map[string]any{
			"name":        sp.Name,
			"voice_id":    sp.VoiceID,
			"backstory":   sp.Backstory,
			"personality": sp.Personality,
			"voice_model": sp.VoiceModel,
		}
	}
	spProfileDump := map[string]any{
		"name":        spProfile.Name,
		"description": spProfile.Description,
		"speakers":    speakersList,
	}

	// 2. Build Briefing
	briefing := epProfile.DefaultBriefing
	if briefingSuffix != "" {
		briefing += "\n\nAdditional instructions: " + briefingSuffix
	}

	// Save preliminary episode record linked to this command
	episodeData := map[string]any{
		"name":            episodeName,
		"episode_profile": epProfileDump,
		"speaker_profile": spProfileDump,
		"briefing":        briefing,
		"content":         content,
		"command":         job.ID,
	}
	episodeRecord, err := db.RepoCreate[domain.PodcastEpisode](ctx, "episode", episodeData)
	if err != nil {
		return nil, fmt.Errorf("failed to create episode record: %w", err)
	}

	// 3. Generate Outline
	log.Println("[Worker] Generating outline...")
	outlineTplBytes, err := os.ReadFile("prompts/podcast/outline.jinja")
	if err != nil {
		return nil, fmt.Errorf("failed to read outline template: %w", err)
	}

	tpl, err := pongo2.FromString(string(outlineTplBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to parse outline template: %w", err)
	}

	renderedOutlinePrompt, err := tpl.Execute(pongo2.Context{
		"briefing":            briefing,
		"context":             content,
		"speakers":            spProfile.Speakers,
		"num_segments":        epProfile.NumSegments,
		"format_instructions": "Return a JSON object with a 'segments' array of objects containing 'name', 'description', and 'size' ('short', 'medium', or 'long').",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to execute outline template: %w", err)
	}

	rawOutline, err := outlineClient.GenerateText(ctx, "", renderedOutlinePrompt)
	if err != nil {
		return nil, fmt.Errorf("failed to generate outline text: %w", err)
	}

	cleanedOutlineJSON := cleanJSONResponse(rawOutline)

	var outline Outline
	if err := json.Unmarshal([]byte(cleanedOutlineJSON), &outline); err != nil {
		return nil, fmt.Errorf("failed to parse outline JSON: %w (raw response: %s)", err, rawOutline)
	}

	// Save outline snapshot to episode record
	var outlineMap map[string]any
	_ = json.Unmarshal([]byte(cleanedOutlineJSON), &outlineMap)
	_, _ = db.RepoUpdate[any](ctx, "episode", episodeRecord.ID.String(), map[string]any{
		"outline": outlineMap,
	})

	// 4. Generate Transcript Segment by Segment
	log.Println("[Worker] Generating transcript...")
	transcriptTplBytes, err := os.ReadFile("prompts/podcast/transcript.jinja")
	if err != nil {
		return nil, fmt.Errorf("failed to read transcript template: %w", err)
	}

	tplTranscript, err := pongo2.FromString(string(transcriptTplBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to parse transcript template: %w", err)
	}

	var speakerNames []string
	for _, sp := range spProfile.Speakers {
		speakerNames = append(speakerNames, sp.Name)
	}

	var fullTranscript []map[string]any

	for idx, seg := range outline.Segments {
		log.Printf("[Worker] Generating segment %d/%d: %s", idx+1, len(outline.Segments), seg.Name)
		isFinal := idx == len(outline.Segments)-1
		turns := 6
		switch seg.Size {
		case "short":
			turns = 3
		case "medium":
			turns = 6
		case "long":
			turns = 10
		}

		renderedTranscriptPrompt, err := tplTranscript.Execute(pongo2.Context{
			"briefing":            briefing,
			"context":             content,
			"speakers":            spProfile.Speakers,
			"outline":             outlineMap,
			"transcript":          fullTranscript,
			"is_final":            isFinal,
			"segment":             seg,
			"speaker_names":       speakerNames,
			"turns":               turns,
			"format_instructions": "Return a JSON object with a 'transcript' array of objects containing 'speaker' and 'dialogue'.",
		})
		if err != nil {
			return nil, fmt.Errorf("failed to execute transcript template: %w", err)
		}

		rawTranscriptResponse, err := transcriptClient.GenerateText(ctx, "", renderedTranscriptPrompt)
		if err != nil {
			return nil, fmt.Errorf("failed to generate transcript text: %w", err)
		}

		cleanedTranscriptJSON := cleanJSONResponse(rawTranscriptResponse)

		var segmentTranscript struct {
			Transcript []map[string]any `json:"transcript"`
		}
		if err := json.Unmarshal([]byte(cleanedTranscriptJSON), &segmentTranscript); err != nil {
			// fallback: try array directly
			var rawArray []map[string]any
			if err2 := json.Unmarshal([]byte(cleanedTranscriptJSON), &rawArray); err2 == nil {
				fullTranscript = append(fullTranscript, rawArray...)
			} else {
				return nil, fmt.Errorf("failed to parse transcript segment: %w (raw response: %s)", err, rawTranscriptResponse)
			}
		} else {
			fullTranscript = append(fullTranscript, segmentTranscript.Transcript...)
		}
	}

	// Save transcript snapshot to episode record
	transcriptMap := map[string]any{
		"transcript": fullTranscript,
	}
	_, _ = db.RepoUpdate[any](ctx, "episode", episodeRecord.ID.String(), map[string]any{
		"transcript": transcriptMap,
	})

	// 5. Generate Voice Audio Clips (TTS)
	log.Println("[Worker] Generating voice clips...")
	uuidName := uuid.Must(uuid.NewV4()).String()
	dataDir := os.Getenv("DATA_FOLDER")
	if dataDir == "" {
		dataDir = "."
	}
	episodeDir := filepath.Join(dataDir, "podcasts", "episodes", uuidName)
	clipsDir := filepath.Join(episodeDir, "clips")
	_ = os.MkdirAll(clipsDir, 0755)

	batchSize := 5
	var audioClipsList []string

	for i := 0; i < len(fullTranscript); i += batchSize {
		end := i + batchSize
		if end > len(fullTranscript) {
			end = len(fullTranscript)
		}

		var wg sync.WaitGroup
		errChan := make(chan error, end-i)

		for j := i; j < end; j++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				turn := fullTranscript[idx]
				speakerName, _ := turn["speaker"].(string)
				dialogueText, _ := turn["dialogue"].(string)

				if speakerName == "" || dialogueText == "" {
					return
				}

				// Find speaker profile configs
				var sp domain.Speaker
				found := false
				for _, s := range spProfile.Speakers {
					if strings.EqualFold(s.Name, speakerName) {
						sp = s
						found = true
						break
					}
				}
				if !found && len(spProfile.Speakers) > 0 {
					sp = spProfile.Speakers[0]
				}

				// Resolve model and client
				voiceModelID := sp.VoiceModel
				if voiceModelID == "" && spProfile.VoiceModel != nil {
					voiceModelID = spProfile.VoiceModel.String()
				}

				var ttsClient ai.AIClient
				var err error
				if voiceModelID != "" {
					ttsClient, err = ai.GetClientForModel(ctx, voiceModelID)
				} else {
					ttsClient, err = ai.GetClientForDefaultModel(ctx, "text_to_speech")
				}
				if err != nil {
					errChan <- fmt.Errorf("failed to resolve TTS client for speaker %s: %w", speakerName, err)
					return
				}

				voiceID := sp.VoiceID
				if voiceID == "" {
					voiceID = "alloy" // default fallback
				}

				audioBytes, err := ttsClient.GenerateSpeech(ctx, dialogueText, voiceID)
				if err != nil {
					errChan <- fmt.Errorf("TTS speech generation failed for speaker %s: %w", speakerName, err)
					return
				}

				clipFilename := fmt.Sprintf("%04d.mp3", idx)
				clipPath := filepath.Join(clipsDir, clipFilename)
				if err := os.WriteFile(clipPath, audioBytes, 0644); err != nil {
					errChan <- fmt.Errorf("failed to save audio clip: %w", err)
					return
				}
			}(j)
		}

		wg.Wait()
		close(errChan)
		for err := range errChan {
			if err != nil {
				return nil, err
			}
		}

		// Collect paths
		for j := i; j < end; j++ {
			audioClipsList = append(audioClipsList, filepath.Join(clipsDir, fmt.Sprintf("%04d.mp3", j)))
		}

		// Sleep between batches
		if end < len(fullTranscript) {
			time.Sleep(1 * time.Second)
		}
	}

	// 6. Concatenate Audio Clips (constant bitrate MP3 byte concatenation)
	log.Println("[Worker] Concatenating audio clips...")
	audioOutDir := filepath.Join(episodeDir, "audio")
	_ = os.MkdirAll(audioOutDir, 0755)

	finalAudioPath := filepath.Join(audioOutDir, fmt.Sprintf("%s.mp3", episodeName))
	err = concatenateMP3Files(finalAudioPath, audioClipsList)
	if err != nil {
		return nil, fmt.Errorf("failed to concatenate audio segments: %w", err)
	}

	// Update episode record with the final path
	finalPathString := "file://" + finalAudioPath
	_, err = db.RepoUpdate[any](ctx, "episode", episodeRecord.ID.String(), map[string]any{
		"audio_file": finalPathString,
	})
	if err != nil {
		log.Printf("[Worker] Warning: failed to save final audio file path to episode record: %v", err)
	}

	return map[string]any{
		"success":          true,
		"episode_id":       episodeRecord.ID.String(),
		"audio_file_path":  finalPathString,
		"outline":          outlineMap,
		"transcript":       transcriptMap,
		"processing_time":  time.Now().Sub(time.Now()).Seconds(), // placeholder
	}, nil
}

// 6. rebuild_embeddings job
func handleRebuildEmbeddings(ctx context.Context, job *domain.CommandJob) (map[string]any, error) {
	// Rebuild coordinators just query the DB and submit individual embed commands
	mode, _ := job.Input["mode"].(string) // "existing" or "all"
	includeSources, _ := job.Input["include_sources"].(bool)
	includeNotes, _ := job.Input["include_notes"].(bool)
	includeInsights, _ := job.Input["include_insights"].(bool)

	totalJobs := 0

	if includeSources {
		var err error
		var resPtr *[]any
		if mode == "existing" {
			resPtr, err = db.RepoQuery[[]any](ctx, `
				RETURN array::distinct(
					SELECT VALUE source.id
					FROM source_embedding
					WHERE embedding != none AND array::len(embedding) > 0
				)
			`, nil)
		} else {
			// Select ids from source
			resMapPtr, errQuery := db.RepoQuery[[]map[string]any](ctx, "SELECT id FROM source WHERE full_text != none AND string::trim(full_text) != '';", nil)
			if errQuery == nil && resMapPtr != nil {
				var converted []any
				for _, row := range *resMapPtr {
					converted = append(converted, row["id"])
				}
				resPtr = &converted
			} else {
				err = errQuery
			}
		}

		if err == nil && resPtr != nil {
			for _, idVal := range *resPtr {
				idStr := ""
				if recordID, ok := idVal.(*models.RecordID); ok {
					idStr = recordID.String()
				} else if s, ok := idVal.(string); ok {
					idStr = s
				}
				if idStr != "" {
					embedSourceJob := map[string]any{
						"app":            "open_notebook",
						"command":        "embed_source",
						"status":         "pending",
						"created":        time.Now().UTC().Format(time.RFC3339),
						"updated":        time.Now().UTC().Format(time.RFC3339),
						"retry_attempts": 0,
						"input": map[string]any{
							"source_id": idStr,
						},
					}
					_, _ = db.RepoCreate[any](ctx, "command", embedSourceJob)
					totalJobs++
				}
			}
		}
	}

	if includeNotes {
		var err error
		var resPtr *[]map[string]any
		if mode == "existing" {
			resPtr, err = db.RepoQuery[[]map[string]any](ctx, "SELECT id FROM note WHERE embedding != none AND array::len(embedding) > 0;", nil)
		} else {
			resPtr, err = db.RepoQuery[[]map[string]any](ctx, "SELECT id FROM note WHERE content != none AND string::trim(content) != '';", nil)
		}

		if err == nil && resPtr != nil {
			for _, row := range *resPtr {
				idVal := row["id"]
				idStr := ""
				if recordID, ok := idVal.(*models.RecordID); ok {
					idStr = recordID.String()
				} else if s, ok := idVal.(string); ok {
					idStr = s
				}
				if idStr != "" {
					embedNoteJob := map[string]any{
						"app":            "open_notebook",
						"command":        "embed_note",
						"status":         "pending",
						"created":        time.Now().UTC().Format(time.RFC3339),
						"updated":        time.Now().UTC().Format(time.RFC3339),
						"retry_attempts": 0,
						"input": map[string]any{
							"note_id": idStr,
						},
					}
					_, _ = db.RepoCreate[any](ctx, "command", embedNoteJob)
					totalJobs++
				}
			}
		}
	}

	if includeInsights {
		var err error
		var resPtr *[]map[string]any
		if mode == "existing" {
			resPtr, err = db.RepoQuery[[]map[string]any](ctx, "SELECT id FROM source_insight WHERE embedding != none AND array::len(embedding) > 0;", nil)
		} else {
			resPtr, err = db.RepoQuery[[]map[string]any](ctx, "SELECT id FROM source_insight WHERE content != none AND string::trim(content) != '';", nil)
		}

		if err == nil && resPtr != nil {
			for _, row := range *resPtr {
				idVal := row["id"]
				idStr := ""
				if recordID, ok := idVal.(*models.RecordID); ok {
					idStr = recordID.String()
				} else if s, ok := idVal.(string); ok {
					idStr = s
				}
				if idStr != "" {
					embedInsightJob := map[string]any{
						"app":            "open_notebook",
						"command":        "embed_insight",
						"status":         "pending",
						"created":        time.Now().UTC().Format(time.RFC3339),
						"updated":        time.Now().UTC().Format(time.RFC3339),
						"retry_attempts": 0,
						"input": map[string]any{
							"insight_id": idStr,
						},
					}
					_, _ = db.RepoCreate[any](ctx, "command", embedInsightJob)
					totalJobs++
				}
			}
		}
	}

	return map[string]any{
		"success":        true,
		"jobs_submitted": totalJobs,
	}, nil
}

// Helper utility functions

type Segment struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Size        string `json:"size"`
}

type Outline struct {
	Segments []Segment `json:"segments"`
}

func isAudioVideoFile(filePath string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".mp3", ".wav", ".m4a", ".ogg", ".flac", ".aac", ".wma", ".webm", ".mp4", ".mov", ".avi", ".mkv":
		return true
	}
	return false
}

func embedTextWithPooling(ctx context.Context, client ai.AIClient, text string) ([]float32, error) {
	chunks := extractor.ChunkText(text, 1500, 225)
	if len(chunks) == 0 {
		return nil, fmt.Errorf("empty text")
	}
	if len(chunks) == 1 {
		return client.EmbedText(ctx, chunks[0])
	}

	var sum []float32
	for _, chunk := range chunks {
		emb, err := client.EmbedText(ctx, chunk)
		if err != nil {
			return nil, err
		}
		if sum == nil {
			sum = make([]float32, len(emb))
		}
		for i, val := range emb {
			sum[i] += val
		}
	}

	numChunks := float32(len(chunks))
	for i := range sum {
		sum[i] /= numChunks
	}
	return sum, nil
}

func cleanJSONResponse(raw string) string {
	for {
		start := strings.Index(raw, "<think>")
		if start == -1 {
			break
		}
		end := strings.Index(raw, "</think>")
		if end == -1 {
			raw = raw[:start]
			break
		}
		raw = raw[:start] + raw[end+len("</think>"):]
	}

	if idx := strings.Index(raw, "```json"); idx != -1 {
		raw = raw[idx+len("```json"):]
	} else if idx := strings.Index(raw, "```"); idx != -1 {
		raw = raw[idx+3:]
	}
	if idx := strings.LastIndex(raw, "```"); idx != -1 {
		raw = raw[:idx]
	}

	return strings.TrimSpace(raw)
}

func concatenateMP3Files(outputFile string, inputFiles []string) error {
	out, err := os.Create(outputFile)
	if err != nil {
		return err
	}
	defer out.Close()

	for _, file := range inputFiles {
		in, err := os.Open(file)
		if err != nil {
			return err
		}
		_, err = io.Copy(out, in)
		in.Close()
		if err != nil {
			return err
		}
	}
	return nil
}
