package worker

import "strings"

// joinTokens is used by non-streaming consumers and intentionally uses a
// pre-sized Builder to avoid the quadratic copying of repeated concatenation.
func joinTokens(tokens []string) string {
	var size int
	for _, token := range tokens {
		size += len(token)
	}
	var result strings.Builder
	result.Grow(size)
	for _, token := range tokens {
		result.WriteString(token)
	}
	return result.String()
}
