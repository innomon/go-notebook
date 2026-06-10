package extractor

// ChunkText splits text into overlapping chunks of a given character size (rune-based for UTF-8 safety)
func ChunkText(text string, chunkSize, overlap int) []string {
	if chunkSize <= 0 {
		chunkSize = 1500
	}
	if overlap < 0 || overlap >= chunkSize {
		overlap = 225
	}

	runes := []rune(text)
	n := len(runes)
	if n == 0 {
		return []string{}
	}

	if n <= chunkSize {
		return []string{string(runes)}
	}

	var chunks []string
	step := chunkSize - overlap

	for start := 0; start < n; start += step {
		end := start + chunkSize
		if end > n {
			end = n
		}

		chunks = append(chunks, string(runes[start:end]))

		// If we reached the end of the text, stop
		if end == n {
			break
		}
	}

	return chunks
}
