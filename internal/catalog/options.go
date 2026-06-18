package catalog

import (
	"sort"
	"strings"
)

var nonLocalityRegionsByCountry = map[string]map[string]struct{}{
	"usa": {
		"alabama":        {},
		"arizona":        {},
		"california":     {},
		"colorado":       {},
		"florida":        {},
		"georgia":        {},
		"illinois":       {},
		"maine":          {},
		"massachusetts":  {},
		"michigan":       {},
		"minnesota":      {},
		"missouri":       {},
		"nevada":         {},
		"new hampshire":  {},
		"north carolina": {},
		"oklahoma":       {},
		"oregon":         {},
		"pennsylvania":   {},
		"south carolina": {},
		"texas":          {},
		"utah":           {},
		"washington":     {},
	},
	"australia": {
		"new south wales": {},
		"queensland":      {},
		"victoria":        {},
	},
}

type YearRange struct {
	Min       int
	Max       int
	Available bool
}

type LocationOption struct {
	Value string
	Label string
}

type FilterOptions struct {
	CreationYears   YearRange
	FirstAlbumYears YearRange
	MemberCounts    []int
	Locations       []LocationOption
}

func BuildFilterOptions(catalog Catalog) FilterOptions {
	var options FilterOptions

	memberCounts := make(map[int]struct{})
	locationCandidates := make(map[string]LocationOption)
	locationSuffixes := make(map[string]struct{})

	for _, artist := range catalog.Artists {
		addCreationYear(&options.CreationYears, artist.CreationYear)
		if !artist.FirstAlbum.IsZero() {
			addYear(&options.FirstAlbumYears, artist.FirstAlbum.Year())
		}

		if count := len(artist.Members); count > 0 {
			memberCounts[count] = struct{}{}
		}

		for _, location := range artist.Locations {
			for _, suffix := range location.Hierarchy[1:] {
				if suffix != "" {
					locationSuffixes[suffix] = struct{}{}
				}
			}

			option, ok := fullLocationOption(location)
			if !ok {
				continue
			}
			key := normalizeSelectedLocation(option.Value)
			if key == "" {
				continue
			}
			existing, exists := locationCandidates[key]
			if !exists || option.Value < existing.Value {
				locationCandidates[key] = option
			}
		}
	}

	options.MemberCounts = sortedMemberCounts(memberCounts)
	options.Locations = sortedLocationOptions(removeSuffixLocationOptions(locationCandidates, locationSuffixes))
	return options
}

func addCreationYear(yearRange *YearRange, year int) {
	if year <= 0 {
		return
	}
	addYear(yearRange, year)
}

func addYear(yearRange *YearRange, year int) {
	if !yearRange.Available {
		yearRange.Min = year
		yearRange.Max = year
		yearRange.Available = true
		return
	}
	if year < yearRange.Min {
		yearRange.Min = year
	}
	if year > yearRange.Max {
		yearRange.Max = year
	}
}

func sortedMemberCounts(counts map[int]struct{}) []int {
	values := make([]int, 0, len(counts))
	for count := range counts {
		values = append(values, count)
	}
	sort.Ints(values)
	return values
}

func fullLocationOption(location Location) (LocationOption, bool) {
	label := strings.TrimSpace(location.Display)
	components := splitDisplayLocation(label)
	if label == "" || len(components) < 2 || isKnownRegionOnlyLocation(components) {
		return LocationOption{}, false
	}
	return LocationOption{
		Value: label,
		Label: label,
	}, true
}

func isKnownRegionOnlyLocation(components []string) bool {
	if len(components) != 2 {
		return false
	}
	region := normalizeText(components[0])
	country := normalizeText(components[1])
	regions, ok := nonLocalityRegionsByCountry[country]
	if !ok {
		return false
	}
	_, found := regions[region]
	return found
}

func removeSuffixLocationOptions(candidates map[string]LocationOption, suffixes map[string]struct{}) map[string]LocationOption {
	options := make(map[string]LocationOption, len(candidates))
	for key, option := range candidates {
		if _, generatedSuffix := suffixes[key]; generatedSuffix {
			continue
		}
		options[key] = option
	}
	return options
}

func sortedLocationOptions(optionsByKey map[string]LocationOption) []LocationOption {
	options := make([]LocationOption, 0, len(optionsByKey))
	for _, option := range optionsByKey {
		options = append(options, option)
	}
	sort.Slice(options, func(i, j int) bool {
		left := normalizeText(options[i].Label)
		right := normalizeText(options[j].Label)
		if left == right {
			return options[i].Value < options[j].Value
		}
		return left < right
	})
	return options
}

func splitDisplayLocation(display string) []string {
	parts := strings.Split(display, ",")
	components := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		components = append(components, part)
	}
	return components
}
