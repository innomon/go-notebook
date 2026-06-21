package extractor

import (
	"context"
	"errors"
	"fmt"
	"go-notebook/internal/ai"
	"os"
	"os/exec"
	"strings"
)

var (
	// Package-level variables to allow mocking in tests
	lookPath   = exec.LookPath
	runCommand = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		cmd := exec.CommandContext(ctx, name, args...)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return nil, fmt.Errorf("command %s failed: %w (output: %s)", name, err, string(output))
		}
		return output, nil
	}
)

// ExtractTextFromImage extracts text from an image file (PNG, JPG, JPEG, WEBP).
// It attempts to run local Tesseract OCR. If Tesseract is unavailable or fails,
// it falls back to the configured LLM Vision API.
func ExtractTextFromImage(ctx context.Context, aiClient ai.AIClient, filePath string) (string, error) {
	// 1. Verify file exists
	if _, err := os.Stat(filePath); err != nil {
		return "", fmt.Errorf("image file not found: %w", err)
	}

	// 2. Check if tesseract binary is available
	_, lookErr := lookPath("tesseract")
	if lookErr == nil {
		// Tesseract is available, try running it
		output, err := runCommand(ctx, "tesseract", filePath, "stdout")
		if err == nil {
			return strings.TrimSpace(string(output)), nil
		}
		// If Tesseract failed, log/proceed to LLM Vision fallback
	}

	// 3. Fallback to LLM Vision API
	if aiClient == nil {
		return "", errors.New("tesseract failed or missing, and no AI client configured for fallback")
	}

	prompt := "Perform OCR on this image to extract all text. Additionally, describe any drawings, diagrams, charts, flowcharts, structures, or visual layout in detail. Output the result as a structured Markdown document containing both the transcribed text and the visual descriptions."
	
	result, err := aiClient.AnalyzeImage(ctx, filePath, prompt)
	if err != nil {
		return "", fmt.Errorf("both tesseract and LLM vision extraction failed: %w", err)
	}

	return result, nil
}

// SetTesseractMock sets the mock functions for LookPath and runCommand.
// It returns a function that restores the original functions when called (typically deferred).
func SetTesseractMock(lp func(string) (string, error), rc func(context.Context, string, ...string) ([]byte, error)) func() {
	origLookPath := lookPath
	origRunCommand := runCommand
	lookPath = lp
	runCommand = rc
	return func() {
		lookPath = origLookPath
		runCommand = origRunCommand
	}
}
