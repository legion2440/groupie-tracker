package catalog

import (
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestSuggestEmptyInput(t *testing.T) {
	catalog := suggestionTestCatalog()
	for _, text := range []string{"", "   ", "\t\n"} {
		t.Run(strconv.Quote(text), func(t *testing.T) {
			got := Suggest(catalog, text)
			if got == nil {
				t.Fatal("expected allocated empty slice, got nil")
			}
			if len(got) != 0 {
				t.Fatalf("expected no suggestions, got %#v", got)
			}
		})
	}
}

func TestSuggestEveryRequiredType(t *testing.T) {
	catalog := suggestionTestCatalog()
	tests := []struct {
		name string
		text string
		want Suggestion
	}{
		{
			name: "artist",
			text: "queen",
			want: Suggestion{Value: "Queen", Field: SearchArtist, Label: "Queen - artist/band"},
		},
		{
			name: "member",
			text: "freddie",
			want: Suggestion{Value: "Freddie Mercury", Field: SearchMember, Label: "Freddie Mercury - member"},
		},
		{
			name: "location",
			text: "washington",
			want: Suggestion{Value: "Seattle, Washington, USA", Field: SearchLocation, Label: "Seattle, Washington, USA - location"},
		},
		{
			name: "first album",
			text: "14-07-1973",
			want: Suggestion{Value: "14-07-1973", Field: SearchFirstAlbum, Label: "14-07-1973 - first album"},
		},
		{
			name: "creation date",
			text: "1970",
			want: Suggestion{Value: "1970", Field: SearchCreationDate, Label: "1970 - creation date"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Suggest(catalog, tt.text)
			if !containsSuggestion(got, tt.want) {
				t.Fatalf("expected suggestion %#v in %#v", tt.want, got)
			}
		})
	}
}

func TestSuggestCaseAndWhitespaceNormalization(t *testing.T) {
	catalog := suggestionTestCatalog()

	got := Suggest(catalog, "QUEEN")
	if !containsSuggestion(got, Suggestion{Value: "Queen", Field: SearchArtist, Label: "Queen - artist/band"}) {
		t.Fatalf("expected case-insensitive Queen match, got %#v", got)
	}

	got = Suggest(catalog, "  phil   collins ")
	if !containsSuggestion(got, Suggestion{Value: "Phil Collins", Field: SearchArtist, Label: "Phil Collins - artist/band"}) {
		t.Fatalf("expected whitespace-normalized Phil Collins match, got %#v", got)
	}
}

func TestSuggestSearchSeparatorEquivalence(t *testing.T) {
	catalog := suggestionTestCatalog()

	for _, query := range []string{"London, UK", "london uk", "london-uk", "london_uk", "london/uk"} {
		t.Run(query, func(t *testing.T) {
			got := Suggest(catalog, query)
			if !containsSuggestion(got, Suggestion{
				Value: "London, UK",
				Field: SearchLocation,
				Label: "London, UK - location",
			}) {
				t.Fatalf("expected London location suggestion, got %#v", got)
			}
		})
	}

	for _, query := range []string{"14-07-1973", "14.07.1973", "14/07/1973", "14_07_1973"} {
		t.Run(query, func(t *testing.T) {
			got := Suggest(catalog, query)
			if !containsSuggestion(got, Suggestion{
				Value: "14-07-1973",
				Field: SearchFirstAlbum,
				Label: "14-07-1973 - first album",
			}) {
				t.Fatalf("expected first-album suggestion, got %#v", got)
			}
		})
	}
}

func TestSuggestRanksExactPrefixAndSubstring(t *testing.T) {
	catalog := Catalog{
		Artists: []ArtistEntry{
			suggestionArtist("Alpha Phil", nil, "", "", 0, nil),
			suggestionArtist("Phil Collins", nil, "", "", 0, nil),
			suggestionArtist("Phil", nil, "", "", 0, nil),
		},
	}

	got := Suggest(catalog, "phil")
	assertSuggestions(t, got, []Suggestion{
		{Value: "Phil", Field: SearchArtist, Label: "Phil - artist/band"},
		{Value: "Phil Collins", Field: SearchArtist, Label: "Phil Collins - artist/band"},
		{Value: "Alpha Phil", Field: SearchArtist, Label: "Alpha Phil - artist/band"},
	})
}

func TestSuggestPrimaryPhrasePreventsTokenFallback(t *testing.T) {
	catalog := Catalog{
		Artists: []ArtistEntry{
			suggestionArtist("London Band", nil, "10-01-1995", "1990", 1990, nil),
			suggestionArtist("River Echo", nil, "11-01-1995", "1991", 1991, []string{"london-uk"}),
		},
	}

	got := Suggest(catalog, "london uk")
	assertSuggestions(t, got, []Suggestion{
		{Value: "London, UK", Field: SearchLocation, Label: "London, UK - location"},
	})
}

func TestSuggestTokenFallbackOrdering(t *testing.T) {
	catalog := Catalog{
		Artists: []ArtistEntry{
			suggestionArtist("London Band", nil, "10-01-1995", "1990", 1990, nil),
			suggestionArtist("River Echo", nil, "11-01-1995", "1991", 1991, []string{"london-england-hall"}),
		},
	}

	got := Suggest(catalog, "london england missing")
	if len(got) < 2 {
		t.Fatalf("expected at least two fallback suggestions, got %#v", got)
	}
	assertSuggestions(t, got[:2], []Suggestion{
		{Value: "London, England, Hall", Field: SearchLocation, Label: "London, England, Hall - location"},
		{Value: "London Band", Field: SearchArtist, Label: "London Band - artist/band"},
	})
}

func TestSuggestTypePriority(t *testing.T) {
	catalog := suggestionTestCatalog()

	got := Suggest(catalog, "phil collins")
	assertSuggestions(t, got[:2], []Suggestion{
		{Value: "Phil Collins", Field: SearchArtist, Label: "Phil Collins - artist/band"},
		{Value: "Phil Collins", Field: SearchMember, Label: "Phil Collins - member"},
	})
}

func TestSuggestAlphabeticalOrderForEqualRankAndType(t *testing.T) {
	catalog := Catalog{
		Artists: []ArtistEntry{
			suggestionArtist("Alice in Chains", nil, "", "", 0, nil),
			suggestionArtist("Alice Cooper", nil, "", "", 0, nil),
		},
	}

	got := Suggest(catalog, "alice")
	assertSuggestions(t, got, []Suggestion{
		{Value: "Alice Cooper", Field: SearchArtist, Label: "Alice Cooper - artist/band"},
		{Value: "Alice in Chains", Field: SearchArtist, Label: "Alice in Chains - artist/band"},
	})
}

func TestSuggestDeduplicatesSameTypeCandidates(t *testing.T) {
	catalog := suggestionTestCatalog()

	got := Suggest(catalog, "john doe")
	assertSuggestions(t, got, []Suggestion{
		{Value: "John Doe", Field: SearchMember, Label: "John Doe - member"},
	})

	got = Suggest(catalog, "london")
	assertSuggestionCount(t, got, SearchLocation, "London, UK", 1)

	got = Suggest(catalog, "1970")
	assertSuggestionCount(t, got, SearchCreationDate, "1970", 1)
}

func TestSuggestPreservesCrossTypeDuplicates(t *testing.T) {
	catalog := suggestionTestCatalog()

	got := Suggest(catalog, "phil collins")
	assertSuggestions(t, got[:2], []Suggestion{
		{Value: "Phil Collins", Field: SearchArtist, Label: "Phil Collins - artist/band"},
		{Value: "Phil Collins", Field: SearchMember, Label: "Phil Collins - member"},
	})
}

func TestSuggestUsesFullLocationDisplayOnly(t *testing.T) {
	catalog := suggestionTestCatalog()

	got := Suggest(catalog, "washington")
	if !containsSuggestion(got, Suggestion{
		Value: "Seattle, Washington, USA",
		Field: SearchLocation,
		Label: "Seattle, Washington, USA - location",
	}) {
		t.Fatalf("expected full Seattle location suggestion, got %#v", got)
	}
	for _, suggestion := range got {
		if suggestion.Field == SearchLocation && (suggestion.Value == "Washington, USA" || suggestion.Value == "USA") {
			t.Fatalf("unexpected hierarchy suffix suggestion: %#v", suggestion)
		}
	}
}

func TestSuggestLimitsToFirst10AfterSorting(t *testing.T) {
	catalog := Catalog{
		Artists: []ArtistEntry{
			suggestionArtist("Match 12", nil, "", "", 0, nil),
			suggestionArtist("Match 02", nil, "", "", 0, nil),
			suggestionArtist("Match 11", nil, "", "", 0, nil),
			suggestionArtist("Match 01", nil, "", "", 0, nil),
			suggestionArtist("Match 10", nil, "", "", 0, nil),
			suggestionArtist("Match 09", nil, "", "", 0, nil),
			suggestionArtist("Match 08", nil, "", "", 0, nil),
			suggestionArtist("Match 07", nil, "", "", 0, nil),
			suggestionArtist("Match 06", nil, "", "", 0, nil),
			suggestionArtist("Match 05", nil, "", "", 0, nil),
			suggestionArtist("Match 04", nil, "", "", 0, nil),
			suggestionArtist("Match 03", nil, "", "", 0, nil),
		},
	}

	got := Suggest(catalog, "match")
	assertSuggestions(t, got, []Suggestion{
		{Value: "Match 01", Field: SearchArtist, Label: "Match 01 - artist/band"},
		{Value: "Match 02", Field: SearchArtist, Label: "Match 02 - artist/band"},
		{Value: "Match 03", Field: SearchArtist, Label: "Match 03 - artist/band"},
		{Value: "Match 04", Field: SearchArtist, Label: "Match 04 - artist/band"},
		{Value: "Match 05", Field: SearchArtist, Label: "Match 05 - artist/band"},
		{Value: "Match 06", Field: SearchArtist, Label: "Match 06 - artist/band"},
		{Value: "Match 07", Field: SearchArtist, Label: "Match 07 - artist/band"},
		{Value: "Match 08", Field: SearchArtist, Label: "Match 08 - artist/band"},
		{Value: "Match 09", Field: SearchArtist, Label: "Match 09 - artist/band"},
		{Value: "Match 10", Field: SearchArtist, Label: "Match 10 - artist/band"},
	})
}

func TestSuggestNoMatches(t *testing.T) {
	got := Suggest(suggestionTestCatalog(), "not present")
	if got == nil {
		t.Fatal("expected allocated empty slice, got nil")
	}
	if len(got) != 0 {
		t.Fatalf("expected no suggestions, got %#v", got)
	}
}

func TestSuggestDeterminism(t *testing.T) {
	catalog := suggestionTestCatalog()
	first := Suggest(catalog, "l")
	for i := 0; i < 5; i++ {
		got := Suggest(catalog, "l")
		if !reflect.DeepEqual(got, first) {
			t.Fatalf("non-deterministic suggestions on iteration %d:\ngot  %#v\nwant %#v", i, got, first)
		}
	}
}

func TestSuggestDoesNotMutateCatalog(t *testing.T) {
	catalog := suggestionTestCatalog()
	original := cloneTestCatalog(catalog)

	_ = Suggest(catalog, "phil")

	if !reflect.DeepEqual(catalog, original) {
		t.Fatalf("catalog mutated:\ngot  %#v\nwant %#v", catalog, original)
	}
}

func TestSuggestDeduplicatesDisplayVariantsByNormalizedValue(t *testing.T) {
	catalog := Catalog{
		Artists: []ArtistEntry{
			suggestionArtist("First", []string{"JOHN   DOE"}, "", "", 0, nil),
			suggestionArtist("Second", []string{"John Doe"}, "", "", 0, nil),
		},
	}

	got := Suggest(catalog, "john doe")
	assertSuggestions(t, got, []Suggestion{
		{Value: "JOHN   DOE", Field: SearchMember, Label: "JOHN   DOE - member"},
	})
}

func TestSuggestDeduplicatesSameTypeSeparatorVariants(t *testing.T) {
	catalog := Catalog{
		Artists: []ArtistEntry{
			suggestionArtist("First", []string{"AC/DC"}, "", "", 0, nil),
			suggestionArtist("Second", []string{"AC DC"}, "", "", 0, nil),
		},
	}

	got := Suggest(catalog, "ac dc")
	assertSuggestions(t, got, []Suggestion{
		{Value: "AC/DC", Field: SearchMember, Label: "AC/DC - member"},
	})
}

func TestSuggestLabelsUseASCIISeparator(t *testing.T) {
	got := Suggest(suggestionTestCatalog(), "london")
	if len(got) == 0 {
		t.Fatal("expected suggestions")
	}
	for _, suggestion := range got {
		if strings.ContainsAny(suggestion.Label, "—–") {
			t.Fatalf("suggestion label contains long dash: %#v", suggestion)
		}
		if !strings.Contains(suggestion.Label, " - ") {
			t.Fatalf("suggestion label missing ASCII separator: %#v", suggestion)
		}
	}
}

func TestSuggestFuzzyBasicTypes(t *testing.T) {
	catalog := suggestionTestCatalog()

	tests := []struct {
		name string
		text string
		want Suggestion
	}{
		{
			name: "artist",
			text: "queeen",
			want: Suggestion{Value: "Queen", Field: SearchArtist, Label: "Queen - artist/band"},
		},
		{
			name: "member",
			text: "freddi mercuri",
			want: Suggestion{Value: "Freddie Mercury", Field: SearchMember, Label: "Freddie Mercury - member"},
		},
		{
			name: "location",
			text: "londn",
			want: Suggestion{Value: "London, UK", Field: SearchLocation, Label: "London, UK - location"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Suggest(catalog, tt.text)
			if !containsSuggestion(got, tt.want) {
				t.Fatalf("expected suggestion %#v in %#v", tt.want, got)
			}
		})
	}
}

func TestSuggestFuzzySingleTokenAndContiguousWindow(t *testing.T) {
	catalog := Catalog{
		Artists: []ArtistEntry{
			suggestionArtist("London Echo", nil, "", "", 0, nil),
			suggestionArtist("Phil Collins Band", nil, "", "", 0, nil),
		},
	}

	got := Suggest(catalog, "londn")
	assertSuggestions(t, got, []Suggestion{
		{Value: "London Echo", Field: SearchArtist, Label: "London Echo - artist/band"},
	})

	got = Suggest(catalog, "phil colins")
	assertSuggestions(t, got, []Suggestion{
		{Value: "Phil Collins Band", Field: SearchArtist, Label: "Phil Collins Band - artist/band"},
	})
}

func TestSuggestFuzzyRanking(t *testing.T) {
	t.Run("lower distance first", func(t *testing.T) {
		catalog := Catalog{
			Artists: []ArtistEntry{
				suggestionArtist("Phil Colon", nil, "", "", 0, nil),
				suggestionArtist("Phil Collins", nil, "", "", 0, nil),
			},
		}

		got := Suggest(catalog, "phil colins")
		assertSuggestions(t, got, []Suggestion{
			{Value: "Phil Collins", Field: SearchArtist, Label: "Phil Collins - artist/band"},
			{Value: "Phil Colon", Field: SearchArtist, Label: "Phil Colon - artist/band"},
		})
	})

	t.Run("type priority breaks equal distance", func(t *testing.T) {
		catalog := Catalog{
			Artists: []ArtistEntry{
				suggestionArtist("Queen", nil, "", "", 0, nil),
				suggestionArtist("Carrier", []string{"Queen"}, "", "", 0, nil),
			},
		}

		got := Suggest(catalog, "qeen")
		assertSuggestions(t, got, []Suggestion{
			{Value: "Queen", Field: SearchArtist, Label: "Queen - artist/band"},
			{Value: "Queen", Field: SearchMember, Label: "Queen - member"},
		})
	})

	t.Run("alphabetical tie", func(t *testing.T) {
		catalog := Catalog{
			Artists: []ArtistEntry{
				suggestionArtist("Qeen B", nil, "", "", 0, nil),
				suggestionArtist("Qeen A", nil, "", "", 0, nil),
			},
		}

		got := Suggest(catalog, "qeenx")
		assertSuggestions(t, got, []Suggestion{
			{Value: "Qeen A", Field: SearchArtist, Label: "Qeen A - artist/band"},
			{Value: "Qeen B", Field: SearchArtist, Label: "Qeen B - artist/band"},
		})
	})
}

func TestSuggestFuzzyDeduplicationAndCrossTypePreservation(t *testing.T) {
	t.Run("same type deduplication", func(t *testing.T) {
		catalog := Catalog{
			Artists: []ArtistEntry{
				suggestionArtist("First", []string{"AC/DC"}, "", "", 0, nil),
				suggestionArtist("Second", []string{"AC DC"}, "", "", 0, nil),
			},
		}

		got := Suggest(catalog, "acx dcx")
		assertSuggestions(t, got, []Suggestion{
			{Value: "AC/DC", Field: SearchMember, Label: "AC/DC - member"},
		})
	})

	t.Run("same value different types", func(t *testing.T) {
		catalog := Catalog{
			Artists: []ArtistEntry{
				suggestionArtist("Qeen", nil, "", "", 0, nil),
				suggestionArtist("Carrier", []string{"Qeen"}, "", "", 0, nil),
			},
		}

		got := Suggest(catalog, "qeenx")
		assertSuggestions(t, got, []Suggestion{
			{Value: "Qeen", Field: SearchArtist, Label: "Qeen - artist/band"},
			{Value: "Qeen", Field: SearchMember, Label: "Qeen - member"},
		})
	})
}

func TestSuggestFuzzyLimitAppliedAfterSorting(t *testing.T) {
	catalog := Catalog{
		Artists: []ArtistEntry{
			suggestionArtist("Match 12", nil, "", "", 0, nil),
			suggestionArtist("Match 02", nil, "", "", 0, nil),
			suggestionArtist("Match 11", nil, "", "", 0, nil),
			suggestionArtist("Match 01", nil, "", "", 0, nil),
			suggestionArtist("Match 10", nil, "", "", 0, nil),
			suggestionArtist("Match 09", nil, "", "", 0, nil),
			suggestionArtist("Match 08", nil, "", "", 0, nil),
			suggestionArtist("Match 07", nil, "", "", 0, nil),
			suggestionArtist("Match 06", nil, "", "", 0, nil),
			suggestionArtist("Match 05", nil, "", "", 0, nil),
			suggestionArtist("Match 04", nil, "", "", 0, nil),
			suggestionArtist("Match 03", nil, "", "", 0, nil),
		},
	}

	got := Suggest(catalog, "matxh")
	assertSuggestions(t, got, []Suggestion{
		{Value: "Match 01", Field: SearchArtist, Label: "Match 01 - artist/band"},
		{Value: "Match 02", Field: SearchArtist, Label: "Match 02 - artist/band"},
		{Value: "Match 03", Field: SearchArtist, Label: "Match 03 - artist/band"},
		{Value: "Match 04", Field: SearchArtist, Label: "Match 04 - artist/band"},
		{Value: "Match 05", Field: SearchArtist, Label: "Match 05 - artist/band"},
		{Value: "Match 06", Field: SearchArtist, Label: "Match 06 - artist/band"},
		{Value: "Match 07", Field: SearchArtist, Label: "Match 07 - artist/band"},
		{Value: "Match 08", Field: SearchArtist, Label: "Match 08 - artist/band"},
		{Value: "Match 09", Field: SearchArtist, Label: "Match 09 - artist/band"},
		{Value: "Match 10", Field: SearchArtist, Label: "Match 10 - artist/band"},
	})
}

func TestSuggestFuzzyOnlyAfterPrimaryAndTokenFallbackFail(t *testing.T) {
	catalog := Catalog{
		Artists: []ArtistEntry{
			suggestionArtist("Queen", nil, "", "", 0, nil),
			suggestionArtist("Qeeen", nil, "", "", 0, nil),
			suggestionArtist("London Band", nil, "", "", 0, nil),
			suggestionArtist("Londn Artist", nil, "", "", 0, nil),
		},
	}

	got := Suggest(catalog, "queen")
	assertSuggestions(t, got, []Suggestion{
		{Value: "Queen", Field: SearchArtist, Label: "Queen - artist/band"},
	})

	got = Suggest(catalog, "london missing")
	assertSuggestions(t, got, []Suggestion{
		{Value: "London Band", Field: SearchArtist, Label: "London Band - artist/band"},
	})
}

func TestSuggestNumericQueriesUsePhraseOnly(t *testing.T) {
	catalog := Catalog{
		Artists: []ArtistEntry{
			suggestionArtist("Numeric", nil, "05-08-1967", "1970", 1970, nil),
		},
	}

	tests := []struct {
		name  string
		query string
		want  []Suggestion
	}{
		{
			name:  "exact date hyphen separator",
			query: "05-08-1967",
			want:  []Suggestion{{Value: "05-08-1967", Field: SearchFirstAlbum, Label: "05-08-1967 - first album"}},
		},
		{
			name:  "exact date dot separator",
			query: "05.08.1967",
			want:  []Suggestion{{Value: "05-08-1967", Field: SearchFirstAlbum, Label: "05-08-1967 - first album"}},
		},
		{
			name:  "exact date slash separator",
			query: "05/08/1967",
			want:  []Suggestion{{Value: "05-08-1967", Field: SearchFirstAlbum, Label: "05-08-1967 - first album"}},
		},
		{
			name:  "exact creation year",
			query: "1970",
			want:  []Suggestion{{Value: "1970", Field: SearchCreationDate, Label: "1970 - creation date"}},
		},
		{
			name:  "wrong date year dot separator",
			query: "05.08.1968",
			want:  []Suggestion{},
		},
		{
			name:  "wrong date year slash separator",
			query: "05/08/1968",
			want:  []Suggestion{},
		},
		{
			name:  "nearby creation year",
			query: "1971",
			want:  []Suggestion{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Suggest(catalog, tt.query)
			assertSuggestions(t, got, tt.want)
		})
	}
}

func TestSuggestFuzzyDisabledForNumericAndShortQueries(t *testing.T) {
	catalog := suggestionTestCatalog()
	for _, query := range []string{"qen", "1971", "05.08.1968", "06.09.1968"} {
		t.Run(query, func(t *testing.T) {
			got := Suggest(catalog, query)
			if len(got) != 0 {
				t.Fatalf("expected no fallback suggestions for %q, got %#v", query, got)
			}
		})
	}

	got := Suggest(catalog, "uk")
	assertSuggestions(t, got, []Suggestion{
		{Value: "London, UK", Field: SearchLocation, Label: "London, UK - location"},
	})
}

func TestSuggestFuzzyDeterminism(t *testing.T) {
	catalog := suggestionTestCatalog()
	first := Suggest(catalog, "queeen")
	for i := 0; i < 5; i++ {
		got := Suggest(catalog, "queeen")
		if !reflect.DeepEqual(got, first) {
			t.Fatalf("non-deterministic fuzzy suggestions on iteration %d:\ngot  %#v\nwant %#v", i, got, first)
		}
	}
}

func suggestionTestCatalog() Catalog {
	return Catalog{
		Artists: []ArtistEntry{
			suggestionArtist(
				"Queen",
				[]string{"Freddie Mercury", "Brian May", "John Doe"},
				"14-07-1973",
				"1970",
				1970,
				[]string{"london-uk", "seattle-washington-usa"},
			),
			suggestionArtist(
				"Phil Collins",
				[]string{"Phil Collins", "John Doe"},
				"05-02-1981",
				"1981",
				1981,
				[]string{"london-uk"},
			),
			suggestionArtist(
				"Alice Cooper",
				[]string{"Alice Person"},
				"10-03-1970",
				"1970",
				1970,
				[]string{"los_angeles-usa"},
			),
		},
	}
}

func suggestionArtist(name string, members []string, firstAlbumRaw string, creationYearText string, creationYear int, rawLocations []string) ArtistEntry {
	locations := make([]Location, len(rawLocations))
	for i, rawLocation := range rawLocations {
		locations[i] = parseLocation(rawLocation)
	}
	return ArtistEntry{
		Name:                 name,
		Members:              append([]string(nil), members...),
		CreationYear:         creationYear,
		FirstAlbumRaw:        firstAlbumRaw,
		Locations:            locations,
		NormalizedName:       normalizeText(name),
		NormalizedMembers:    normalizeMembers(members),
		NormalizedFirstAlbum: normalizeText(firstAlbumRaw),
		CreationYearText:     creationYearText,
	}
}

func assertSuggestions(t *testing.T, got []Suggestion, want []Suggestion) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("suggestions mismatch:\ngot  %#v\nwant %#v", got, want)
	}
}

func containsSuggestion(suggestions []Suggestion, want Suggestion) bool {
	for _, suggestion := range suggestions {
		if suggestion == want {
			return true
		}
	}
	return false
}

func assertSuggestionCount(t *testing.T, suggestions []Suggestion, field SearchField, value string, want int) {
	t.Helper()
	count := 0
	for _, suggestion := range suggestions {
		if suggestion.Field == field && suggestion.Value == value {
			count++
		}
	}
	if count != want {
		t.Fatalf("suggestion count for %s/%q mismatch: got %d want %d in %#v", field, value, count, want, suggestions)
	}
}
