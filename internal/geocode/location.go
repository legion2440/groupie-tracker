package geocode

import (
	"strings"
	"unicode"

	"groupie-tracker/internal/catalog"
)

type LocationSpec struct {
	Raw            string
	Key            string
	Display        string
	Query          string
	City           string
	Country        string
	CountryAliases []string
}

var displayLocationAbbreviations = map[string]string{
	"usa":   "USA",
	"u.s.a": "U.S.A",
	"uk":    "UK",
	"uae":   "UAE",
}

var nominatimCountryNames = map[string]string{
	"netherlands antilles": "Curacao",
	"u.s.a":                "United States",
	"uae":                  "United Arab Emirates",
	"uk":                   "United Kingdom",
	"usa":                  "United States",
}

var countryAliases = map[string][]string{
	"usa": {
		"usa",
		"u.s.a",
		"us",
		"united states",
		"united states of america",
	},
	"u.s.a": {
		"usa",
		"u.s.a",
		"us",
		"united states",
		"united states of america",
	},
	"uk": {
		"uk",
		"gb",
		"great britain",
		"united kingdom",
	},
	"uae": {
		"uae",
		"united arab emirates",
	},
	"netherlands antilles": {
		"netherlands antilles",
		"curacao",
		"curaçao",
	},
}

func ParseLocation(raw string) LocationSpec {
	parts := splitLocationParts(raw)
	displayParts := make([]string, 0, len(parts))
	queryParts := make([]string, 0, len(parts))
	keyParts := make([]string, 0, len(parts))

	for i, part := range parts {
		display := formatLocationPart(part)
		if display == "" {
			continue
		}
		displayParts = append(displayParts, display)
		keyParts = append(keyParts, locationKeyPart(part))

		queryPart := display
		if i == len(parts)-1 {
			if countryName, ok := nominatimCountryNames[strings.ToLower(part)]; ok {
				queryPart = countryName
			}
		}
		queryParts = append(queryParts, queryPart)
	}

	spec := LocationSpec{
		Raw:     raw,
		Key:     strings.Join(keyParts, "-"),
		Display: strings.Join(displayParts, ", "),
		Query:   strings.Join(queryParts, ", "),
	}
	if len(displayParts) > 0 {
		spec.City = displayParts[0]
	}
	if len(displayParts) > 0 {
		spec.Country = displayParts[len(displayParts)-1]
		spec.CountryAliases = aliasesForCountry(spec.Country)
	}
	return spec
}

func NormalizeLocationKey(raw string) string {
	return ParseLocation(raw).Key
}

func DisplayLocation(raw string) string {
	return ParseLocation(raw).Display
}

func NominatimSearchQuery(raw string) string {
	return ParseLocation(raw).Query
}

func splitLocationParts(raw string) []string {
	raw = strings.ReplaceAll(raw, ",", "-")
	chunks := strings.Split(raw, "-")
	parts := make([]string, 0, len(chunks))
	for _, chunk := range chunks {
		part := strings.Join(strings.Fields(strings.ReplaceAll(chunk, "_", " ")), " ")
		if part == "" {
			continue
		}
		parts = append(parts, part)
	}
	return parts
}

func locationKeyPart(part string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.ReplaceAll(part, " ", "_")), "_"))
}

func formatLocationPart(part string) string {
	words := strings.Fields(strings.ReplaceAll(part, "_", " "))
	for i, word := range words {
		words[i] = formatLocationWord(word)
	}
	return strings.Join(words, " ")
}

func formatLocationWord(word string) string {
	lower := strings.ToLower(word)
	if abbreviation, ok := displayLocationAbbreviations[lower]; ok {
		return abbreviation
	}
	runes := []rune(lower)
	if len(runes) == 0 {
		return ""
	}
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

func aliasesForCountry(country string) []string {
	normalized := normalizeMatchText(country)
	aliases := []string{country, normalized}
	if known, ok := countryAliases[strings.ReplaceAll(normalized, " ", ".")]; ok {
		aliases = append(aliases, known...)
	}
	if known, ok := countryAliases[normalized]; ok {
		aliases = append(aliases, known...)
	}
	if queryName, ok := nominatimCountryNames[normalized]; ok {
		aliases = append(aliases, queryName)
	}
	return uniqueNormalizedAliases(aliases)
}

func uniqueNormalizedAliases(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		normalized := normalizeMatchText(value)
		if normalized == "" {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, value)
	}
	return result
}

func normalizeMatchText(value string) string {
	return foldLatin(catalog.NormalizeSearchText(value))
}

func foldLatin(value string) string {
	replacer := strings.NewReplacer(
		"á", "a", "à", "a", "â", "a", "ä", "a", "ã", "a", "å", "a", "ā", "a", "ă", "a", "ą", "a",
		"æ", "ae",
		"ç", "c", "ć", "c", "č", "c",
		"ď", "d",
		"é", "e", "è", "e", "ê", "e", "ë", "e", "ē", "e", "ė", "e", "ę", "e",
		"í", "i", "ì", "i", "î", "i", "ï", "i", "ī", "i",
		"ł", "l",
		"ñ", "n", "ń", "n",
		"ó", "o", "ò", "o", "ô", "o", "ö", "o", "õ", "o", "ø", "o", "ō", "o",
		"œ", "oe",
		"ř", "r",
		"ś", "s", "š", "s", "ß", "ss",
		"ť", "t",
		"ú", "u", "ù", "u", "û", "u", "ü", "u", "ū", "u",
		"ý", "y", "ÿ", "y",
		"ž", "z", "ź", "z", "ż", "z",
	)
	return replacer.Replace(value)
}
