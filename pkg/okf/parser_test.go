package okf

import (
	"strings"
	"testing"
)

func TestParseDocument_ValidOKF(t *testing.T) {
	rawInput := `---
type: Go Struct
title: DataPayload
description: Contains primary parameters for system storage.
tags: [core, data-layer]
---
# Main Implementation Header
This is normal markdown text payload block.`

	meta, body, err := ParseDocument(strings.NewReader(rawInput))
	if err != nil {
		t.Fatalf("Expected zero errors parsing valid document, got: %v", err)
	}
	if meta.Type != "Go Struct" {
		t.Errorf("Expected Type 'Go Struct', extracted: '%s'", meta.Type)
	}
	if meta.Title != "DataPayload" {
		t.Errorf("Expected Title 'DataPayload', got '%s'", meta.Title)
	}
	if meta.Description != "Contains primary parameters for system storage." {
		t.Errorf("Expected Description 'Contains primary parameters for system storage.', got '%s'", meta.Description)
	}
	if !strings.Contains(string(body), "Main Implementation Header") {
		t.Errorf("Markdown body text block missing or extracted incorrectly.")
	}
}

func TestParseDocument_MissingMandatoryFields(t *testing.T) {
	rawInput := `---
title: Broken Document Schema
description: Missing explicit type parameter key block.
---
# Content Header`

	_, _, err := ParseDocument(strings.NewReader(rawInput))
	if err != ErrMissingFields {
		t.Errorf("Expected ErrMissingFields constraint violation error, caught: %v", err)
	}
}

func TestParseDocument_NoFrontmatter(t *testing.T) {
	rawInput := `# Plain Markdown Document
This document has no YAML frontmatter block at all.`

	_, _, err := ParseDocument(strings.NewReader(rawInput))
	if err != ErrNoFrontmatter {
		t.Errorf("Expected ErrNoFrontmatter error, caught: %v", err)
	}
}

func TestParseDocument_EmptyBody(t *testing.T) {
	rawInput := `---
type: Configuration
title: Config Details
description: Setting values.
---`

	meta, body, err := ParseDocument(strings.NewReader(rawInput))
	if err != nil {
		t.Fatalf("Expected zero errors, got: %v", err)
	}
	if meta.Type != "Configuration" {
		t.Errorf("Expected Type 'Configuration', got '%s'", meta.Type)
	}
	if len(body) != 0 {
		t.Errorf("Expected empty body slice, got length %d", len(body))
	}
}

func TestParseDocument_FrontmatterOnly(t *testing.T) {
	rawInput := `---
type: Configuration
title: Config Details
description: Setting values.
---
`

	meta, body, err := ParseDocument(strings.NewReader(rawInput))
	if err != nil {
		t.Fatalf("Expected zero errors, got: %v", err)
	}
	if meta.Type != "Configuration" {
		t.Errorf("Expected Type 'Configuration', got '%s'", meta.Type)
	}
	if strings.TrimSpace(string(body)) != "" {
		t.Errorf("Expected empty body, got: %q", string(body))
	}
}

func TestExtractLinks_ValidPaths(t *testing.T) {
	bodyContent := []byte(`
Review structural designs via [Linear Algebra](./math/matrix.md).
Do not follow external routing vectors like [Google](https://google.com).
Check structural logic using [State Matrix Layout](../logic/state.md).
`)

	links := ExtractLinks(bodyContent)
	if len(links) != 2 {
		t.Fatalf("Expected exactly 2 internal relative paths extracted, found: %d", len(links))
	}
	if links[0] != "./math/matrix.md" || links[1] != "../logic/state.md" {
		t.Errorf("Extracted paths mapping order is flawed or distorted: %v", links)
	}
}

func TestExtractLinks_EmptyBody(t *testing.T) {
	links := ExtractLinks([]byte(""))
	if len(links) != 0 {
		t.Errorf("Expected empty slice, got: %v", links)
	}
}

func TestExtractLinks_OnlyExternalLinks(t *testing.T) {
	bodyContent := []byte(`
Check [Google](https://google.com) or [GitHub](http://github.com)
`)
	links := ExtractLinks(bodyContent)
	if len(links) != 0 {
		t.Errorf("Expected empty slice when only external HTTP/HTTPS links exist, got: %v", links)
	}
}

func TestParseDocument_WindowsLineEndings(t *testing.T) {
	rawInput := "---\r\ntype: Go Struct\r\ntitle: WindowsDoc\r\ndescription: Document with CRLF endings.\r\n---\r\n# CRLF Content\r\nHello Windows World"
	meta, body, err := ParseDocument(strings.NewReader(rawInput))
	if err != nil {
		t.Fatalf("Expected zero errors parsing document with CRLF, got: %v", err)
	}
	if meta.Type != "Go Struct" {
		t.Errorf("Expected Type 'Go Struct', got '%s'", meta.Type)
	}
	if !strings.Contains(string(body), "Hello Windows World") {
		t.Errorf("Expected body to contain 'Hello Windows World'")
	}
}

func TestExtractLinks_ParenthesesInLabel(t *testing.T) {
	bodyContent := []byte("Check out [Label (with parens)](./math/matrix.md)")
	links := ExtractLinks(bodyContent)
	if len(links) != 1 {
		t.Fatalf("Expected exactly 1 link path, got: %d", len(links))
	}
	if links[0] != "./math/matrix.md" {
		t.Errorf("Expected extracted link to be './math/matrix.md', got '%s'", links[0])
	}
}
