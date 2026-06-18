package catalog

import (
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestFilterEmptyCriteria(t *testing.T) {
	catalog := queryTestCatalog()

	got, err := Filter(catalog, Criteria{})
	if err != nil {
		t.Fatalf("Filter returned error: %v", err)
	}
	assertArtistIDs(t, got, []int{1, 2, 3})

	got[0].Name = "Changed"
	if catalog.Artists[0].Name != "Echo Lane" {
		t.Fatalf("catalog top-level artist mutated: %q", catalog.Artists[0].Name)
	}
}

func TestFilterBroadSearch(t *testing.T) {
	catalog := queryTestCatalog()
	tests := []struct {
		name string
		text string
		want []int
	}{
		{
			name: "artist name with case and whitespace normalization",
			text: "  NoRtHeRn   LiGhTs ",
			want: []int{3},
		},
		{
			name: "member",
			text: "solo",
			want: []int{3},
		},
		{
			name: "location",
			text: "washington",
			want: []int{1},
		},
		{
			name: "first album",
			text: "05-05",
			want: []int{2},
		},
		{
			name: "creation year",
			text: "2001",
			want: []int{3},
		},
		{
			name: "partial substring",
			text: "gel",
			want: []int{2},
		},
		{
			name: "no match",
			text: "not present",
			want: []int{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Filter(catalog, Criteria{SearchText: tt.text})
			if err != nil {
				t.Fatalf("Filter returned error: %v", err)
			}
			assertArtistIDs(t, got, tt.want)
		})
	}
}

func TestNormalizeSearchTextSeparatorEquivalence(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "comma", in: "London, UK", want: "london uk"},
		{name: "space", in: "london uk", want: "london uk"},
		{name: "hyphen", in: "london-uk", want: "london uk"},
		{name: "underscore", in: "london_uk", want: "london uk"},
		{name: "slash", in: "london/uk", want: "london uk"},
		{name: "backslash", in: `london\uk`, want: "london uk"},
		{name: "date hyphen", in: "05-08-1967", want: "05 08 1967"},
		{name: "date dot", in: "05.08.1967", want: "05 08 1967"},
		{name: "date slash", in: "05/08/1967", want: "05 08 1967"},
		{name: "date underscore", in: "05_08_1967", want: "05 08 1967"},
		{name: "case and repeated whitespace", in: "  LoNdOn   UK  ", want: "london uk"},
		{name: "mixed separators", in: "los_angeles-USA", want: "los angeles usa"},
		{name: "unicode lowered only", in: "  Zoë   ARTIST ", want: "zoë artist"},
		{name: "long dashes", in: "London–UK—Arena", want: "london uk arena"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeSearchText(tt.in); got != tt.want {
				t.Fatalf("normalizeSearchText(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestFilterSearchSeparatorEquivalence(t *testing.T) {
	catalog := Catalog{
		Artists: []ArtistEntry{
			queryTestArtist(1, "Separator Test", []string{"Case Member"}, 1990, "05-08-1967", []string{"london-uk"}),
		},
	}

	for _, query := range []string{"London, UK", "london uk", "london-uk", "london_uk", "london/uk"} {
		t.Run(query, func(t *testing.T) {
			got, err := Filter(catalog, Criteria{SearchText: query, SearchField: SearchLocation})
			if err != nil {
				t.Fatalf("Filter returned error: %v", err)
			}
			assertArtistIDs(t, got, []int{1})
			if got[0].Locations[0].Display != "London, UK" {
				t.Fatalf("location display changed: %q", got[0].Locations[0].Display)
			}
		})
	}

	for _, query := range []string{"05-08-1967", "05.08.1967", "05/08/1967", "05_08_1967"} {
		t.Run(query, func(t *testing.T) {
			got, err := Filter(catalog, Criteria{SearchText: query, SearchField: SearchFirstAlbum})
			if err != nil {
				t.Fatalf("Filter returned error: %v", err)
			}
			assertArtistIDs(t, got, []int{1})
			if got[0].FirstAlbumRaw != "05-08-1967" {
				t.Fatalf("first album display changed: %q", got[0].FirstAlbumRaw)
			}
		})
	}
}

func TestFilterPrimaryPhrasePreventsTokenFallback(t *testing.T) {
	catalog := Catalog{
		Artists: []ArtistEntry{
			queryTestArtist(1, "London Band", []string{"Ava Stone"}, 1990, "10-01-1995", nil),
			queryTestArtist(2, "River Echo", []string{"Chris Pike"}, 1991, "11-01-1995", []string{"london-uk"}),
		},
	}

	got, err := Filter(catalog, Criteria{SearchText: "london uk"})
	if err != nil {
		t.Fatalf("Filter returned error: %v", err)
	}
	assertArtistIDs(t, got, []int{2})
}

func TestFilterWholeTokenFallback(t *testing.T) {
	catalog := Catalog{
		Artists: []ArtistEntry{
			queryTestArtist(1, "River Echo", []string{"Chris Pike"}, 1991, "11-01-1995", []string{"london-uk"}),
			queryTestArtist(2, "The Band", []string{"Ava Stone"}, 1990, "10-01-1995", nil),
		},
	}

	got, err := Filter(catalog, Criteria{SearchText: "london england"})
	if err != nil {
		t.Fatalf("Filter returned error: %v", err)
	}
	assertArtistIDs(t, got, []int{1})

	got, err = Filter(catalog, Criteria{SearchText: "he missing"})
	if err != nil {
		t.Fatalf("Filter returned error: %v", err)
	}
	assertArtistIDs(t, got, []int{})
}

func TestFilterTypedTokenFallbackStaysInSelectedField(t *testing.T) {
	catalog := Catalog{
		Artists: []ArtistEntry{
			queryTestArtist(1, "London Band", []string{"Ava Stone"}, 1990, "10-01-1995", nil),
			queryTestArtist(2, "River Echo", []string{"London Member"}, 1991, "11-01-1995", []string{"london-uk"}),
		},
	}

	got, err := Filter(catalog, Criteria{SearchText: "london england", SearchField: SearchLocation})
	if err != nil {
		t.Fatalf("Filter returned error: %v", err)
	}
	assertArtistIDs(t, got, []int{2})
}

func TestFilterTokenFallbackActivationIgnoresOtherFilters(t *testing.T) {
	catalog := Catalog{
		Artists: []ArtistEntry{
			queryTestArtist(1, "London UK", []string{"Ava Stone"}, 1990, "10-01-1995", nil),
			queryTestArtist(2, "London Band", []string{"Chris Pike"}, 2001, "11-01-1995", nil),
		},
	}

	got, err := Filter(catalog, Criteria{
		SearchText:   "london uk",
		CreationFrom: intPtr(2000),
	})
	if err != nil {
		t.Fatalf("Filter returned error: %v", err)
	}
	assertArtistIDs(t, got, []int{})
}

func TestFilterFuzzyBasicTypos(t *testing.T) {
	catalog := Catalog{
		Artists: []ArtistEntry{
			queryTestArtist(1, "Queen", []string{"Freddie Mercury"}, 1970, "14-07-1973", []string{"london-uk"}),
			queryTestArtist(2, "Pink Floyd", []string{"Syd Barrett"}, 1965, "05-08-1967", []string{"london-uk"}),
			queryTestArtist(3, "Solo Artist", []string{"Phil Collins"}, 1981, "05-02-1981", []string{"paris-france"}),
		},
	}

	tests := []struct {
		query string
		want  []int
	}{
		{query: "queeen", want: []int{1}},
		{query: "qeen", want: []int{1}},
		{query: "pink floid", want: []int{2}},
		{query: "phil colins", want: []int{3}},
		{query: "londn", want: []int{1, 2}},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			got, err := Filter(catalog, Criteria{SearchText: tt.query})
			if err != nil {
				t.Fatalf("Filter returned error: %v", err)
			}
			assertArtistIDs(t, got, tt.want)
		})
	}
}

func TestFilterFuzzyOnlyAfterStrongerModesFail(t *testing.T) {
	catalog := Catalog{
		Artists: []ArtistEntry{
			queryTestArtist(1, "Queen", nil, 1970, "14-07-1973", nil),
			queryTestArtist(2, "Qeen", nil, 1971, "15-07-1973", nil),
			queryTestArtist(3, "London Band", nil, 1990, "10-01-1995", nil),
			queryTestArtist(4, "River Echo", nil, 1991, "11-01-1995", []string{"london-uk"}),
			queryTestArtist(5, "Londn Artist", nil, 1992, "12-01-1995", nil),
		},
	}

	got, err := Filter(catalog, Criteria{SearchText: "qeen"})
	if err != nil {
		t.Fatalf("Filter returned error: %v", err)
	}
	assertArtistIDs(t, got, []int{2})

	got, err = Filter(catalog, Criteria{SearchText: "london england"})
	if err != nil {
		t.Fatalf("Filter returned error: %v", err)
	}
	assertArtistIDs(t, got, []int{3, 4})
}

func TestFilterFuzzyThresholdBoundaries(t *testing.T) {
	catalog := Catalog{
		Artists: []ArtistEntry{
			queryTestArtist(1, "abcd", nil, 1990, "10-01-1995", nil),
			queryTestArtist(2, "abcdefg", nil, 1990, "10-01-1995", nil),
			queryTestArtist(3, "abcdefghijklmn", nil, 1990, "10-01-1995", nil),
		},
	}

	tests := []struct {
		name  string
		query string
		want  []int
	}{
		{name: "length 3 disabled", query: "abx", want: []int{}},
		{name: "length 4 accepts distance 1", query: "abce", want: []int{1}},
		{name: "length 4 rejects distance 2", query: "abef", want: []int{}},
		{name: "length 7 accepts distance 2", query: "abcdezz", want: []int{2}},
		{name: "length 7 rejects distance 3", query: "abczzzz", want: []int{}},
		{name: "length 13 accepts distance 3", query: "abcdefghijkxyz", want: []int{3}},
		{name: "length 13 rejects distance 4", query: "abcdefghijwxyz", want: []int{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Filter(catalog, Criteria{SearchText: tt.query, SearchField: SearchArtist})
			if err != nil {
				t.Fatalf("Filter returned error: %v", err)
			}
			assertArtistIDs(t, got, tt.want)
		})
	}
}

func TestFilterNumericQueriesUsePhraseOnly(t *testing.T) {
	catalog := Catalog{
		Artists: []ArtistEntry{
			queryTestArtist(1, "Numeric", nil, 1970, "05-08-1967", nil),
		},
	}

	tests := []struct {
		name     string
		criteria Criteria
		want     []int
	}{
		{
			name:     "exact date hyphen separator broad",
			criteria: Criteria{SearchText: "05-08-1967"},
			want:     []int{1},
		},
		{
			name:     "exact date dot separator broad",
			criteria: Criteria{SearchText: "05.08.1967"},
			want:     []int{1},
		},
		{
			name:     "exact date slash separator broad",
			criteria: Criteria{SearchText: "05/08/1967"},
			want:     []int{1},
		},
		{
			name:     "exact creation year broad",
			criteria: Criteria{SearchText: "1970"},
			want:     []int{1},
		},
		{
			name:     "wrong date year dot separator broad",
			criteria: Criteria{SearchText: "05.08.1968"},
			want:     []int{},
		},
		{
			name:     "wrong date year slash separator broad",
			criteria: Criteria{SearchText: "05/08/1968"},
			want:     []int{},
		},
		{
			name:     "nearby creation year broad",
			criteria: Criteria{SearchText: "1971"},
			want:     []int{},
		},
		{
			name:     "exact date typed first album",
			criteria: Criteria{SearchText: "05/08/1967", SearchField: SearchFirstAlbum},
			want:     []int{1},
		},
		{
			name:     "wrong date year typed first album",
			criteria: Criteria{SearchText: "05.08.1968", SearchField: SearchFirstAlbum},
			want:     []int{},
		},
		{
			name:     "exact creation year typed",
			criteria: Criteria{SearchText: "1970", SearchField: SearchCreationDate},
			want:     []int{1},
		},
		{
			name:     "nearby creation year typed",
			criteria: Criteria{SearchText: "1971", SearchField: SearchCreationDate},
			want:     []int{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Filter(catalog, tt.criteria)
			if err != nil {
				t.Fatalf("Filter returned error: %v", err)
			}
			assertArtistIDs(t, got, tt.want)
		})
	}
}

func TestFilterFuzzyTypedIsolation(t *testing.T) {
	catalog := Catalog{
		Artists: []ArtistEntry{
			queryTestArtist(1, "Qeen", []string{"Ava Stone"}, 1990, "10-01-1995", []string{"paris-france"}),
			queryTestArtist(2, "River Echo", []string{"Queen"}, 1991, "11-01-1995", []string{"london-uk"}),
			queryTestArtist(3, "London Band", []string{"Chris Pike"}, 1992, "12-01-1995", []string{"berlin-germany"}),
		},
	}

	tests := []struct {
		name  string
		query string
		field SearchField
		want  []int
	}{
		{name: "artist", query: "qeen", field: SearchArtist, want: []int{1}},
		{name: "member", query: "qeen", field: SearchMember, want: []int{2}},
		{name: "location", query: "londn", field: SearchLocation, want: []int{2}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Filter(catalog, Criteria{SearchText: tt.query, SearchField: tt.field})
			if err != nil {
				t.Fatalf("Filter returned error: %v", err)
			}
			assertArtistIDs(t, got, tt.want)
		})
	}
}

func TestFilterFuzzyModeSelectionIgnoresOtherFilters(t *testing.T) {
	catalog := Catalog{
		Artists: []ArtistEntry{
			queryTestArtist(1, "Queen", nil, 1970, "14-07-1973", nil),
			queryTestArtist(2, "Qeen", nil, 2000, "15-07-1973", nil),
		},
	}

	got, err := Filter(catalog, Criteria{
		SearchText:   "queen",
		CreationFrom: intPtr(1990),
	})
	if err != nil {
		t.Fatalf("Filter returned error: %v", err)
	}
	assertArtistIDs(t, got, []int{})
}

func TestFilterFuzzyDoesNotMutateCatalogOrCriteriaAndIsDeterministic(t *testing.T) {
	catalog := Catalog{
		Artists: []ArtistEntry{
			queryTestArtist(1, "Queen", []string{"Freddie Mercury"}, 1970, "14-07-1973", []string{"london-uk"}),
			queryTestArtist(2, "Pink Floyd", []string{"Syd Barrett"}, 1965, "05-08-1967", []string{"london-uk"}),
		},
	}
	originalCatalog := cloneTestCatalog(catalog)
	criteria := Criteria{
		SearchText:   "queeen",
		MemberCounts: []int{1},
	}
	originalCriteria := Criteria{
		SearchText:   criteria.SearchText,
		MemberCounts: append([]int(nil), criteria.MemberCounts...),
	}

	first, err := Filter(catalog, criteria)
	if err != nil {
		t.Fatalf("Filter returned error: %v", err)
	}
	assertArtistIDs(t, first, []int{1})

	first[0].Members[0] = "Changed"
	first[0].Locations[0].Hierarchy[0] = "changed"

	if !reflect.DeepEqual(catalog, originalCatalog) {
		t.Fatalf("catalog mutated:\ngot  %#v\nwant %#v", catalog, originalCatalog)
	}
	if !reflect.DeepEqual(criteria, originalCriteria) {
		t.Fatalf("criteria mutated:\ngot  %#v\nwant %#v", criteria, originalCriteria)
	}

	want, err := Filter(catalog, criteria)
	if err != nil {
		t.Fatalf("Filter returned error: %v", err)
	}
	for i := 0; i < 5; i++ {
		got, err := Filter(catalog, criteria)
		if err != nil {
			t.Fatalf("Filter returned error on iteration %d: %v", i, err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("non-deterministic fuzzy result on iteration %d:\ngot  %#v\nwant %#v", i, got, want)
		}
	}
}

func TestFilterTypedSearch(t *testing.T) {
	catalog := queryTestCatalog()
	tests := []struct {
		name  string
		text  string
		field SearchField
		want  []int
	}{
		{
			name:  "artist only",
			text:  "echo lane",
			field: SearchArtist,
			want:  []int{1},
		},
		{
			name:  "member only",
			text:  "echo lane",
			field: SearchMember,
			want:  []int{2},
		},
		{
			name:  "location only",
			text:  "washington",
			field: SearchLocation,
			want:  []int{1},
		},
		{
			name:  "first album only",
			text:  "1984",
			field: SearchFirstAlbum,
			want:  []int{2},
		},
		{
			name:  "creation date only",
			text:  "2001",
			field: SearchCreationDate,
			want:  []int{3},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Filter(catalog, Criteria{
				SearchText:  tt.text,
				SearchField: tt.field,
			})
			if err != nil {
				t.Fatalf("Filter returned error: %v", err)
			}
			assertArtistIDs(t, got, tt.want)
		})
	}
}

func TestFilterEmptySearchTextDoesNotRestrict(t *testing.T) {
	catalog := queryTestCatalog()

	got, err := Filter(catalog, Criteria{
		SearchText:   " \t\n ",
		SearchField:  SearchMember,
		MemberCounts: []int{2},
	})
	if err != nil {
		t.Fatalf("Filter returned error: %v", err)
	}
	assertArtistIDs(t, got, []int{1})
}

func TestFilterCreationRange(t *testing.T) {
	catalog := queryTestCatalog()
	tests := []struct {
		name string
		from *int
		to   *int
		want []int
	}{
		{
			name: "lower bound inclusive",
			from: intPtr(1990),
			want: []int{1, 3},
		},
		{
			name: "upper bound inclusive",
			to:   intPtr(1990),
			want: []int{1, 2},
		},
		{
			name: "both bounds",
			from: intPtr(1985),
			to:   intPtr(1995),
			want: []int{1},
		},
		{
			name: "only lower bound",
			from: intPtr(2000),
			want: []int{3},
		},
		{
			name: "only upper bound",
			to:   intPtr(1985),
			want: []int{2},
		},
		{
			name: "outside range",
			from: intPtr(2010),
			want: []int{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Filter(catalog, Criteria{
				CreationFrom: tt.from,
				CreationTo:   tt.to,
			})
			if err != nil {
				t.Fatalf("Filter returned error: %v", err)
			}
			assertArtistIDs(t, got, tt.want)
		})
	}

	_, err := Filter(catalog, Criteria{
		CreationFrom: intPtr(2000),
		CreationTo:   intPtr(1990),
	})
	if err == nil || !strings.Contains(err.Error(), "creation range") {
		t.Fatalf("expected creation range error, got %v", err)
	}
}

func TestFilterFirstAlbumRange(t *testing.T) {
	catalog := queryTestCatalog()
	tests := []struct {
		name string
		from *time.Time
		to   *time.Time
		want []int
	}{
		{
			name: "lower boundary inclusive",
			from: timePtr(date(1995, time.January, 10)),
			want: []int{1, 3},
		},
		{
			name: "upper boundary inclusive",
			to:   timePtr(date(1995, time.January, 10)),
			want: []int{1, 2},
		},
		{
			name: "both boundaries",
			from: timePtr(date(1990, time.January, 1)),
			to:   timePtr(date(2000, time.January, 1)),
			want: []int{1},
		},
		{
			name: "only lower boundary",
			from: timePtr(date(2000, time.January, 1)),
			want: []int{3},
		},
		{
			name: "only upper boundary",
			to:   timePtr(date(1990, time.January, 1)),
			want: []int{2},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Filter(catalog, Criteria{
				FirstAlbumFrom: tt.from,
				FirstAlbumTo:   tt.to,
			})
			if err != nil {
				t.Fatalf("Filter returned error: %v", err)
			}
			assertArtistIDs(t, got, tt.want)
		})
	}

	_, err := Filter(catalog, Criteria{
		FirstAlbumFrom: timePtr(date(2000, time.January, 1)),
		FirstAlbumTo:   timePtr(date(1990, time.January, 1)),
	})
	if err == nil || !strings.Contains(err.Error(), "first album range") {
		t.Fatalf("expected first album range error, got %v", err)
	}
}

func TestFilterMemberCounts(t *testing.T) {
	catalog := queryTestCatalog()
	tests := []struct {
		name   string
		counts []int
		want   []int
	}{
		{
			name:   "one selected count",
			counts: []int{2},
			want:   []int{1},
		},
		{
			name:   "multiple counts OR",
			counts: []int{1, 3},
			want:   []int{2, 3},
		},
		{
			name:   "duplicate selections",
			counts: []int{1, 1},
			want:   []int{2},
		},
		{
			name:   "empty selection",
			counts: nil,
			want:   []int{1, 2, 3},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Filter(catalog, Criteria{MemberCounts: tt.counts})
			if err != nil {
				t.Fatalf("Filter returned error: %v", err)
			}
			assertArtistIDs(t, got, tt.want)
		})
	}

	for _, count := range []int{0, -1} {
		t.Run("invalid count "+strconv.Itoa(count), func(t *testing.T) {
			_, err := Filter(catalog, Criteria{MemberCounts: []int{count}})
			if err == nil || !strings.Contains(err.Error(), "member count") {
				t.Fatalf("expected member count error, got %v", err)
			}
		})
	}
}

func TestFilterMemberCountAtLeastEight(t *testing.T) {
	catalog := Catalog{
		Artists: []ArtistEntry{
			queryTestArtist(1, "Quartet", []string{"A", "B", "C", "D"}, 1990, "10-01-1995", nil),
			queryTestArtist(2, "Octet", []string{"A", "B", "C", "D", "E", "F", "G", "H"}, 1991, "11-01-1995", nil),
			queryTestArtist(3, "Nonet", []string{"A", "B", "C", "D", "E", "F", "G", "H", "I"}, 1992, "12-01-1995", nil),
		},
	}

	got, err := Filter(catalog, Criteria{
		MemberCounts:   []int{4},
		MinMemberCount: intPtr(8),
	})
	if err != nil {
		t.Fatalf("Filter returned error: %v", err)
	}
	assertArtistIDs(t, got, []int{1, 2, 3})
}

func TestFilterLocations(t *testing.T) {
	catalog := queryTestCatalog()
	tests := []struct {
		name      string
		locations []string
		want      []int
	}{
		{
			name:      "exact full location",
			locations: []string{"Seattle, Washington, USA"},
			want:      []int{1},
		},
		{
			name:      "hierarchy match",
			locations: []string{"Washington, USA"},
			want:      []int{1},
		},
		{
			name:      "multiple selections OR",
			locations: []string{"Los Angeles, USA", "London, UK"},
			want:      []int{2, 3},
		},
		{
			name:      "case and whitespace normalization",
			locations: []string{"  washington,   USA "},
			want:      []int{1},
		},
		{
			name:      "duplicate selections",
			locations: []string{"Washington, USA", " washington, usa "},
			want:      []int{1},
		},
		{
			name:      "unknown location",
			locations: []string{"Berlin, Germany"},
			want:      []int{},
		},
		{
			name:      "empty selections ignored",
			locations: []string{" ", ""},
			want:      []int{1, 2, 3},
		},
		{
			name:      "no arbitrary substring false positive",
			locations: []string{"ashington, usa"},
			want:      []int{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Filter(catalog, Criteria{Locations: tt.locations})
			if err != nil {
				t.Fatalf("Filter returned error: %v", err)
			}
			assertArtistIDs(t, got, tt.want)
		})
	}
}

func TestFilterCombinedCriteriaUsesANDAcrossGroups(t *testing.T) {
	catalog := queryTestCatalog()
	base := Criteria{
		SearchText:     "echo",
		SearchField:    SearchArtist,
		CreationFrom:   intPtr(1985),
		CreationTo:     intPtr(1995),
		FirstAlbumFrom: timePtr(date(1995, time.January, 10)),
		FirstAlbumTo:   timePtr(date(1995, time.January, 10)),
		MemberCounts:   []int{2},
		Locations:      []string{"Washington, USA"},
	}

	got, err := Filter(catalog, base)
	if err != nil {
		t.Fatalf("Filter returned error: %v", err)
	}
	assertArtistIDs(t, got, []int{1})

	tests := []struct {
		name   string
		change func(*Criteria)
	}{
		{
			name: "search fails",
			change: func(c *Criteria) {
				c.SearchText = "missing"
			},
		},
		{
			name: "creation range fails",
			change: func(c *Criteria) {
				c.CreationFrom = intPtr(1991)
			},
		},
		{
			name: "first album range fails",
			change: func(c *Criteria) {
				c.FirstAlbumFrom = timePtr(date(1996, time.January, 1))
				c.FirstAlbumTo = timePtr(date(1996, time.January, 1))
			},
		},
		{
			name: "member count fails",
			change: func(c *Criteria) {
				c.MemberCounts = []int{1}
			},
		},
		{
			name: "location fails",
			change: func(c *Criteria) {
				c.Locations = []string{"London, UK"}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			criteria := base
			criteria.MemberCounts = append([]int(nil), base.MemberCounts...)
			criteria.Locations = append([]string(nil), base.Locations...)
			tt.change(&criteria)

			got, err := Filter(catalog, criteria)
			if err != nil {
				t.Fatalf("Filter returned error: %v", err)
			}
			assertArtistIDs(t, got, []int{})
		})
	}
}

func TestFilterValidation(t *testing.T) {
	catalog := queryTestCatalog()
	tests := []struct {
		name     string
		criteria Criteria
		wantErr  string
	}{
		{
			name:     "unknown search field",
			criteria: Criteria{SearchField: SearchField("album")},
			wantErr:  "search field",
		},
		{
			name: "reversed creation range",
			criteria: Criteria{
				CreationFrom: intPtr(2000),
				CreationTo:   intPtr(1990),
			},
			wantErr: "creation range",
		},
		{
			name: "reversed album range",
			criteria: Criteria{
				FirstAlbumFrom: timePtr(date(2000, time.January, 1)),
				FirstAlbumTo:   timePtr(date(1990, time.January, 1)),
			},
			wantErr: "first album range",
		},
		{
			name:     "invalid member count",
			criteria: Criteria{MemberCounts: []int{-2}},
			wantErr:  "member count",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Filter(catalog, tt.criteria)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestFilterDoesNotMutateCatalogOrCriteria(t *testing.T) {
	catalog := queryTestCatalog()
	originalCatalog := cloneTestCatalog(catalog)
	criteria := Criteria{
		SearchText:   "echo",
		MemberCounts: []int{2, 2},
		Locations:    []string{" Washington, USA ", "Washington, USA"},
	}
	originalMemberCounts := append([]int(nil), criteria.MemberCounts...)
	originalLocations := append([]string(nil), criteria.Locations...)

	got, err := Filter(catalog, criteria)
	if err != nil {
		t.Fatalf("Filter returned error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected one result, got %d", len(got))
	}

	got[0].Members[0] = "Changed"
	got[0].Locations[0].Hierarchy[0] = "changed"

	if !reflect.DeepEqual(catalog, originalCatalog) {
		t.Fatalf("catalog mutated:\ngot  %#v\nwant %#v", catalog, originalCatalog)
	}
	if !reflect.DeepEqual(criteria.MemberCounts, originalMemberCounts) {
		t.Fatalf("criteria member counts mutated: got %v want %v", criteria.MemberCounts, originalMemberCounts)
	}
	if !reflect.DeepEqual(criteria.Locations, originalLocations) {
		t.Fatalf("criteria locations mutated: got %v want %v", criteria.Locations, originalLocations)
	}
}

func TestFilterDeterminism(t *testing.T) {
	catalog := queryTestCatalog()
	criteria := Criteria{
		SearchText:   "usa",
		MemberCounts: []int{1, 2},
		Locations:    []string{"USA"},
	}

	first, err := Filter(catalog, criteria)
	if err != nil {
		t.Fatalf("Filter returned error: %v", err)
	}
	for i := 0; i < 5; i++ {
		got, err := Filter(catalog, criteria)
		if err != nil {
			t.Fatalf("Filter returned error on iteration %d: %v", i, err)
		}
		if !reflect.DeepEqual(got, first) {
			t.Fatalf("non-deterministic result on iteration %d:\ngot  %#v\nwant %#v", i, got, first)
		}
	}
}

func queryTestCatalog() Catalog {
	return Catalog{
		Artists: []ArtistEntry{
			queryTestArtist(1, "Echo Lane", []string{"Ava Stone", "Chris Pike"}, 1990, "10-01-1995", []string{"seattle-washington-usa"}),
			queryTestArtist(2, "Ava Stone", []string{"Echo Lane"}, 1980, "05-05-1984", []string{"los_angeles-usa"}),
			queryTestArtist(3, "Northern Lights", []string{"Solo Star", "Mila Noon", "Zoë Artist"}, 2001, "20-07-2003", []string{"london-uk"}),
		},
	}
}

func queryTestArtist(id int, name string, members []string, creationYear int, firstAlbumRaw string, rawLocations []string) ArtistEntry {
	locations := make([]Location, len(rawLocations))
	for i, rawLocation := range rawLocations {
		locations[i] = parseLocation(rawLocation)
	}
	return ArtistEntry{
		ID:                   id,
		Name:                 name,
		Members:              append([]string(nil), members...),
		CreationYear:         creationYear,
		FirstAlbumRaw:        firstAlbumRaw,
		FirstAlbum:           mustParseTestDate(firstAlbumRaw),
		Locations:            locations,
		NormalizedName:       normalizeText(name),
		NormalizedMembers:    normalizeMembers(members),
		NormalizedFirstAlbum: normalizeText(firstAlbumRaw),
		CreationYearText:     strconv.Itoa(creationYear),
	}
}

func assertArtistIDs(t *testing.T, artists []ArtistEntry, want []int) {
	t.Helper()
	got := make([]int, len(artists))
	for i, artist := range artists {
		got[i] = artist.ID
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("artist IDs mismatch: got %v want %v", got, want)
	}
}

func intPtr(value int) *int {
	return &value
}

func timePtr(value time.Time) *time.Time {
	return &value
}

func date(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

func mustParseTestDate(raw string) time.Time {
	parsed, err := time.ParseInLocation(firstAlbumLayout, raw, time.UTC)
	if err != nil {
		panic(err)
	}
	return parsed
}

func cloneTestCatalog(catalog Catalog) Catalog {
	cloned := Catalog{
		Artists: make([]ArtistEntry, len(catalog.Artists)),
	}
	for i, artist := range catalog.Artists {
		cloned.Artists[i] = cloneArtistEntry(artist)
	}
	return cloned
}
