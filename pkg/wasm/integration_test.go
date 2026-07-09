package wasm

import (
	"context"
	"encoding/json"
	"go-notebook/pkg/bases"
	"os"
	"testing"
)

func TestIntegration_ObsidianBasesAndWasm(t *testing.T) {
	ctx := context.Background()
	m, err := NewManager(ctx)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}
	defer m.Close(ctx)

	// Try loading calculate_days_since if it's there
	actualWasmBytes, err := os.ReadFile("../../extensions/bin/calculate_days_since.wasm")
	if err != nil {
		t.Skip("calculate_days_since.wasm not found, skipping integration test")
	}

	err = m.LoadPlugin(ctx, "days_since", actualWasmBytes)
	if err != nil {
		t.Fatalf("Failed to load plugin: %v", err)
	}

	notes := []*bases.Note{
		{
			FilePath:   "doc1.md",
			Properties: map[string]any{"title": "Note One", "created_at": "2026-07-01", "status": "active"},
			Content:    "Content",
		},
		{
			FilePath:   "doc2.md",
			Properties: map[string]any{"title": "Note Two", "created_at": "2026-07-08", "status": "active"},
			Content:    "Content",
		},
	}

	config := &bases.BaseConfig{
		ViewType: "table",
		Filters:  []bases.FilterCondition{},
		Formulas: map[string]string{
			"age_days": "calculate_days_since",
		},
	}

	runFormula := func(funcName string, properties map[string]any) (any, error) {
		payload := struct {
			Properties map[string]any `json:"properties"`
			Args       []string       `json:"args"`
		}{
			Properties: properties,
			Args:       []string{"created_at", "2026-07-09"}, // Reference date is 2026-07-09
		}
		payloadBytes, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}

		resBytes, err := m.Execute(ctx, "days_since", funcName, payloadBytes)
		if err != nil {
			return nil, err
		}

		var result struct {
			Days  int    `json:"days"`
			Error string `json:"error"`
		}
		if err := json.Unmarshal(resBytes, &result); err != nil {
			return nil, err
		}
		if result.Error != "" {
			return nil, os.ErrInvalid
		}

		return result.Days, nil
	}

	resp, err := bases.Execute(notes, config, runFormula)
	if err != nil {
		t.Fatalf("Failed to execute bases engine with WASM: %v", err)
	}

	if len(resp.Rows) != 2 {
		t.Fatalf("Expected 2 rows, got %d", len(resp.Rows))
	}

	// note1 (2026-07-01 to 2026-07-09) -> 8 days
	if val, ok := resp.Rows[0]["age_days"]; !ok || val != 8 {
		t.Errorf("Expected note1 age_days to be 8, got %v", val)
	}

	// note2 (2026-07-08 to 2026-07-09) -> 1 day
	if val, ok := resp.Rows[1]["age_days"]; !ok || val != 1 {
		t.Errorf("Expected note2 age_days to be 1, got %v", val)
	}
}
