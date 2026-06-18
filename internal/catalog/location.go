package catalog

import (
	"sort"
	"strings"
	"unicode"
)

var knownLocationAbbreviations = map[string]string{
	"usa":   "USA",
	"uk":    "UK",
	"uae":   "UAE",
	"u.s.a": "U.S.A",
}

func parseLocation(raw string) Location {
	components := strings.Split(raw, "-")
	displayComponents := make([]string, 0, len(components))
	for _, component := range components {
		display := formatLocationComponent(component)
		if display == "" {
			continue
		}
		displayComponents = append(displayComponents, display)
	}

	display := strings.Join(displayComponents, ", ")
	hierarchy := make([]string, 0, len(displayComponents))
	for i := range displayComponents {
		hierarchy = append(hierarchy, normalizeText(strings.Join(displayComponents[i:], ", ")))
	}

	return Location{
		Raw:        raw,
		Display:    display,
		Normalized: normalizeText(display),
		Hierarchy:  hierarchy,
	}
}

func buildLocations(rawLocations []string) []Location {
	byNormalized := make(map[string]Location, len(rawLocations))
	for _, raw := range rawLocations {
		location := parseLocation(raw)
		if location.Normalized == "" {
			continue
		}
		existing, exists := byNormalized[location.Normalized]
		if !exists || location.Raw < existing.Raw {
			byNormalized[location.Normalized] = location
		}
	}

	locations := make([]Location, 0, len(byNormalized))
	for _, location := range byNormalized {
		locations = append(locations, location)
	}
	sort.Slice(locations, func(i, j int) bool {
		if locations[i].Normalized == locations[j].Normalized {
			return locations[i].Raw < locations[j].Raw
		}
		return locations[i].Normalized < locations[j].Normalized
	})
	return locations
}

func formatLocationComponent(component string) string {
	words := strings.Fields(strings.ReplaceAll(component, "_", " "))
	for i, word := range words {
		words[i] = formatLocationWord(word)
	}
	return strings.Join(words, " ")
}

func formatLocationWord(word string) string {
	lower := strings.ToLower(word)
	if abbreviation, ok := knownLocationAbbreviations[lower]; ok {
		return abbreviation
	}
	runes := []rune(lower)
	if len(runes) == 0 {
		return ""
	}
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}
