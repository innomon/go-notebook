package bases

import (
	"errors"
	"testing"
)

func TestExecute_FilterAndTable(t *testing.T) {
	notes := []*Note{
		{
			FilePath:   "note1.md",
			Content:    "Content of note 1",
			Properties: map[string]any{"status": "active", "priority": 5, "title": "Note One"},
		},
		{
			FilePath:   "note2.md",
			Content:    "Content of note 2",
			Properties: map[string]any{"status": "inactive", "priority": 4, "title": "Note Two"},
		},
		{
			FilePath:   "note3.md",
			Content:    "Content of note 3",
			Properties: map[string]any{"status": "active", "priority": 2, "title": "Note Three"},
		},
	}

	config := &BaseConfig{
		ViewType: "table",
		Filters: []FilterCondition{
			{Property: "status", Operator: "eq", Value: "active"},
			{Property: "priority", Operator: "gt", Value: 3},
		},
		Formulas: map[string]string{
			"double_priority": "double_formula",
		},
	}

	// Mock evaluator
	mockRunFormula := func(funcName string, properties map[string]any) (any, error) {
		if funcName == "double_formula" {
			prio, ok := properties["priority"]
			if !ok {
				return nil, errors.New("priority not found")
			}
			switch v := prio.(type) {
			case int:
				return v * 2, nil
			case float64:
				return v * 2, nil
			}
		}
		return nil, errors.New("unknown function")
	}

	resp, err := Execute(notes, config, mockRunFormula)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if resp.Type != "table" {
		t.Errorf("Expected response type 'table', got '%s'", resp.Type)
	}

	// Only note1 should match filters (status=active, priority=5 > 3)
	if len(resp.Rows) != 1 {
		t.Errorf("Expected 1 row, got %d", len(resp.Rows))
	} else {
		row := resp.Rows[0]
		if row["title"] != "Note One" {
			t.Errorf("Expected row title 'Note One', got '%v'", row["title"])
		}
		if val, ok := row["double_priority"]; !ok || val != 10 {
			t.Errorf("Expected double_priority formula to evaluate to 10, got %v", val)
		}
	}

	// Validate columns are generated
	if len(resp.Columns) == 0 {
		t.Errorf("Expected columns to be populated")
	}
}

func TestExecute_CardView(t *testing.T) {
	notes := []*Note{
		{
			FilePath:   "note1.md",
			Content:    "Content of note 1",
			Properties: map[string]any{"status": "active", "priority": 5, "title": "Note One"},
		},
	}

	config := &BaseConfig{
		ViewType: "card-grid",
		Filters:  []FilterCondition{},
		Formulas: map[string]string{
			"custom_label": "greet_formula",
		},
	}

	mockRunFormula := func(funcName string, properties map[string]any) (any, error) {
		if funcName == "greet_formula" {
			return "Hello Note One", nil
		}
		return nil, nil
	}

	resp, err := Execute(notes, config, mockRunFormula)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if resp.Type != "card-grid" {
		t.Errorf("Expected response type 'card-grid', got '%s'", resp.Type)
	}

	if len(resp.Items) != 1 {
		t.Fatalf("Expected 1 item, got %d", len(resp.Items))
	}

	item := resp.Items[0]
	if item.Title != "Note One" {
		t.Errorf("Expected item title 'Note One', got '%s'", item.Title)
	}

	// Verify formula column is added to properties
	found := false
	for _, prop := range item.Properties {
		if prop.Label == "custom_label" {
			found = true
			if prop.Value != "Hello Note One" {
				t.Errorf("Expected custom_label property value 'Hello Note One', got '%v'", prop.Value)
			}
		}
	}
	if !found {
		t.Errorf("Expected custom_label property to be present in card item properties")
	}
}

func TestEngine_CoverageOperators(t *testing.T) {
	notes := []*Note{
		{
			FilePath: "note_cov.md",
			Properties: map[string]any{
				"status":    "inactive",
				"priority":  int64(2),
				"weight":    float32(1.5),
				"score":     float64(8.5),
				"tags":      []any{"work", "urgent"},
				"category":  "documentation",
				"is_draft":  true,
			},
		},
	}

	mockRun := func(name string, p map[string]any) (any, error) {
		return nil, nil
	}

	tests := []struct {
		operator string
		prop     string
		val      any
		matches  bool
	}{
		{"ne", "status", "active", true},
		{"ne", "status", "inactive", false},
		{"lt", "priority", 3, true},
		{"lt", "priority", 1, false},
		{"gt", "weight", 1.0, true},
		{"gt", "weight", 2.0, false},
		{"gt", "score", 8.0, true},
		{"gt", "score", 9.0, false},
		{"contains", "tags", "work", true},
		{"contains", "tags", "leisure", false},
		{"contains", "category", "ment", true},
		{"contains", "category", "code", false},
		// Test cases for mismatching types in gt/lt/contains
		{"gt", "status", 5, false},
		{"lt", "status", 5, false},
		{"contains", "status", 5, false},
		{"contains", "tags", 5, false},
		{"unknown", "status", 5, false},
	}

	for _, tt := range tests {
		config := &BaseConfig{
			ViewType: "table",
			Filters: []FilterCondition{
				{Property: tt.prop, Operator: tt.operator, Value: tt.val},
			},
		}
		resp, err := Execute(notes, config, mockRun)
		if err != nil {
			t.Fatalf("Unexpected error for operator %s: %v", tt.operator, err)
		}
		matched := len(resp.Rows) == 1
		if matched != tt.matches {
			t.Errorf("Expected filter %s %s %v to result in match=%t, got %t", tt.prop, tt.operator, tt.val, tt.matches, matched)
		}
	}
}

