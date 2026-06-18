package catalog

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

func boundedLevenshtein(left string, right string, maxDistance int) (int, bool) {
	if maxDistance < 0 {
		return 0, false
	}
	if left == right {
		return 0, true
	}

	leftRunes := []rune(left)
	rightRunes := []rune(right)
	if absInt(len(leftRunes)-len(rightRunes)) > maxDistance {
		return 0, false
	}
	if len(leftRunes) == 0 {
		return len(rightRunes), len(rightRunes) <= maxDistance
	}
	if len(rightRunes) == 0 {
		return len(leftRunes), len(leftRunes) <= maxDistance
	}

	previous := make([]int, len(rightRunes)+1)
	current := make([]int, len(rightRunes)+1)
	for j := range previous {
		previous[j] = j
	}

	for i, leftRune := range leftRunes {
		current[0] = i + 1
		rowMin := current[0]
		for j, rightRune := range rightRunes {
			substitutionCost := 1
			if leftRune == rightRune {
				substitutionCost = 0
			}

			deletion := previous[j+1] + 1
			insertion := current[j] + 1
			substitution := previous[j] + substitutionCost
			current[j+1] = minInt(deletion, minInt(insertion, substitution))
			rowMin = minInt(rowMin, current[j+1])
		}
		if rowMin > maxDistance {
			return 0, false
		}
		previous, current = current, previous
	}

	distance := previous[len(rightRunes)]
	return distance, distance <= maxDistance
}

func fuzzyDistanceLimit(query string) int {
	if !allowsSearchFallback(query) {
		return 0
	}
	switch length := utf8.RuneCountInString(query); {
	case length <= 3:
		return 0
	case length <= 6:
		return 1
	case length <= 12:
		return 2
	default:
		return 3
	}
}

func SearchTextFuzzyDistance(query string, candidate string) (int, bool) {
	normalizedQuery := NormalizeSearchText(query)
	normalizedCandidate := NormalizeSearchText(candidate)
	if normalizedQuery == "" || normalizedCandidate == "" {
		return 0, false
	}
	return boundedLevenshtein(normalizedQuery, normalizedCandidate, fuzzyDistanceLimit(normalizedQuery))
}

func allowsSearchFallback(query string) bool {
	return containsLetter(query)
}

func containsLetter(value string) bool {
	for _, r := range value {
		if unicode.IsLetter(r) {
			return true
		}
	}
	return false
}

func fuzzyMatchDistance(query string, queryTokens []string, candidate string, maxDistance int) (int, bool) {
	if maxDistance == 0 || !allowsSearchFallback(query) || len(queryTokens) == 0 || candidate == "" {
		return 0, false
	}

	candidateTokens := strings.Fields(candidate)
	if len(candidateTokens) == 0 {
		return 0, false
	}

	bestDistance := maxDistance + 1
	if len(queryTokens) == 1 {
		for _, token := range candidateTokens {
			if distance, ok := boundedLevenshtein(queryTokens[0], token, maxDistance); ok && distance < bestDistance {
				bestDistance = distance
			}
		}
	} else {
		windowSize := len(queryTokens)
		if len(candidateTokens) >= windowSize {
			for i := 0; i <= len(candidateTokens)-windowSize; i++ {
				window := strings.Join(candidateTokens[i:i+windowSize], " ")
				if distance, ok := boundedLevenshtein(query, window, maxDistance); ok && distance < bestDistance {
					bestDistance = distance
				}
			}
		}
		if distance, ok := boundedLevenshtein(query, candidate, maxDistance); ok && distance < bestDistance {
			bestDistance = distance
		}
	}

	if bestDistance <= maxDistance {
		return bestDistance, true
	}
	return 0, false
}

func minInt(left int, right int) int {
	if left < right {
		return left
	}
	return right
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
