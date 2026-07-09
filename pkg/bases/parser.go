package bases

import (
	"strings"

	"gopkg.in/yaml.v3"
)

// ParseNote parses a markdown file content, extracting the YAML frontmatter properties
// and the body content.
func ParseNote(filePath string, markdownContent string) (*Note, error) {
	// Normalize line endings
	markdownContent = strings.ReplaceAll(markdownContent, "\r\n", "\n")
	lines := strings.Split(markdownContent, "\n")

	if len(lines) > 0 && strings.TrimSpace(lines[0]) == "---" {
		// Find closing ---
		endIdx := -1
		for i := 1; i < len(lines); i++ {
			if strings.TrimSpace(lines[i]) == "---" {
				endIdx = i
				break
			}
		}

		if endIdx != -1 {
			yamlContent := strings.Join(lines[1:endIdx], "\n")
			bodyContent := strings.Join(lines[endIdx+1:], "\n")

			// Clean up extra leading/trailing newlines from body
			bodyContent = strings.TrimPrefix(bodyContent, "\n")
			bodyContent = strings.TrimSuffix(bodyContent, "\n")

			properties := make(map[string]any)
			if err := yaml.Unmarshal([]byte(yamlContent), &properties); err != nil {
				return nil, err
			}

			return &Note{
				FilePath:   filePath,
				Properties: properties,
				Content:    bodyContent,
				}, nil
		}
	}

	// No frontmatter found
	bodyContent := strings.TrimPrefix(markdownContent, "\n")
	bodyContent = strings.TrimSuffix(bodyContent, "\n")

	return &Note{
		FilePath:   filePath,
		Properties: make(map[string]any),
		Content:    bodyContent,
	}, nil
}

// LoadBaseConfig parses a .base configuration file (YAML or JSON) content.
func LoadBaseConfig(configContent []byte) (*BaseConfig, error) {
	var config BaseConfig
	if err := yaml.Unmarshal(configContent, &config); err != nil {
		return nil, err
	}
	return &config, nil
}
