package extractor

import (
	"context"
	"errors"
	"go-notebook/internal/ai"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type mockVisionClient struct {
	ai.AIClient
	analyzeImageFn func(ctx context.Context, filePath string, prompt string) (string, error)
}

func (m *mockVisionClient) AnalyzeImage(ctx context.Context, filePath string, prompt string) (string, error) {
	if m.analyzeImageFn != nil {
		return m.analyzeImageFn(ctx, filePath, prompt)
	}
	return "", nil
}

func TestExtractTextFromImage_TesseractSuccess(t *testing.T) {
	// Setup dummy image file
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.png")
	if err := os.WriteFile(filePath, []byte("fake image content"), 0644); err != nil {
		t.Fatalf("failed to create dummy image: %v", err)
	}

	// Mock LookPath and Command execution
	origLookPath := lookPath
	origRunCommand := runCommand
	defer func() {
		lookPath = origLookPath
		runCommand = origRunCommand
	}()

	lookPath = func(binary string) (string, error) {
		if binary == "tesseract" {
			return "/usr/bin/tesseract", nil
		}
		return "", errors.New("not found")
	}

	runCommand = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if name == "tesseract" && len(args) == 2 && args[0] == filePath && args[1] == "stdout" {
			return []byte("transcribed text from local tesseract"), nil
		}
		return nil, errors.New("mock command failed")
	}

	ctx := context.Background()
	client := &mockVisionClient{}

	result, err := ExtractTextFromImage(ctx, client, filePath)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	expected := "transcribed text from local tesseract"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestExtractTextFromImage_TesseractFailure_LLMFallback(t *testing.T) {
	// Setup dummy image file
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.jpg")
	if err := os.WriteFile(filePath, []byte("fake image content"), 0644); err != nil {
		t.Fatalf("failed to create dummy image: %v", err)
	}

	// Mock LookPath and Command execution
	origLookPath := lookPath
	origRunCommand := runCommand
	defer func() {
		lookPath = origLookPath
		runCommand = origRunCommand
	}()

	// Simulate tesseract missing
	lookPath = func(binary string) (string, error) {
		return "", errors.New("tesseract not found")
	}

	clientCalled := false
	client := &mockVisionClient{
		analyzeImageFn: func(ctx context.Context, fp string, prompt string) (string, error) {
			if fp != filePath {
				return "", errors.New("unexpected file path in mock client")
			}
			clientCalled = true
			return "# Whiteboard Notes\n- Hand-drawn diagram description\n- Transcribed text: fallback text", nil
		},
	}

	ctx := context.Background()
	result, err := ExtractTextFromImage(ctx, client, filePath)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !clientCalled {
		t.Error("expected AIClient.AnalyzeImage to be called, but it was not")
	}

	expectedContains := "Transcribed text: fallback text"
	if !strings.Contains(result, expectedContains) {
		t.Errorf("expected result to contain %q, but got %q", expectedContains, result)
	}
}
