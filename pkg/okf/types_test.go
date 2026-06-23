package okf

import (
	"testing"
	"time"
)

// TestTypesCompilation ensures that the defined types and fields compile and can be instantiated.
func TestTypesCompilation(t *testing.T) {
	meta := Metadata{
		Type:        "test-type",
		Title:       "test-title",
		Description: "test-description",
		Resource:    "test-resource",
		Tags:        []string{"tag1", "tag2"},
		Timestamp:   time.Now(),
		Properties:  map[string]interface{}{"key": "value"},
	}

	node := BundleNode{
		ID:            "test-id",
		Metadata:      meta,
		OutboundLinks: []string{"link1"},
	}

	if node.ID != "test-id" {
		t.Errorf("expected ID 'test-id', got '%s'", node.ID)
	}

	if ErrNoFrontmatter == nil {
		t.Error("expected ErrNoFrontmatter to be initialized")
	}

	if ErrMissingFields == nil {
		t.Error("expected ErrMissingFields to be initialized")
	}
}
