package catalog

import (
	"reflect"
	"testing"
	"time"
)

func TestBuildFilterOptionsYearRanges(t *testing.T) {
	catalog := Catalog{
		Artists: []ArtistEntry{
			optionsTestArtist("Early", nil, 1965, date(1963, time.January, 1), nil),
			optionsTestArtist("Late", nil, 2015, date(2018, time.December, 31), nil),
			optionsTestArtist("Middle", nil, 1980, date(1974, time.June, 2), nil),
		},
	}

	got := BuildFilterOptions(catalog)

	assertYearRange(t, got.CreationYears, YearRange{Min: 1965, Max: 2015, Available: true})
	assertYearRange(t, got.FirstAlbumYears, YearRange{Min: 1963, Max: 2018, Available: true})
}

func TestBuildFilterOptionsMissingYearsAndEmptyCatalog(t *testing.T) {
	tests := []struct {
		name    string
		catalog Catalog
	}{
		{
			name: "missing values",
			catalog: Catalog{
				Artists: []ArtistEntry{
					optionsTestArtist("Missing", nil, 0, time.Time{}, nil),
					optionsTestArtist("Negative", nil, -1, time.Time{}, nil),
				},
			},
		},
		{name: "empty catalog", catalog: Catalog{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildFilterOptions(tt.catalog)
			assertYearRange(t, got.CreationYears, YearRange{})
			assertYearRange(t, got.FirstAlbumYears, YearRange{})
			if len(got.MemberCounts) != 0 {
				t.Fatalf("expected no member counts, got %v", got.MemberCounts)
			}
			if len(got.Locations) != 0 {
				t.Fatalf("expected no locations, got %#v", got.Locations)
			}
		})
	}
}

func TestBuildFilterOptionsMemberCountsDeduplicateAndSort(t *testing.T) {
	catalog := Catalog{
		Artists: []ArtistEntry{
			optionsTestArtist("Solo", []string{"A"}, 0, time.Time{}, nil),
			optionsTestArtist("Duo", []string{"A", "B"}, 0, time.Time{}, nil),
			optionsTestArtist("Duo Again", []string{"C", "D"}, 0, time.Time{}, nil),
			optionsTestArtist("Empty", nil, 0, time.Time{}, nil),
			optionsTestArtist("Trio", []string{"A", "B", "C"}, 0, time.Time{}, nil),
		},
	}

	got := BuildFilterOptions(catalog)
	assertInts(t, got.MemberCounts, []int{1, 2, 3})
}

func TestBuildFilterOptionsFullLocationLeavesOnly(t *testing.T) {
	catalog := Catalog{
		Artists: []ArtistEntry{
			optionsTestArtist("A", nil, 0, time.Time{}, []string{"seattle-washington-usa"}),
			optionsTestArtist("B", nil, 0, time.Time{}, []string{"seoul-south_korea"}),
			optionsTestArtist("C", nil, 0, time.Time{}, []string{"busan-south_korea"}),
			optionsTestArtist("D", nil, 0, time.Time{}, []string{"bratislava-slovakia"}),
			optionsTestArtist("E", nil, 0, time.Time{}, []string{"sochaux-france"}),
			optionsTestArtist("Duplicate", nil, 0, time.Time{}, []string{"seoul-south_korea"}),
			optionsTestArtist("Region Suffix", nil, 0, time.Time{}, []string{"washington-usa"}),
			optionsTestArtist("Region Only", nil, 0, time.Time{}, []string{"california-usa", "new_south_wales-australia"}),
			optionsTestArtist("Country Only", nil, 0, time.Time{}, []string{"usa", "south_korea"}),
		},
	}

	got := BuildFilterOptions(catalog)
	want := []LocationOption{
		{Value: "Bratislava, Slovakia", Label: "Bratislava, Slovakia"},
		{Value: "Busan, South Korea", Label: "Busan, South Korea"},
		{Value: "Seattle, Washington, USA", Label: "Seattle, Washington, USA"},
		{Value: "Seoul, South Korea", Label: "Seoul, South Korea"},
		{Value: "Sochaux, France", Label: "Sochaux, France"},
	}
	if !reflect.DeepEqual(got.Locations, want) {
		t.Fatalf("location options mismatch:\ngot  %#v\nwant %#v", got.Locations, want)
	}
	for _, forbidden := range []LocationOption{
		{Value: "South Korea", Label: "South Korea"},
		{Value: "Slovakia", Label: "Slovakia"},
		{Value: "Washington, USA", Label: "Washington, USA"},
		{Value: "California, USA", Label: "California, USA"},
		{Value: "New South Wales, Australia", Label: "New South Wales, Australia"},
		{Value: "USA", Label: "USA"},
	} {
		if containsLocationOption(got.Locations, forbidden) {
			t.Fatalf("hierarchy suffix option should not be exposed: %#v in %#v", forbidden, got.Locations)
		}
	}
}

func TestBuildFilterOptionsDoesNotMutateCatalogAndReturnsIndependentSlices(t *testing.T) {
	catalog := Catalog{
		Artists: []ArtistEntry{
			optionsTestArtist("A", []string{"A", "B"}, 1990, date(1995, time.January, 10), []string{"seattle-washington-usa"}),
			optionsTestArtist("B", []string{"A"}, 2000, date(2001, time.February, 3), []string{"london-uk"}),
		},
	}
	original := cloneTestCatalog(catalog)

	got := BuildFilterOptions(catalog)
	if len(got.MemberCounts) == 0 || len(got.Locations) == 0 {
		t.Fatalf("expected populated options, got %#v", got)
	}
	got.MemberCounts[0] = 99
	got.Locations[0].Value = "Changed"
	got.Locations[0].Label = "Changed"

	if !reflect.DeepEqual(catalog, original) {
		t.Fatalf("catalog mutated:\ngot  %#v\nwant %#v", catalog, original)
	}

	again := BuildFilterOptions(catalog)
	assertInts(t, again.MemberCounts, []int{1, 2})
	if again.Locations[0].Value == "Changed" || again.Locations[0].Label == "Changed" {
		t.Fatalf("returned location slice reused mutated values: %#v", again.Locations)
	}
}

func optionsTestArtist(name string, members []string, creationYear int, firstAlbum time.Time, rawLocations []string) ArtistEntry {
	locations := make([]Location, len(rawLocations))
	for i, rawLocation := range rawLocations {
		locations[i] = parseLocation(rawLocation)
	}
	return ArtistEntry{
		Name:         name,
		Members:      append([]string(nil), members...),
		CreationYear: creationYear,
		FirstAlbum:   firstAlbum,
		Locations:    locations,
	}
}

func assertYearRange(t *testing.T, got YearRange, want YearRange) {
	t.Helper()
	if got != want {
		t.Fatalf("year range mismatch: got %#v want %#v", got, want)
	}
}

func assertInts(t *testing.T, got []int, want []int) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ints mismatch: got %v want %v", got, want)
	}
}

func containsLocationOption(options []LocationOption, want LocationOption) bool {
	for _, option := range options {
		if option == want {
			return true
		}
	}
	return false
}
