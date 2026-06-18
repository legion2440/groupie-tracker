package server

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	"groupie-tracker/internal/catalog"
	"groupie-tracker/internal/model"
)

func testCatalog() catalog.Catalog {
	cat, err := catalog.Build(
		[]model.Artist{
			{
				ID:           1,
				Name:         "Echo Lane",
				Image:        "echo.jpeg",
				Members:      []string{"Ava Stone", "Chris Pike"},
				CreationDate: 1990,
				FirstAlbum:   "10-01-1995",
			},
			{
				ID:           2,
				Name:         "Ava Stone",
				Image:        "ava.jpeg",
				Members:      []string{"Echo Lane"},
				CreationDate: 1980,
				FirstAlbum:   "05-05-1984",
			},
			{
				ID:           3,
				Name:         "Northern Lights",
				Image:        "northern.jpeg",
				Members:      []string{"Solo Star", "Mila Noon", "Zoe Artist"},
				CreationDate: 2001,
				FirstAlbum:   "20-07-2003",
			},
		},
		[]model.Relation{
			{ID: 1, DatesLocations: map[string][]string{"seattle-washington-usa": {"10-01-1995"}}},
			{ID: 2, DatesLocations: map[string][]string{"los_angeles-usa": {"05-05-1984"}}},
			{ID: 3, DatesLocations: map[string][]string{"london-uk": {"20-07-2003"}}},
		},
	)
	if err != nil {
		panic(err)
	}
	return cat
}

func testArtist(id int, name string, image string, members []string, creationYear int, firstAlbum string, locations []catalog.Location) catalog.ArtistEntry {
	return catalog.ArtistEntry{
		ID:                   id,
		Slug:                 catalog.ArtistSlug(name),
		Name:                 name,
		Image:                image,
		Members:              append([]string(nil), members...),
		CreationYear:         creationYear,
		FirstAlbumRaw:        firstAlbum,
		FirstAlbum:           mustServerTestDate(firstAlbum),
		Locations:            cloneTestLocations(locations),
		NormalizedName:       normalizeForTest(name),
		NormalizedMembers:    normalizeMembersForTest(members),
		NormalizedFirstAlbum: normalizeForTest(firstAlbum),
		CreationYearText:     intStringForTest(creationYear),
	}
}

func testLocation(raw string, display string, normalized string, hierarchy []string) catalog.Location {
	return catalog.Location{
		Raw:        raw,
		Display:    display,
		Normalized: normalized,
		Hierarchy:  append([]string(nil), hierarchy...),
	}
}

func testDependenciesWithCatalog(cat catalog.Catalog, calls *int) dependencies {
	return dependencies{
		updateNow: func() error {
			return nil
		},
		loadCatalog: func() (catalog.Catalog, error) {
			if calls != nil {
				(*calls)++
			}
			return cloneTestCatalogForServer(cat), nil
		},
		loadRelations: func() ([]model.Relation, error) {
			return nil, nil
		},
		previewLookup: func(context.Context, string) (deezerPreviewResult, error) {
			return deezerPreviewResult{}, nil
		},
	}
}

func cloneTestCatalogForServer(cat catalog.Catalog) catalog.Catalog {
	cloned := catalog.Catalog{
		Artists:        make([]catalog.ArtistEntry, len(cat.Artists)),
		ArtistByID:     cloneArtistByIDForServer(cat.ArtistByID),
		ArtistIDBySlug: cloneStringIntMapForServer(cat.ArtistIDBySlug),
		ArtistSlugByID: cloneIntStringMapForServer(cat.ArtistSlugByID),
	}
	for i, artist := range cat.Artists {
		cloned.Artists[i] = cloneTestArtistEntry(artist)
	}
	return cloned
}

func cloneTestArtistEntry(artist catalog.ArtistEntry) catalog.ArtistEntry {
	cloned := artist
	cloned.Members = append([]string(nil), artist.Members...)
	cloned.NormalizedMembers = append([]string(nil), artist.NormalizedMembers...)
	cloned.Locations = cloneTestLocations(artist.Locations)
	return cloned
}

func cloneArtistByIDForServer(src map[int]catalog.ArtistEntry) map[int]catalog.ArtistEntry {
	if src == nil {
		return nil
	}
	cloned := make(map[int]catalog.ArtistEntry, len(src))
	for key, value := range src {
		cloned[key] = cloneTestArtistEntry(value)
	}
	return cloned
}

func cloneStringIntMapForServer(src map[string]int) map[string]int {
	if src == nil {
		return nil
	}
	cloned := make(map[string]int, len(src))
	for key, value := range src {
		cloned[key] = value
	}
	return cloned
}

func cloneIntStringMapForServer(src map[int]string) map[int]string {
	if src == nil {
		return nil
	}
	cloned := make(map[int]string, len(src))
	for key, value := range src {
		cloned[key] = value
	}
	return cloned
}

func cloneTestLocations(locations []catalog.Location) []catalog.Location {
	cloned := make([]catalog.Location, len(locations))
	for i, location := range locations {
		cloned[i] = location
		cloned[i].Hierarchy = append([]string(nil), location.Hierarchy...)
	}
	return cloned
}

func assertRenderedArtists(t *testing.T, body string, want []string) {
	t.Helper()
	all := []string{"Echo Lane", "Ava Stone", "Northern Lights"}
	last := -1
	for _, name := range want {
		marker := `aria-label="` + name + `"`
		idx := strings.Index(body, marker)
		if idx < 0 {
			t.Fatalf("expected rendered artist %q in body", name)
		}
		if idx <= last {
			t.Fatalf("artist %q rendered out of order", name)
		}
		last = idx
	}

	for _, name := range all {
		if containsStringForTest(want, name) {
			continue
		}
		marker := `aria-label="` + name + `"`
		if strings.Contains(body, marker) {
			t.Fatalf("did not expect rendered artist %q in body", name)
		}
	}
}

func containsStringForTest(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func normalizeForTest(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(value), " "))
}

func normalizeMembersForTest(members []string) []string {
	normalized := make([]string, len(members))
	for i, member := range members {
		normalized[i] = normalizeForTest(member)
	}
	return normalized
}

func intStringForTest(value int) string {
	return strconv.Itoa(value)
}

func mustServerTestDate(raw string) time.Time {
	parsed, err := time.ParseInLocation("02-01-2006", raw, time.UTC)
	if err != nil {
		panic(err)
	}
	return parsed
}
