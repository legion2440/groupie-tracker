package server

import (
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"groupie-tracker/internal/catalog"
)

func TestParseCriteria(t *testing.T) {
	t.Run("empty query", func(t *testing.T) {
		got, err := parseCriteria(url.Values{})
		if err != nil {
			t.Fatalf("parseCriteria returned error: %v", err)
		}
		if !reflect.DeepEqual(got, catalog.Criteria{}) {
			t.Fatalf("criteria mismatch: got %#v", got)
		}
	})

	t.Run("broad search", func(t *testing.T) {
		got, err := parseCriteria(url.Values{"q": {"phil"}})
		if err != nil {
			t.Fatalf("parseCriteria returned error: %v", err)
		}
		if got.SearchText != "phil" || got.SearchField != catalog.SearchAny {
			t.Fatalf("search criteria mismatch: %#v", got)
		}
	})

	t.Run("typed search", func(t *testing.T) {
		tests := []catalog.SearchField{
			catalog.SearchArtist,
			catalog.SearchMember,
			catalog.SearchLocation,
			catalog.SearchFirstAlbum,
			catalog.SearchCreationDate,
		}
		for _, field := range tests {
			t.Run(string(field), func(t *testing.T) {
				got, err := parseCriteria(url.Values{"q": {"phil"}, "search_type": {string(field)}})
				if err != nil {
					t.Fatalf("parseCriteria returned error: %v", err)
				}
				if got.SearchField != field {
					t.Fatalf("search field mismatch: got %q want %q", got.SearchField, field)
				}
			})
		}
	})

	t.Run("empty normalized query clears typed search", func(t *testing.T) {
		for _, values := range []url.Values{
			{"q": {""}, "search_type": {"member"}},
			{"q": {"   "}, "search_type": {"location"}},
			{"q": {"---"}, "search_type": {"first_album"}},
		} {
			got, err := parseCriteria(values)
			if err != nil {
				t.Fatalf("parseCriteria returned error for %v: %v", values, err)
			}
			if got.SearchField != catalog.SearchAny {
				t.Fatalf("expected SearchAny for %v, got %q", values, got.SearchField)
			}
		}
	})
}

func TestParseCriteriaValidation(t *testing.T) {
	t.Run("unknown search type", func(t *testing.T) {
		if _, err := parseCriteria(url.Values{"search_type": {"band"}}); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("search length", func(t *testing.T) {
		if _, err := parseCriteria(url.Values{"q": {strings.Repeat("ж", 200)}}); err != nil {
			t.Fatalf("expected 200 runes to be accepted: %v", err)
		}
		if _, err := parseCriteria(url.Values{"q": {strings.Repeat("ж", 201)}}); err == nil {
			t.Fatal("expected 201 runes to be rejected")
		}
	})
}

func TestParseCriteriaCreationBounds(t *testing.T) {
	tests := []struct {
		name string
		in   url.Values
		from *int
		to   *int
	}{
		{
			name: "both valid",
			in:   url.Values{"creation_from": {"1970"}, "creation_to": {"1980"}},
			from: intPtrForServerTest(1970),
			to:   intPtrForServerTest(1980),
		},
		{
			name: "lower only",
			in:   url.Values{"creation_from": {"1970"}},
			from: intPtrForServerTest(1970),
		},
		{
			name: "upper only",
			in:   url.Values{"creation_to": {"1980"}},
			to:   intPtrForServerTest(1980),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseCriteria(tt.in)
			if err != nil {
				t.Fatalf("parseCriteria returned error: %v", err)
			}
			assertIntPtrForServerTest(t, got.CreationFrom, tt.from)
			assertIntPtrForServerTest(t, got.CreationTo, tt.to)
		})
	}

	for _, in := range []url.Values{
		{"creation_from": {"bad"}},
		{"creation_from": {"2000"}, "creation_to": {"1990"}},
	} {
		if _, err := parseCriteria(in); err == nil {
			t.Fatalf("expected error for %v", in)
		}
	}
}

func TestParseCriteriaAlbumBounds(t *testing.T) {
	tests := []struct {
		name string
		in   url.Values
		from *time.Time
		to   *time.Time
	}{
		{
			name: "both valid",
			in:   url.Values{"album_from": {"1970-01-02"}, "album_to": {"1980-03-04"}},
			from: timePtrForServerTest(dateForServerTest(1970, time.January, 2)),
			to:   timePtrForServerTest(dateForServerTest(1980, time.March, 4)),
		},
		{
			name: "lower only",
			in:   url.Values{"album_from": {"1970-01-02"}},
			from: timePtrForServerTest(dateForServerTest(1970, time.January, 2)),
		},
		{
			name: "upper only",
			in:   url.Values{"album_to": {"1980-03-04"}},
			to:   timePtrForServerTest(dateForServerTest(1980, time.March, 4)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseCriteria(tt.in)
			if err != nil {
				t.Fatalf("parseCriteria returned error: %v", err)
			}
			assertTimePtrForServerTest(t, got.FirstAlbumFrom, tt.from)
			assertTimePtrForServerTest(t, got.FirstAlbumTo, tt.to)
		})
	}

	for _, in := range []url.Values{
		{"album_from": {"bad"}},
		{"album_from": {"2000-01-01"}, "album_to": {"1990-01-01"}},
	} {
		if _, err := parseCriteria(in); err == nil {
			t.Fatalf("expected error for %v", in)
		}
	}
}

func TestParseCriteriaRepeatedMembersAndLocations(t *testing.T) {
	values, err := url.ParseQuery("members=1&members=4&members=8%2B&locations=London%2C+UK&locations=Paris%2C+France")
	if err != nil {
		t.Fatalf("parse query: %v", err)
	}

	got, err := parseCriteria(values)
	if err != nil {
		t.Fatalf("parseCriteria returned error: %v", err)
	}
	if want := []int{1, 4}; !reflect.DeepEqual(got.MemberCounts, want) {
		t.Fatalf("member counts mismatch: got %v want %v", got.MemberCounts, want)
	}
	if got.MinMemberCount == nil || *got.MinMemberCount != 8 {
		t.Fatalf("minimum member count mismatch: got %v want 8", got.MinMemberCount)
	}
	if want := []string{"London, UK", "Paris, France"}; !reflect.DeepEqual(got.Locations, want) {
		t.Fatalf("locations mismatch: got %v want %v", got.Locations, want)
	}

	for _, values := range []url.Values{
		{"members": {"bad"}},
		{"members": {"0"}},
		{"members": {"-1"}},
	} {
		if _, err := parseCriteria(values); err == nil {
			t.Fatalf("expected error for %v", values)
		}
	}
}

func intPtrForServerTest(value int) *int {
	return &value
}

func timePtrForServerTest(value time.Time) *time.Time {
	return &value
}

func dateForServerTest(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

func assertIntPtrForServerTest(t *testing.T, got *int, want *int) {
	t.Helper()
	if got == nil || want == nil {
		if got != want {
			t.Fatalf("int pointer mismatch: got %v want %v", got, want)
		}
		return
	}
	if *got != *want {
		t.Fatalf("int pointer value mismatch: got %d want %d", *got, *want)
	}
}

func assertTimePtrForServerTest(t *testing.T, got *time.Time, want *time.Time) {
	t.Helper()
	if got == nil || want == nil {
		if got != want {
			t.Fatalf("time pointer mismatch: got %v want %v", got, want)
		}
		return
	}
	if !got.Equal(*want) {
		t.Fatalf("time pointer value mismatch: got %v want %v", *got, *want)
	}
}
