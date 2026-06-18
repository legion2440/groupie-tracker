package server

import (
	"fmt"
	"net/url"
	"strconv"
	"time"
	"unicode/utf8"

	"groupie-tracker/internal/catalog"
)

const (
	maxSearchQueryRunes = 200
	albumQueryLayout    = "2006-01-02"
)

func parseCriteria(values url.Values) (catalog.Criteria, error) {
	criteria := catalog.Criteria{
		SearchText: values.Get("q"),
	}

	if searchTextTooLong(criteria.SearchText) {
		return catalog.Criteria{}, fmt.Errorf("q exceeds %d runes", maxSearchQueryRunes)
	}

	searchField, err := parseSearchField(values.Get("search_type"))
	if err != nil {
		return catalog.Criteria{}, err
	}
	criteria.SearchField = searchField
	if catalog.NormalizeSearchText(criteria.SearchText) == "" {
		criteria.SearchField = catalog.SearchAny
	}

	if criteria.CreationFrom, err = parseOptionalInt(values, "creation_from"); err != nil {
		return catalog.Criteria{}, err
	}
	if criteria.CreationTo, err = parseOptionalInt(values, "creation_to"); err != nil {
		return catalog.Criteria{}, err
	}
	if criteria.CreationFrom != nil && criteria.CreationTo != nil && *criteria.CreationFrom > *criteria.CreationTo {
		return catalog.Criteria{}, fmt.Errorf("creation range is reversed")
	}

	if criteria.FirstAlbumFrom, err = parseOptionalDate(values, "album_from"); err != nil {
		return catalog.Criteria{}, err
	}
	if criteria.FirstAlbumTo, err = parseOptionalDate(values, "album_to"); err != nil {
		return catalog.Criteria{}, err
	}
	if criteria.FirstAlbumFrom != nil && criteria.FirstAlbumTo != nil && criteria.FirstAlbumFrom.After(*criteria.FirstAlbumTo) {
		return catalog.Criteria{}, fmt.Errorf("first album range is reversed")
	}

	criteria.MemberCounts, criteria.MinMemberCount, err = parseMemberCounts(values["members"])
	if err != nil {
		return catalog.Criteria{}, err
	}
	criteria.Locations = parseLocations(values["locations"])

	return criteria, nil
}

func searchTextTooLong(text string) bool {
	return utf8.RuneCountInString(text) > maxSearchQueryRunes
}

func parseSearchField(value string) (catalog.SearchField, error) {
	field := catalog.SearchField(value)
	switch field {
	case catalog.SearchAny, catalog.SearchArtist, catalog.SearchMember, catalog.SearchLocation, catalog.SearchFirstAlbum, catalog.SearchCreationDate:
		return field, nil
	default:
		return "", fmt.Errorf("unknown search_type %q", value)
	}
}

func parseOptionalInt(values url.Values, key string) (*int, error) {
	raw := values.Get(key)
	if raw == "" {
		return nil, nil
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", key, err)
	}
	return &parsed, nil
}

func parseOptionalDate(values url.Values, key string) (*time.Time, error) {
	raw := values.Get(key)
	if raw == "" {
		return nil, nil
	}
	parsed, err := time.Parse(albumQueryLayout, raw)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", key, err)
	}
	return &parsed, nil
}

func parseMemberCounts(values []string) ([]int, *int, error) {
	if len(values) == 0 {
		return nil, nil, nil
	}
	counts := make([]int, 0, len(values))
	var minCount *int
	for _, raw := range values {
		if raw == "" {
			continue
		}
		if raw == "8+" {
			min := 8
			minCount = &min
			continue
		}
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			return nil, nil, fmt.Errorf("parse members: %w", err)
		}
		if parsed < 1 {
			return nil, nil, fmt.Errorf("members must be positive: %d", parsed)
		}
		counts = append(counts, parsed)
	}
	return counts, minCount, nil
}

func parseLocations(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	locations := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		locations = append(locations, value)
	}
	return locations
}
