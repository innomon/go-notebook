package bases

import (
	"testing"
)

func TestParseNote_WithYAMLFrontmatter(t *testing.T) {
	markdown := `---
title: My First Base Note
status: active
priority: 5
tags:
  - note
  - test
---
# Main Content
This is the body of the note.`

	note, err := ParseNote("test_note.md", markdown)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if note.FilePath != "test_note.md" {
		t.Errorf("Expected FilePath to be 'test_note.md', got '%s'", note.FilePath)
	}

	if note.Content != "# Main Content\nThis is the body of the note." {
		t.Errorf("Expected Content to match body, got '%s'", note.Content)
	}

	title, ok := note.Properties["title"]
	if !ok || title != "My First Base Note" {
		t.Errorf("Expected title property to be 'My First Base Note', got '%v'", title)
	}

	status, ok := note.Properties["status"]
	if !ok || status != "active" {
		t.Errorf("Expected status property to be 'active', got '%v'", status)
	}

	priority, ok := note.Properties["priority"]
	if !ok {
		t.Errorf("Expected priority property to exist")
	} else {
		// YAML numbers can be parsed as int or float depending on decoder
		// Let's assert they match numeric 5
		switch v := priority.(type) {
		case int:
			if v != 5 {
				t.Errorf("Expected priority 5, got %d", v)
			}
		case int64:
			if v != 5 {
				t.Errorf("Expected priority 5, got %d", v)
			}
		case float64:
			if v != 5.0 {
				t.Errorf("Expected priority 5.0, got %f", v)
			}
		default:
			t.Errorf("Unexpected type for priority: %T", priority)
		}
	}
}

func TestParseNote_NoFrontmatter(t *testing.T) {
	markdown := `# Main Content Only
This note has no YAML frontmatter.`

	note, err := ParseNote("no_frontmatter.md", markdown)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if note.Content != "# Main Content Only\nThis note has no YAML frontmatter." {
		t.Errorf("Expected content to match, got '%s'", note.Content)
	}

	if len(note.Properties) != 0 {
		t.Errorf("Expected properties to be empty, got %d items", len(note.Properties))
	}
}

func TestLoadBaseConfig_YAML(t *testing.T) {
	configYAML := `
filters:
  - property: status
    operator: eq
    value: active
  - property: priority
    operator: gt
    value: 3
view_type: table
formulas:
  age_in_days: calculate_days_since
host_permissions:
  read_other_notes: true
  access_env: false
`
	config, err := LoadBaseConfig([]byte(configYAML))
	if err != nil {
		t.Fatalf("Expected no error loading config, got %v", err)
	}

	if config.ViewType != "table" {
		t.Errorf("Expected view_type 'table', got '%s'", config.ViewType)
	}

	if len(config.Filters) != 2 {
		t.Errorf("Expected 2 filters, got %d", len(config.Filters))
	}

	if config.Filters[0].Property != "status" || config.Filters[0].Operator != "eq" || config.Filters[0].Value != "active" {
		t.Errorf("First filter does not match expected, got %+v", config.Filters[0])
	}

	if config.Filters[1].Property != "priority" || config.Filters[1].Operator != "gt" {
		t.Errorf("Second filter does not match expected, got %+v", config.Filters[1])
	}

	wasmFunc, ok := config.Formulas["age_in_days"]
	if !ok || wasmFunc != "calculate_days_since" {
		t.Errorf("Expected formula 'age_in_days' to map to 'calculate_days_since', got '%s'", wasmFunc)
	}

	if !config.HostPermissions.ReadOtherNotes {
		t.Errorf("Expected read_other_notes to be true")
	}

	if config.HostPermissions.AccessEnv {
		t.Errorf("Expected access_env to be false")
	}
}
