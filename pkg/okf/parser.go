package okf

import (
	"bufio"
	"bytes"
	"io"
	"strings"

	"gopkg.in/yaml.v3"
)

// ParseDocument splits the file content from the reader into structured Metadata
// and a clean Markdown body byte array. It validates that the document starts with
// a valid YAML frontmatter block and contains the mandatory fields: type, title,
// and description.
func ParseDocument(r io.Reader) (*Metadata, []byte, error) {
	scanner := bufio.NewScanner(r)
	var frontmatterBuf bytes.Buffer
	var bodyBuf bytes.Buffer
	inFrontmatter := false
	frontmatterChecked := false
	lineCount := 0

	for scanner.Scan() {
		line := scanner.Text()
		lineCount++
		if lineCount == 1 {
			if strings.TrimSpace(line) == "---" {
				inFrontmatter = true
				continue
			}
		}
		if inFrontmatter {
			if strings.TrimSpace(line) == "---" {
				inFrontmatter = false
				frontmatterChecked = true
				continue
			}
			frontmatterBuf.WriteString(line + "\n")
		} else {
			bodyBuf.WriteString(line + "\n")
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, nil, err
	}

	if !frontmatterChecked {
		return nil, bodyBuf.Bytes(), ErrNoFrontmatter
	}

	var meta Metadata
	if err := yaml.Unmarshal(frontmatterBuf.Bytes(), &meta); err != nil {
		return nil, bodyBuf.Bytes(), err
	}

	// Dynamic Validation check for mandatory fields
	if meta.Type == "" || meta.Title == "" || meta.Description == "" {
		return &meta, bodyBuf.Bytes(), ErrMissingFields
	}

	return &meta, bodyBuf.Bytes(), nil
}
