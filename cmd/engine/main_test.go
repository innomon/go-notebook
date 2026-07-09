package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestEngineCLI_Integration(t *testing.T) {
	// 1. Create a temporary workspace directory
	tempDir, err := os.MkdirTemp("", "base_test_workspace")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 2. Write mock notes
	note1Content := `---
title: Note A
created_at: 2026-07-01
status: active
---
Content A`
	if err := os.WriteFile(filepath.Join(tempDir, "note1.md"), []byte(note1Content), 0644); err != nil {
		t.Fatalf("Failed to write note1: %v", err)
	}

	note2Content := `---
title: Note B
created_at: 2026-07-08
status: inactive
---
Content B`
	if err := os.WriteFile(filepath.Join(tempDir, "note2.md"), []byte(note2Content), 0644); err != nil {
		t.Fatalf("Failed to write note2: %v", err)
	}

	// 3. Write config file
	configContent := `
view_type: table
filters:
  - property: status
    operator: eq
    value: active
formulas:
  age: calculate_days_since
`
	configPath := filepath.Join(tempDir, "config.base")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	// 4. Build CLI binary
	binPath := filepath.Join(tempDir, "engine_cli")
	buildCmd := exec.Command("go", "build", "-o", binPath, "main.go")
	buildCmd.Dir = "."
	var buildErr bytes.Buffer
	buildCmd.Stderr = &buildErr
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build CLI binary: %v (stderr: %s)", err, buildErr.String())
	}

	// 5. Run CLI
	// Pass extensions dir relative to current package directory (which is cmd/engine/ -> ../../extensions/bin)
	cmd := exec.Command(binPath, "-dir", tempDir, "-config", configPath, "-extensions", "../../extensions/bin")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("CLI execution failed: %v (stderr: %s)", err, stderr.String())
	}

	// 6. Verify stdout matches A2UI response
	var response struct {
		Type string           `json:"type"`
		Rows []map[string]any `json:"rows"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse CLI stdout as JSON: %v (stdout: %s)", err, stdout.String())
	}

	if response.Type != "table" {
		t.Errorf("Expected response type 'table', got '%s'", response.Type)
	}

	// Note A should match filter (status=active), Note B (inactive) should be filtered out
	if len(response.Rows) != 1 {
		t.Fatalf("Expected 1 row in response, got %d", len(response.Rows))
	}

	row := response.Rows[0]
	if row["title"] != "Note A" {
		t.Errorf("Expected matching note title 'Note A', got '%v'", row["title"])
	}

	// Formula age evaluated (from 2026-07-01 to 2026-07-09) -> 8 days
	// Note: First arg defaults to "created_at" in calculate_days_since main.go
	if val, ok := row["age"]; !ok {
		t.Errorf("Expected computed formula column 'age' to be present")
	} else {
		// Float64 unmarshal
		if valFloat, ok := val.(float64); !ok || valFloat != 8 {
			t.Errorf("Expected age value 8, got %v (%T)", val, val)
		}
	}
}
