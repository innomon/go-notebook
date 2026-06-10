package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ChatMessage represents a message in a chat history
type ChatMessage struct {
	Role    string `json:"role"`    // "system", "user", "assistant"
	Content string `json:"content"`
}

// AIClient is the unified interface for interacting with LLM and embedding providers
type AIClient interface {
	EmbedText(ctx context.Context, text string) ([]float32, error)
	GenerateChatStream(ctx context.Context, systemPrompt string, messages []ChatMessage, onToken func(string)) error
	GenerateText(ctx context.Context, systemPrompt string, userPrompt string) (string, error)
	GenerateSpeech(ctx context.Context, text string, voice string) ([]byte, error)
	TranscribeAudio(ctx context.Context, filePath string) (string, error)
}

// ClientConfig holds keys and endpoints resolved from credentials or environment
type ClientConfig struct {
	Provider   string
	Model      string
	APIKey     string
	BaseURL    string
	APIVersion string
	Project    string
	Location   string
	NumCtx     int
}

type providerClient struct {
	cfg ClientConfig
}

// NewClient returns an AIClient based on configuration
func NewClient(cfg ClientConfig) AIClient {
	return &providerClient{cfg: cfg}
}

func (pc *providerClient) EmbedText(ctx context.Context, text string) ([]float32, error) {
	provider := strings.ToLower(pc.cfg.Provider)
	switch provider {
	case "openai", "openai_compatible", "openai-compatible", "groq", "mistral", "deepseek", "openrouter":
		return pc.embedOpenAI(ctx, text)
	case "google", "gemini":
		return pc.embedGemini(ctx, text)
	case "ollama":
		return pc.embedOllama(ctx, text)
	default:
		return nil, fmt.Errorf("embedding not supported for provider: %s", pc.cfg.Provider)
	}
}

func (pc *providerClient) GenerateChatStream(ctx context.Context, systemPrompt string, messages []ChatMessage, onToken func(string)) error {
	provider := strings.ToLower(pc.cfg.Provider)
	switch provider {
	case "openai", "openai_compatible", "openai-compatible", "groq", "mistral", "deepseek", "openrouter":
		return pc.streamOpenAI(ctx, systemPrompt, messages, onToken)
	case "anthropic":
		return pc.streamAnthropic(ctx, systemPrompt, messages, onToken)
	case "google", "gemini":
		return pc.streamGemini(ctx, systemPrompt, messages, onToken)
	case "ollama":
		return pc.streamOllama(ctx, systemPrompt, messages, onToken)
	default:
		return fmt.Errorf("chat streaming not supported for provider: %s", pc.cfg.Provider)
	}
}

func (pc *providerClient) GenerateText(ctx context.Context, systemPrompt string, userPrompt string) (string, error) {
	var buf strings.Builder
	messages := []ChatMessage{{Role: "user", Content: userPrompt}}
	err := pc.GenerateChatStream(ctx, systemPrompt, messages, func(token string) {
		buf.WriteString(token)
	})
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

// --- OpenAI Ingestion & Streaming ---

func (pc *providerClient) embedOpenAI(ctx context.Context, text string) ([]float32, error) {
	baseURL := pc.cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	url := strings.TrimSuffix(baseURL, "/") + "/embeddings"

	reqBody, _ := json.Marshal(map[string]any{
		"model": pc.cfg.Model,
		"input": text,
	})

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if pc.cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+pc.cfg.APIKey)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("OpenAI embedding error (%d): %s", resp.StatusCode, string(bodyBytes))
	}

	var res struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}
	if len(res.Data) == 0 {
		return nil, fmt.Errorf("empty embedding response from OpenAI compatible API")
	}
	return res.Data[0].Embedding, nil
}

func (pc *providerClient) streamOpenAI(ctx context.Context, systemPrompt string, messages []ChatMessage, onToken func(string)) error {
	baseURL := pc.cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	url := strings.TrimSuffix(baseURL, "/") + "/chat/completions"

	var formattedMessages []map[string]any
	if systemPrompt != "" {
		formattedMessages = append(formattedMessages, map[string]any{
			"role":    "system",
			"content": systemPrompt,
		})
	}
	for _, m := range messages {
		formattedMessages = append(formattedMessages, map[string]any{
			"role":    m.Role,
			"content": m.Content,
		})
	}

	reqBody, _ := json.Marshal(map[string]any{
		"model":    pc.cfg.Model,
		"messages": formattedMessages,
		"stream":   true,
	})

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if pc.cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+pc.cfg.APIKey)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("OpenAI stream error (%d): %s", resp.StatusCode, string(bodyBytes))
	}

	reader := bufio.NewReader(resp.Body)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data:") {
			continue
		}

		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			break
		}

		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err == nil {
			if len(chunk.Choices) > 0 {
				token := chunk.Choices[0].Delta.Content
				if token != "" {
					onToken(token)
				}
			}
		}
	}
	return nil
}

// --- Anthropic Ingestion & Streaming ---

func (pc *providerClient) streamAnthropic(ctx context.Context, systemPrompt string, messages []ChatMessage, onToken func(string)) error {
	url := "https://api.anthropic.com/v1/messages"

	var formattedMessages []map[string]any
	for _, m := range messages {
		// Anthropic does not support 'system' role in messages list; it must be passed as top-level parameter
		role := m.Role
		if role == "system" {
			role = "user" // Fallback if system messages are in array
		}
		formattedMessages = append(formattedMessages, map[string]any{
			"role":    role,
			"content": m.Content,
		})
	}

	bodyMap := map[string]any{
		"model":      pc.cfg.Model,
		"messages":   formattedMessages,
		"stream":     true,
		"max_tokens": 4000,
	}
	if systemPrompt != "" {
		bodyMap["system"] = systemPrompt
	}

	reqBody, _ := json.Marshal(bodyMap)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", pc.cfg.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Anthropic stream error (%d): %s", resp.StatusCode, string(bodyBytes))
	}

	reader := bufio.NewReader(resp.Body)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data:") {
			continue
		}

		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		var event struct {
			Type  string `json:"type"`
			Delta struct {
				Text string `json:"text"`
			} `json:"delta"`
		}
		if err := json.Unmarshal([]byte(data), &event); err == nil {
			if event.Type == "content_block_delta" && event.Delta.Text != "" {
				onToken(event.Delta.Text)
			}
		}
	}
	return nil
}

// --- Google Gemini Ingestion & Streaming ---

func (pc *providerClient) embedGemini(ctx context.Context, text string) ([]float32, error) {
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:embedContent?key=%s", pc.cfg.Model, pc.cfg.APIKey)

	reqBody, _ := json.Marshal(map[string]any{
		"content": map[string]any{
			"parts": []map[string]any{
				{"text": text},
			},
		},
	})

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Gemini embedding error (%d): %s", resp.StatusCode, string(bodyBytes))
	}

	var res struct {
		Embedding struct {
			Values []float32 `json:"values"`
		} `json:"embedding"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}
	return res.Embedding.Values, nil
}

func (pc *providerClient) streamGemini(ctx context.Context, systemPrompt string, messages []ChatMessage, onToken func(string)) error {
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:streamGenerateContent?key=%s", pc.cfg.Model, pc.cfg.APIKey)

	type Part struct {
		Text string `json:"text"`
	}
	type Content struct {
		Role  string `json:"role,omitempty"`
		Parts []Part `json:"parts"`
	}

	var contents []Content
	if systemPrompt != "" {
		contents = append(contents, Content{
			Role:  "user",
			Parts: []Part{{Text: "System Instruction:\n" + systemPrompt}},
		})
		contents = append(contents, Content{
			Role:  "model",
			Parts: []Part{{Text: "Acknowledged."}},
		})
	}

	for _, m := range messages {
		role := m.Role
		if role == "system" {
			role = "user"
		} else if role == "assistant" {
			role = "model"
		}
		contents = append(contents, Content{
			Role:  role,
			Parts: []Part{{Text: m.Content}},
		})
	}

	reqBody, _ := json.Marshal(map[string]any{
		"contents": contents,
	})

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Gemini stream error (%d): %s", resp.StatusCode, string(bodyBytes))
	}

	// Gemini returns chunks of SSE or array of json content blocks depending on config/API version.
	// Historically, v1beta streamGenerateContent returns a JSON array chunk stream:
	// [{"candidates": [{"content": {"parts": [{"text": "..."}]}}]}]
	// Let's decode it token by token using a json Decoder.
	dec := json.NewDecoder(resp.Body)
	// Read open bracket [
	t, err := dec.Token()
	if err != nil {
		// Try parsing as single object if not array
		var singleObj struct {
			Candidates []struct {
				Content struct {
					Parts []Part `json:"parts"`
				} `json:"content"`
			} `json:"candidates"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&singleObj); err == nil {
			for _, cand := range singleObj.Candidates {
				for _, p := range cand.Content.Parts {
					onToken(p.Text)
				}
			}
		}
		return nil
	}

	// If it was array, it starts with '['
	if delim, ok := t.(json.Delim); ok && delim == '[' {
		for dec.More() {
			var chunk struct {
				Candidates []struct {
					Content struct {
						Parts []Part `json:"parts"`
					} `json:"content"`
				} `json:"candidates"`
			}
			if err := dec.Decode(&chunk); err == nil {
				for _, cand := range chunk.Candidates {
					for _, p := range cand.Content.Parts {
						onToken(p.Text)
					}
				}
			}
		}
	}
	return nil
}

// --- Ollama Ingestion & Streaming ---

func (pc *providerClient) embedOllama(ctx context.Context, text string) ([]float32, error) {
	baseURL := pc.cfg.BaseURL
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	url := strings.TrimSuffix(baseURL, "/") + "/api/embeddings"

	reqBody, _ := json.Marshal(map[string]any{
		"model":  pc.cfg.Model,
		"prompt": text,
	})

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Ollama embedding error (%d): %s", resp.StatusCode, string(bodyBytes))
	}

	var res struct {
		Embedding []float32 `json:"embedding"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}
	return res.Embedding, nil
}

func (pc *providerClient) streamOllama(ctx context.Context, systemPrompt string, messages []ChatMessage, onToken func(string)) error {
	baseURL := pc.cfg.BaseURL
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	url := strings.TrimSuffix(baseURL, "/") + "/api/chat"

	var formattedMessages []map[string]any
	if systemPrompt != "" {
		formattedMessages = append(formattedMessages, map[string]any{
			"role":    "system",
			"content": systemPrompt,
		})
	}
	for _, m := range messages {
		formattedMessages = append(formattedMessages, map[string]any{
			"role":    m.Role,
			"content": m.Content,
		})
	}

	options := map[string]any{}
	if pc.cfg.NumCtx > 0 {
		options["num_ctx"] = pc.cfg.NumCtx
	}

	reqBody, _ := json.Marshal(map[string]any{
		"model":    pc.cfg.Model,
		"messages": formattedMessages,
		"stream":   true,
		"options":  options,
	})

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Ollama stream error (%d): %s", resp.StatusCode, string(bodyBytes))
	}

	dec := json.NewDecoder(resp.Body)
	for {
		var chunk struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			Done bool `json:"done"`
		}
		if err := dec.Decode(&chunk); err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		if chunk.Message.Content != "" {
			onToken(chunk.Message.Content)
		}
		if chunk.Done {
			break
		}
	}
	return nil
}

func (pc *providerClient) GenerateSpeech(ctx context.Context, text string, voice string) ([]byte, error) {
	provider := strings.ToLower(pc.cfg.Provider)
	switch provider {
	case "openai", "openai_compatible", "openai-compatible":
		return pc.generateSpeechOpenAI(ctx, text, voice)
	case "google", "gemini":
		return pc.generateSpeechGemini(ctx, text, voice)
	default:
		return nil, fmt.Errorf("speech generation not supported for provider: %s", pc.cfg.Provider)
	}
}

func (pc *providerClient) TranscribeAudio(ctx context.Context, filePath string) (string, error) {
	provider := strings.ToLower(pc.cfg.Provider)
	switch provider {
	case "openai", "openai_compatible", "openai-compatible", "groq":
		return pc.transcribeAudioOpenAI(ctx, filePath)
	case "google", "gemini":
		return pc.transcribeAudioGemini(ctx, filePath)
	default:
		return "", fmt.Errorf("transcription not supported for provider: %s", pc.cfg.Provider)
	}
}

func (pc *providerClient) generateSpeechOpenAI(ctx context.Context, text, voice string) ([]byte, error) {
	baseURL := pc.cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	url := strings.TrimSuffix(baseURL, "/") + "/audio/speech"

	reqBody, _ := json.Marshal(map[string]any{
		"model":           pc.cfg.Model,
		"voice":           voice,
		"input":           text,
		"response_format": "mp3",
	})

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if pc.cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+pc.cfg.APIKey)
	}

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("OpenAI TTS error (%d): %s", resp.StatusCode, string(bodyBytes))
	}

	return io.ReadAll(resp.Body)
}

func (pc *providerClient) generateSpeechGemini(ctx context.Context, text, voice string) ([]byte, error) {
	baseHost := os.Getenv("GEMINI_API_BASE_URL")
	if baseHost == "" {
		baseHost = "https://generativelanguage.googleapis.com"
	}
	model := pc.cfg.Model
	if model == "" {
		model = "gemini-3.1-flash-tts-preview"
	}
	url := fmt.Sprintf("%s/v1beta/models/%s:generateContent?key=%s", strings.TrimSuffix(baseHost, "/"), model, pc.cfg.APIKey)

	payload := map[string]any{
		"contents": []any{
			map[string]any{
				"parts": []any{
					map[string]any{"text": text},
				},
			},
		},
		"generationConfig": map[string]any{
			"responseModalities": []string{"AUDIO"},
			"speechConfig": map[string]any{
				"voiceConfig": map[string]any{
					"prebuiltVoiceConfig": map[string]any{
						"voiceName": voice,
					},
				},
			},
		},
	}

	if strings.HasPrefix(model, "gemini-2.5") {
		payload["systemInstruction"] = map[string]any{
			"parts": []any{
				map[string]any{"text": "Read aloud the following text."},
			},
		}
	}

	reqBody, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Gemini TTS error (%d): %s", resp.StatusCode, string(bodyBytes))
	}

	var res struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					InlineData struct {
						MimeType string `json:"mimeType"`
						Data     string `json:"data"`
					} `json:"inlineData"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}

	if len(res.Candidates) == 0 || len(res.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("empty audio candidate returned from Gemini")
	}

	b64Data := res.Candidates[0].Content.Parts[0].InlineData.Data
	pcmData, err := base64.StdEncoding.DecodeString(b64Data)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64 audio: %w", err)
	}

	return convertPCMToWav(pcmData, 24000, 16, 1)
}

func convertPCMToWav(pcmData []byte, sampleRate int, bitDepth int, channels int) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString("RIFF")
	dataSize := len(pcmData)
	fileSize := uint32(dataSize + 36)
	if err := binary.Write(&buf, binary.LittleEndian, fileSize); err != nil {
		return nil, err
	}
	buf.WriteString("WAVE")
	buf.WriteString("fmt ")
	chunkSize := uint32(16)
	if err := binary.Write(&buf, binary.LittleEndian, chunkSize); err != nil {
		return nil, err
	}
	audioFormat := uint16(1) // PCM
	if err := binary.Write(&buf, binary.LittleEndian, audioFormat); err != nil {
		return nil, err
	}
	numChannels := uint16(channels)
	if err := binary.Write(&buf, binary.LittleEndian, numChannels); err != nil {
		return nil, err
	}
	sampleRateVal := uint32(sampleRate)
	if err := binary.Write(&buf, binary.LittleEndian, sampleRateVal); err != nil {
		return nil, err
	}
	byteRate := uint32(sampleRate * channels * bitDepth / 8)
	if err := binary.Write(&buf, binary.LittleEndian, byteRate); err != nil {
		return nil, err
	}
	blockAlign := uint16(channels * bitDepth / 8)
	if err := binary.Write(&buf, binary.LittleEndian, blockAlign); err != nil {
		return nil, err
	}
	bitsPerSample := uint16(bitDepth)
	if err := binary.Write(&buf, binary.LittleEndian, bitsPerSample); err != nil {
		return nil, err
	}
	buf.WriteString("data")
	if err := binary.Write(&buf, binary.LittleEndian, uint32(dataSize)); err != nil {
		return nil, err
	}
	buf.Write(pcmData)
	return buf.Bytes(), nil
}

func (pc *providerClient) transcribeAudioOpenAI(ctx context.Context, filePath string) (string, error) {
	baseURL := pc.cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	url := strings.TrimSuffix(baseURL, "/") + "/audio/transcriptions"

	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(part, file); err != nil {
		return "", err
	}

	_ = writer.WriteField("model", pc.cfg.Model)
	_ = writer.WriteField("response_format", "json")
	_ = writer.Close()

	req, err := http.NewRequestWithContext(ctx, "POST", url, body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if pc.cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+pc.cfg.APIKey)
	}

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("OpenAI STT error (%d): %s", resp.StatusCode, string(bodyBytes))
	}

	var res struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", err
	}
	return res.Text, nil
}

func (pc *providerClient) transcribeAudioGemini(ctx context.Context, filePath string) (string, error) {
	baseHost := os.Getenv("GEMINI_API_BASE_URL")
	if baseHost == "" {
		baseHost = "https://generativelanguage.googleapis.com"
	}
	model := pc.cfg.Model
	if model == "" {
		model = "gemini-2.5-flash"
	}
	url := fmt.Sprintf("%s/v1beta/models/%s:generateContent?key=%s", strings.TrimSuffix(baseHost, "/"), model, pc.cfg.APIKey)

	audioBytes, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	audioBase64 := base64.StdEncoding.EncodeToString(audioBytes)

	mimeType := "audio/mp3"
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".wav":
		mimeType = "audio/wav"
	case ".aac":
		mimeType = "audio/aac"
	case ".ogg":
		mimeType = "audio/ogg"
	case ".flac":
		mimeType = "audio/flac"
	}

	payload := map[string]any{
		"contents": []any{
			map[string]any{
				"parts": []any{
					map[string]any{"text": "Generate a transcript of the speech in this audio file."},
					map[string]any{
						"inline_data": map[string]any{
							"mime_type": mimeType,
							"data":      audioBase64,
						},
					},
				},
			},
		},
	}

	reqBody, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("Gemini STT error (%d): %s", resp.StatusCode, string(bodyBytes))
	}

	var res struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", err
	}

	if len(res.Candidates) == 0 || len(res.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("empty transcription candidate returned from Gemini")
	}

	return res.Candidates[0].Content.Parts[0].Text, nil
}
