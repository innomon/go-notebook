package graphrag

import (
	"regexp"
	"strings"
)

// TextBlock represents a parsed segment of text
type TextBlock struct {
	Index int
	Text  string
}

// TextChunk represents an overlapping slice of text with offset positions
type TextChunk struct {
	ChunkID    int    `json:"chunk_id"`
	Text       string `json:"text"`
	CharStart  int    `json:"char_start"`
	CharEnd    int    `json:"char_end"`
	Source     string `json:"source"`
	TokenCount int    `json:"token_count"`
}

var noisePatterns = []*regexp.Regexp{
	regexp.MustCompile(`<[^>]+>`),
	regexp.MustCompile(`\[[^\]]*\]`),
	regexp.MustCompile(`={2,}\s*.*?\s*={2,}`),
}

// ParseText splits raw text into blocks by paragraph boundary
func ParseText(content string) []TextBlock {
	rawBlocks := regexp.MustCompile(`\n\s*\n`).Split(content, -1)
	var blocks []TextBlock
	index := 0
	for _, raw := range rawBlocks {
		// Clean internal newlines
		lines := strings.Split(raw, "\n")
		var cleanLines []string
		for _, line := range lines {
			t := strings.TrimSpace(line)
			if t != "" {
				cleanLines = append(cleanLines, t)
			}
		}
		text := strings.Join(cleanLines, " ")
		if text != "" {
			blocks = append(blocks, TextBlock{Index: index, Text: text})
			index++
		}
	}
	return blocks
}

// CleanText filters out patterns like HTML tags and excessive spacing
func CleanText(blocks []TextBlock) []TextBlock {
	var cleaned []TextBlock
	spaceRegex := regexp.MustCompile(`\s+`)
	for _, block := range blocks {
		text := block.Text
		for _, pattern := range noisePatterns {
			text = pattern.ReplaceAllString(text, " ")
		}
		text = spaceRegex.ReplaceAllString(text, " ")
		text = strings.TrimSpace(text)
		if len(text) >= 3 {
			cleaned = append(cleaned, TextBlock{Index: block.Index, Text: text})
		}
	}
	return cleaned
}

// EstimateTokenCount estimates the tokens in a string (approx. 1 token = 0.75 words / 4 chars)
func EstimateTokenCount(text string) int {
	words := strings.Fields(text)
	return int(float64(len(words)) * 1.3)
}

// ChunkBlocks chunks cleaned blocks into overlapping segments
func ChunkBlocks(blocks []TextBlock, chunkSize, overlap int, sourceName string) []TextChunk {
	if len(blocks) == 0 {
		return nil
	}

	var allTexts []string
	for _, b := range blocks {
		allTexts = append(allTexts, b.Text)
	}
	allText := strings.Join(allTexts, " ")
	words := strings.Fields(allText)
	if len(words) == 0 {
		return nil
	}

	// Track character positions for each word in allText
	charPositions := make([]int, len(words))
	currentPos := 0
	for idx, word := range words {
		idxInText := strings.Index(allText[currentPos:], word)
		if idxInText != -1 {
			charPositions[idx] = currentPos + idxInText
			currentPos = charPositions[idx] + len(word)
		} else {
			charPositions[idx] = currentPos
		}
	}

	var chunks []TextChunk
	chunkID := 0
	i := 0

	for i < len(words) {
		var window []string
		var tc int
		j := i

		// Fill window until chunkSize is exceeded
		for j < len(words) {
			window = append(window, words[j])
			tc = EstimateTokenCount(strings.Join(window, " "))
			if tc >= chunkSize {
				j++
				break
			}
			j++
		}

		if len(window) == 0 {
			break
		}

		chunkText := strings.Join(window, " ")
		charStart := charPositions[i]
		lastWordIdx := j - 1
		if lastWordIdx >= len(words) {
			lastWordIdx = len(words) - 1
		}
		charEnd := charPositions[lastWordIdx] + len(window[len(window)-1])

		chunks = append(chunks, TextChunk{
			ChunkID:    chunkID,
			Text:       chunkText,
			CharStart:  charStart,
			CharEnd:    charEnd,
			Source:     sourceName,
			TokenCount: tc,
		})
		chunkID++

		if j >= len(words) {
			break
		}

		// Overlap calculation
		var overlapWords []string
		ot := 0
		for k := j - 1; k >= i; k-- {
			overlapWords = append([]string{words[k]}, overlapWords...)
			ot = EstimateTokenCount(strings.Join(overlapWords, " "))
			if ot >= overlap {
				break
			}
		}

		nextI := j - len(overlapWords)
		if nextI <= i {
			nextI = i + 1
		}
		i = nextI
	}

	return chunks
}
