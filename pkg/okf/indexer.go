package okf

import (
	"regexp"
	"strings"
)

// LinkRegex matches standard markdown link patterns: [Label](path/to/target).
// The first captured group matches the link target path.
var LinkRegex = regexp.MustCompile(`\[[^\]]+\]\(([^)]+)\)`)

// ExtractLinks sweeps the markdown body byte slice for valid internal relative link paths.
// It filters out external HTTP and HTTPS links, returning only local link targets.
func ExtractLinks(body []byte) []string {
	matches := LinkRegex.FindAllSubmatch(body, -1)
	links := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) > 1 {
			linkPath := string(match[1])
			if !strings.HasPrefix(linkPath, "http://") && !strings.HasPrefix(linkPath, "https://") {
				links = append(links, linkPath)
			}
		}
	}
	return links
}
