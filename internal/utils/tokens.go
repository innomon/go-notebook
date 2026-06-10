package utils

import "strings"

// TokenCount estimates the number of tokens in the input string.
// It uses a word-count fallback estimation: words * 1.3.
func TokenCount(input string) int {
	if input == "" {
		return 0
	}
	words := strings.Fields(input)
	return int(float64(len(words)) * 1.3)
}
