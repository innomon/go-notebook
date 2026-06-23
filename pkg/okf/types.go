// Package okf implements the Open Knowledge Format (OKF) specification for structured metadata parsing,
// validation, and graph indexing.
package okf

import (
	"errors"
	"time"
)

var (
	// ErrNoFrontmatter is returned when a document lacks a valid OKF YAML frontmatter block.
	ErrNoFrontmatter = errors.New("file does not contain valid OKF frontmatter")

	// ErrMissingFields is returned when a document lacks the required OKF fields: type, title, or description.
	ErrMissingFields = errors.New("missing mandatory OKF fields (type, title, description)")
)

// Metadata represents the standardized Open Knowledge Format frontmatter header.
type Metadata struct {
	Type        string                 `yaml:"type"`                 // Required: e.g., "Go Function", "Prompt Template", "SQL Query"
	Title       string                 `yaml:"title"`                // Required: Short descriptive name
	Description string                 `yaml:"description"`          // Required: Summary of intent/capabilities
	Resource    string                 `yaml:"resource,omitempty"`   // Optional: Relative file path, URI, or target module identifier
	Tags        []string               `yaml:"tags,omitempty"`       // Optional: Categorization index slices
	Timestamp   time.Time              `yaml:"timestamp,omitempty"`  // Optional: Last verified or compiled epoch
	Properties  map[string]interface{} `yaml:"properties,omitempty"` // Optional: Extensible custom metadata block
}

// BundleNode represents a node within the local knowledge graph array.
type BundleNode struct {
	ID            string   `json:"id"`       // Unique node identifier (Relative filepath or UUID)
	Metadata      Metadata `json:"metadata"` // Raw structured OKF metadata block
	OutboundLinks []string `json:"outbound"` // Extracted slice of local file relative path mappings
}
