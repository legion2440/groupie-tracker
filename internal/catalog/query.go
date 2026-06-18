package catalog

import (
	"fmt"
	"strings"
	"time"
)

type SearchField string

const (
	SearchAny          SearchField = ""
	SearchArtist       SearchField = "artist"
	SearchMember       SearchField = "member"
	SearchLocation     SearchField = "location"
	SearchFirstAlbum   SearchField = "first_album"
	SearchCreationDate SearchField = "creation_date"
)

type Criteria struct {
	SearchText  string
	SearchField SearchField

	CreationFrom *int
	CreationTo   *int

	FirstAlbumFrom *time.Time
	FirstAlbumTo   *time.Time

	MemberCounts   []int
	MinMemberCount *int
	Locations      []string
}

type searchMode int

const (
	searchModePhrase searchMode = iota
	searchModeToken
	searchModeFuzzy
)

type preparedCriteria struct {
	searchText         string
	searchField        SearchField
	searchTokens       []string
	fuzzyTokens        []string
	fuzzyDistanceLimit int
	allowsFallback     bool
	searchMode         searchMode
	creationFrom       *int
	creationTo         *int
	firstAlbumFrom     *time.Time
	firstAlbumTo       *time.Time
	memberCounts       map[int]struct{}
	minMemberCount     *int
	locations          map[string]struct{}
}

func Filter(catalog Catalog, criteria Criteria) ([]ArtistEntry, error) {
	prepared, err := prepareCriteria(criteria)
	if err != nil {
		return nil, err
	}
	if prepared.searchText != "" && prepared.allowsFallback && !catalogHasPrimarySearchMatch(catalog, prepared) {
		if len(prepared.searchTokens) > 0 && catalogHasTokenSearchMatch(catalog, prepared) {
			prepared.searchMode = searchModeToken
		} else if prepared.fuzzyDistanceLimit > 0 && catalogHasFuzzySearchMatch(catalog, prepared) {
			prepared.searchMode = searchModeFuzzy
		}
	}

	matches := make([]ArtistEntry, 0, len(catalog.Artists))
	for _, artist := range catalog.Artists {
		if !matchesSearch(artist, prepared) {
			continue
		}
		if !matchesCreationRange(artist, prepared) {
			continue
		}
		if !matchesFirstAlbumRange(artist, prepared) {
			continue
		}
		if !matchesMemberCount(artist, prepared) {
			continue
		}
		if !matchesLocation(artist, prepared) {
			continue
		}
		matches = append(matches, cloneArtistEntry(artist))
	}
	return matches, nil
}

func prepareCriteria(criteria Criteria) (preparedCriteria, error) {
	if !isKnownSearchField(criteria.SearchField) {
		return preparedCriteria{}, fmt.Errorf("unknown search field %q", criteria.SearchField)
	}
	if criteria.CreationFrom != nil && criteria.CreationTo != nil && *criteria.CreationFrom > *criteria.CreationTo {
		return preparedCriteria{}, fmt.Errorf("invalid creation range: from %d after to %d", *criteria.CreationFrom, *criteria.CreationTo)
	}
	if criteria.FirstAlbumFrom != nil && criteria.FirstAlbumTo != nil && criteria.FirstAlbumFrom.After(*criteria.FirstAlbumTo) {
		return preparedCriteria{}, fmt.Errorf("invalid first album range: from %s after to %s",
			criteria.FirstAlbumFrom.Format(firstAlbumLayout),
			criteria.FirstAlbumTo.Format(firstAlbumLayout),
		)
	}

	memberCounts := make(map[int]struct{}, len(criteria.MemberCounts))
	for _, count := range criteria.MemberCounts {
		if count < 1 {
			return preparedCriteria{}, fmt.Errorf("invalid member count %d: must be at least 1", count)
		}
		memberCounts[count] = struct{}{}
	}
	if criteria.MinMemberCount != nil && *criteria.MinMemberCount < 1 {
		return preparedCriteria{}, fmt.Errorf("invalid minimum member count %d: must be at least 1", *criteria.MinMemberCount)
	}

	locations := make(map[string]struct{}, len(criteria.Locations))
	for _, location := range criteria.Locations {
		normalized := normalizeSelectedLocation(location)
		if normalized == "" {
			continue
		}
		locations[normalized] = struct{}{}
	}

	searchText := normalizeSearchText(criteria.SearchText)

	return preparedCriteria{
		searchText:         searchText,
		searchField:        criteria.SearchField,
		searchTokens:       searchQueryTokens(searchText),
		fuzzyTokens:        strings.Fields(searchText),
		fuzzyDistanceLimit: fuzzyDistanceLimit(searchText),
		allowsFallback:     allowsSearchFallback(searchText),
		searchMode:         searchModePhrase,
		creationFrom:       criteria.CreationFrom,
		creationTo:         criteria.CreationTo,
		firstAlbumFrom:     criteria.FirstAlbumFrom,
		firstAlbumTo:       criteria.FirstAlbumTo,
		memberCounts:       memberCounts,
		minMemberCount:     criteria.MinMemberCount,
		locations:          locations,
	}, nil
}

func isKnownSearchField(field SearchField) bool {
	switch field {
	case SearchAny, SearchArtist, SearchMember, SearchLocation, SearchFirstAlbum, SearchCreationDate:
		return true
	default:
		return false
	}
}

func matchesSearch(artist ArtistEntry, criteria preparedCriteria) bool {
	if criteria.searchText == "" {
		return true
	}
	switch criteria.searchMode {
	case searchModeToken:
		return matchesSearchTokens(artist, criteria)
	case searchModeFuzzy:
		return matchesSearchFuzzy(artist, criteria)
	default:
		return matchesSearchPhrase(artist, criteria)
	}
}

func catalogHasPrimarySearchMatch(catalog Catalog, criteria preparedCriteria) bool {
	for _, artist := range catalog.Artists {
		if matchesSearchPhrase(artist, criteria) {
			return true
		}
	}
	return false
}

func catalogHasTokenSearchMatch(catalog Catalog, criteria preparedCriteria) bool {
	for _, artist := range catalog.Artists {
		if matchesSearchTokens(artist, criteria) {
			return true
		}
	}
	return false
}

func catalogHasFuzzySearchMatch(catalog Catalog, criteria preparedCriteria) bool {
	for _, artist := range catalog.Artists {
		if matchesSearchFuzzy(artist, criteria) {
			return true
		}
	}
	return false
}

func matchesSearchPhrase(artist ArtistEntry, criteria preparedCriteria) bool {
	for _, key := range searchKeysForField(artist, criteria.searchField) {
		if strings.Contains(key, criteria.searchText) {
			return true
		}
	}
	return false
}

func matchesSearchTokens(artist ArtistEntry, criteria preparedCriteria) bool {
	for _, key := range searchKeysForField(artist, criteria.searchField) {
		if countMatchedSearchTokens(key, criteria.searchTokens) > 0 {
			return true
		}
	}
	return false
}

func matchesSearchFuzzy(artist ArtistEntry, criteria preparedCriteria) bool {
	for _, key := range searchKeysForField(artist, criteria.searchField) {
		if _, ok := fuzzyMatchDistance(criteria.searchText, criteria.fuzzyTokens, key, criteria.fuzzyDistanceLimit); ok {
			return true
		}
	}
	return false
}

func searchKeysForField(artist ArtistEntry, field SearchField) []string {
	switch field {
	case SearchAny:
		keys := make([]string, 0, 4+len(artist.Members)+len(artist.Locations))
		keys = append(keys, searchKeysForField(artist, SearchArtist)...)
		keys = append(keys, searchKeysForField(artist, SearchMember)...)
		keys = append(keys, searchKeysForField(artist, SearchLocation)...)
		keys = append(keys, searchKeysForField(artist, SearchFirstAlbum)...)
		keys = append(keys, searchKeysForField(artist, SearchCreationDate)...)
		return keys
	case SearchArtist:
		return []string{normalizeSearchText(artist.Name)}
	case SearchMember:
		keys := make([]string, 0, len(artist.Members))
		for _, member := range artist.Members {
			keys = append(keys, normalizeSearchText(member))
		}
		return keys
	case SearchLocation:
		keys := make([]string, 0, len(artist.Locations))
		for _, location := range artist.Locations {
			keys = append(keys, normalizeSearchText(location.Display))
		}
		return keys
	case SearchFirstAlbum:
		return []string{normalizeSearchText(artist.FirstAlbumRaw)}
	case SearchCreationDate:
		return []string{normalizeSearchText(artist.CreationYearText)}
	default:
		return nil
	}
}

func matchesCreationRange(artist ArtistEntry, criteria preparedCriteria) bool {
	if criteria.creationFrom != nil && artist.CreationYear < *criteria.creationFrom {
		return false
	}
	if criteria.creationTo != nil && artist.CreationYear > *criteria.creationTo {
		return false
	}
	return true
}

func matchesFirstAlbumRange(artist ArtistEntry, criteria preparedCriteria) bool {
	if criteria.firstAlbumFrom != nil && artist.FirstAlbum.Before(*criteria.firstAlbumFrom) {
		return false
	}
	if criteria.firstAlbumTo != nil && artist.FirstAlbum.After(*criteria.firstAlbumTo) {
		return false
	}
	return true
}

func matchesMemberCount(artist ArtistEntry, criteria preparedCriteria) bool {
	if len(criteria.memberCounts) == 0 && criteria.minMemberCount == nil {
		return true
	}
	count := len(artist.Members)
	if _, ok := criteria.memberCounts[count]; ok {
		return true
	}
	return criteria.minMemberCount != nil && count >= *criteria.minMemberCount
}

func matchesLocation(artist ArtistEntry, criteria preparedCriteria) bool {
	if len(criteria.locations) == 0 {
		return true
	}
	for _, location := range artist.Locations {
		for _, hierarchy := range location.Hierarchy {
			if _, ok := criteria.locations[hierarchy]; ok {
				return true
			}
		}
	}
	return false
}

func normalizeSelectedLocation(location string) string {
	if strings.Contains(location, ",") {
		components := strings.Split(location, ",")
		normalized := make([]string, 0, len(components))
		for _, component := range components {
			component = normalizeText(component)
			if component == "" {
				continue
			}
			normalized = append(normalized, component)
		}
		return strings.Join(normalized, ", ")
	}
	if strings.Contains(location, "-") {
		return parseLocation(location).Normalized
	}
	return normalizeText(location)
}

func cloneArtistEntry(artist ArtistEntry) ArtistEntry {
	cloned := artist
	cloned.Members = append([]string(nil), artist.Members...)
	cloned.NormalizedMembers = append([]string(nil), artist.NormalizedMembers...)
	cloned.Locations = cloneLocations(artist.Locations)
	return cloned
}

func cloneLocations(locations []Location) []Location {
	cloned := make([]Location, len(locations))
	for i, location := range locations {
		cloned[i] = location
		cloned[i].Hierarchy = append([]string(nil), location.Hierarchy...)
	}
	return cloned
}
