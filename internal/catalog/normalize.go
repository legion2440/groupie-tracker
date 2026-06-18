package catalog

import (
	"strings"
	"unicode/utf8"
)

func normalizeText(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

func NormalizeSearchText(value string) string {
	mapped := strings.Map(func(r rune) rune {
		switch r {
		case ',', '.', '-', '_', '/', '\\', '–', '—':
			return ' '
		default:
			return r
		}
	}, strings.TrimSpace(value))
	return strings.ToLower(strings.Join(strings.Fields(mapped), " "))
}

func ArtistSlug(name string) string {
	return strings.Trim(strings.ReplaceAll(NormalizeSearchText(name), " ", "-"), "-")
}

func normalizeSearchText(value string) string {
	return NormalizeSearchText(value)
}

func searchQueryTokens(normalized string) []string {
	seen := make(map[string]struct{})
	tokens := make([]string, 0)
	for _, token := range strings.Fields(normalized) {
		if utf8.RuneCountInString(token) < 2 {
			continue
		}
		if _, exists := seen[token]; exists {
			continue
		}
		seen[token] = struct{}{}
		tokens = append(tokens, token)
	}
	return tokens
}

func searchTokenSet(normalized string) map[string]struct{} {
	tokens := make(map[string]struct{})
	for _, token := range strings.Fields(normalized) {
		tokens[token] = struct{}{}
	}
	return tokens
}

func countMatchedSearchTokens(candidate string, queryTokens []string) int {
	if candidate == "" || len(queryTokens) == 0 {
		return 0
	}
	candidateTokens := searchTokenSet(candidate)
	matches := 0
	for _, token := range queryTokens {
		if _, ok := candidateTokens[token]; ok {
			matches++
		}
	}
	return matches
}
