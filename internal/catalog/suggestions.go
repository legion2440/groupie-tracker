package catalog

import (
	"sort"
	"strings"
)

const suggestionLimit = 10

type Suggestion struct {
	Value string
	Label string
	Field SearchField
}

type suggestionCandidate struct {
	Suggestion
	normalized    string
	matchRank     int
	typeRank      int
	matchedTokens int
	fuzzyDistance int
}

type suggestionKey struct {
	field      SearchField
	normalized string
}

func Suggest(catalog Catalog, text string) []Suggestion {
	query := normalizeSearchText(text)
	if query == "" {
		return []Suggestion{}
	}

	candidates := dedupeSuggestionCandidates(collectSuggestionCandidates(catalog))
	matches := make([]suggestionCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		rank, ok := suggestionMatchRank(candidate.normalized, query)
		if !ok {
			continue
		}
		candidate.matchRank = rank
		candidate.typeRank = suggestionTypePriority(candidate.Field)
		matches = append(matches, candidate)
	}

	if allowsSearchFallback(query) {
		if len(matches) == 0 {
			matches = tokenFallbackSuggestionMatches(candidates, searchQueryTokens(query))
		}
		if len(matches) == 0 {
			matches = fuzzySuggestionMatches(candidates, query, strings.Fields(query), fuzzyDistanceLimit(query))
		}
	}

	matches = sortSuggestionCandidates(matches)

	suggestions := make([]Suggestion, len(matches))
	for i, match := range matches {
		suggestions[i] = match.Suggestion
	}
	return suggestions
}

func collectSuggestionCandidates(catalog Catalog) []suggestionCandidate {
	var candidates []suggestionCandidate
	for _, artist := range catalog.Artists {
		candidates = append(candidates, newSuggestionCandidate(artist.Name, SearchArtist))

		for _, member := range artist.Members {
			candidates = append(candidates, newSuggestionCandidate(member, SearchMember))
		}

		for _, location := range artist.Locations {
			candidates = append(candidates, newSuggestionCandidate(location.Display, SearchLocation))
		}

		candidates = append(candidates, newSuggestionCandidate(artist.FirstAlbumRaw, SearchFirstAlbum))
		candidates = append(candidates, newSuggestionCandidate(artist.CreationYearText, SearchCreationDate))
	}
	return candidates
}

func dedupeSuggestionCandidates(candidates []suggestionCandidate) []suggestionCandidate {
	seen := make(map[suggestionKey]struct{}, len(candidates))
	deduped := make([]suggestionCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.normalized == "" || candidate.Field == SearchAny {
			continue
		}
		key := suggestionKey{
			field:      candidate.Field,
			normalized: candidate.normalized,
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		deduped = append(deduped, candidate)
	}
	return deduped
}

func newSuggestionCandidate(value string, field SearchField) suggestionCandidate {
	normalized := normalizeSearchText(value)
	return suggestionCandidate{
		Suggestion: Suggestion{
			Value: value,
			Label: suggestionLabel(value, field),
			Field: field,
		},
		normalized: normalized,
	}
}

func tokenFallbackSuggestionMatches(candidates []suggestionCandidate, queryTokens []string) []suggestionCandidate {
	if len(queryTokens) == 0 {
		return []suggestionCandidate{}
	}

	matches := make([]suggestionCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		matchedTokens := countMatchedSearchTokens(candidate.normalized, queryTokens)
		if matchedTokens == 0 {
			continue
		}
		candidate.matchRank = 3
		candidate.matchedTokens = matchedTokens
		candidate.typeRank = suggestionTypePriority(candidate.Field)
		matches = append(matches, candidate)
	}
	return matches
}

func fuzzySuggestionMatches(candidates []suggestionCandidate, query string, queryTokens []string, maxDistance int) []suggestionCandidate {
	if maxDistance == 0 {
		return []suggestionCandidate{}
	}

	matches := make([]suggestionCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		distance, ok := fuzzyMatchDistance(query, queryTokens, candidate.normalized, maxDistance)
		if !ok {
			continue
		}
		candidate.matchRank = 4
		candidate.fuzzyDistance = distance
		candidate.typeRank = suggestionTypePriority(candidate.Field)
		matches = append(matches, candidate)
	}
	return matches
}

func sortSuggestionCandidates(matches []suggestionCandidate) []suggestionCandidate {
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].matchRank != matches[j].matchRank {
			return matches[i].matchRank < matches[j].matchRank
		}
		if matches[i].matchRank == 3 && matches[i].matchedTokens != matches[j].matchedTokens {
			return matches[i].matchedTokens > matches[j].matchedTokens
		}
		if matches[i].matchRank == 4 && matches[i].fuzzyDistance != matches[j].fuzzyDistance {
			return matches[i].fuzzyDistance < matches[j].fuzzyDistance
		}
		if matches[i].typeRank != matches[j].typeRank {
			return matches[i].typeRank < matches[j].typeRank
		}
		if matches[i].normalized != matches[j].normalized {
			return matches[i].normalized < matches[j].normalized
		}
		if matches[i].Value != matches[j].Value {
			return matches[i].Value < matches[j].Value
		}
		return matches[i].Field < matches[j].Field
	})

	if len(matches) > suggestionLimit {
		matches = matches[:suggestionLimit]
	}
	return matches
}

func suggestionMatchRank(candidate string, query string) (int, bool) {
	switch {
	case candidate == query:
		return 0, true
	case strings.HasPrefix(candidate, query):
		return 1, true
	case strings.Contains(candidate, query):
		return 2, true
	default:
		return 0, false
	}
}

func suggestionLabel(value string, field SearchField) string {
	return value + " - " + suggestionTypeLabel(field)
}

func suggestionTypeLabel(field SearchField) string {
	switch field {
	case SearchArtist:
		return "artist/band"
	case SearchMember:
		return "member"
	case SearchLocation:
		return "location"
	case SearchFirstAlbum:
		return "first album"
	case SearchCreationDate:
		return "creation date"
	default:
		return ""
	}
}

func suggestionTypePriority(field SearchField) int {
	switch field {
	case SearchArtist:
		return 0
	case SearchMember:
		return 1
	case SearchLocation:
		return 2
	case SearchFirstAlbum:
		return 3
	case SearchCreationDate:
		return 4
	default:
		return 5
	}
}
