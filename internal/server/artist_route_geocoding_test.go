package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"groupie-tracker/internal/geocode"
	"groupie-tracker/internal/model"
)

func TestArtistSlugRouteUsesRawLocationKeyForCoordinates(t *testing.T) {
	var lookupCalls []string
	mux := artistRouteMuxForGeocodingTest(
		[]model.Relation{
			{
				ID: 1,
				DatesLocations: map[string][]string{
					"seattle-washington-usa": {"10-01-1995"},
				},
			},
		},
		func(rawLocation string) (geocode.Coordinate, bool) {
			lookupCalls = append(lookupCalls, rawLocation)
			if rawLocation != "seattle-washington-usa" {
				return geocode.Coordinate{}, false
			}
			return geocode.Coordinate{Latitude: 47.6062, Longitude: -122.3321}, true
		},
	)

	res, body := performArtistRouteRequest(t, mux, "/echo-lane")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", res.StatusCode, body)
	}
	if len(lookupCalls) != 1 || lookupCalls[0] != "seattle-washington-usa" {
		t.Fatalf("lookup calls = %v, want [seattle-washington-usa]", lookupCalls)
	}
	for _, want := range []string{
		`data-latitude="47.606200"`,
		`data-longitude="-122.332100"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected %q in body: %s", want, body)
		}
	}
	if strings.Contains(body, `data-latitude="0.000000"`) || strings.Contains(body, `data-longitude="0.000000"`) {
		t.Fatalf("coordinates were replaced with zero values: %s", body)
	}
}

func TestArtistSlugRouteMissingCoordinateReturnsServerError(t *testing.T) {
	mux := artistRouteMuxForGeocodingTest(
		[]model.Relation{
			{
				ID: 1,
				DatesLocations: map[string][]string{
					"seattle-washington-usa": {"10-01-1995"},
				},
			},
		},
		func(string) (geocode.Coordinate, bool) {
			return geocode.Coordinate{}, false
		},
	)

	res, body := performArtistRouteRequest(t, mux, "/echo-lane")
	if res.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500; body: %s", res.StatusCode, body)
	}
	for _, forbidden := range []string{
		`data-latitude="0.000000"`,
		`data-longitude="0.000000"`,
		`class="concert-location`,
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("unexpected partial coordinate render %q in body: %s", forbidden, body)
		}
	}
}

func TestArtistSlugRouteKeepsCoordinatesWithSortedConcerts(t *testing.T) {
	var lookupCalls []string
	mux := artistRouteMuxForGeocodingTest(
		[]model.Relation{
			{
				ID: 1,
				DatesLocations: map[string][]string{
					"seattle-washington-usa": {"10-01-1995"},
					"london-uk":              {"01-01-1995"},
					"tokyo-japan":            {"05-01-1995"},
				},
			},
		},
		func(rawLocation string) (geocode.Coordinate, bool) {
			lookupCalls = append(lookupCalls, rawLocation)
			coordinates := map[string]geocode.Coordinate{
				"london-uk":              {Latitude: 51.5074, Longitude: -0.1278},
				"seattle-washington-usa": {Latitude: 47.6062, Longitude: -122.3321},
				"tokyo-japan":            {Latitude: 35.6762, Longitude: 139.6503},
			}
			coordinate, ok := coordinates[rawLocation]
			return coordinate, ok
		},
	)

	res, body := performArtistRouteRequest(t, mux, "/echo-lane")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", res.StatusCode, body)
	}
	for _, want := range []string{"seattle-washington-usa", "london-uk", "tokyo-japan"} {
		if !containsStringForTest(lookupCalls, want) {
			t.Fatalf("lookup calls = %v, missing %s", lookupCalls, want)
		}
	}
	if len(lookupCalls) != 3 {
		t.Fatalf("lookup call count = %d, want 3: %v", len(lookupCalls), lookupCalls)
	}

	firstCard := firstConcertCard(t, body)
	for _, want := range []string{
		`London, UK`,
		`01-01-1995`,
		`data-latitude="51.507400"`,
		`data-longitude="-0.127800"`,
	} {
		if !strings.Contains(firstCard, want) {
			t.Fatalf("expected first sorted card to contain %q, got: %s", want, firstCard)
		}
	}
	for _, want := range []string{
		`Seattle, Washington, USA`,
		`data-latitude="47.606200"`,
		`data-longitude="-122.332100"`,
		`Tokyo, Japan`,
		`data-latitude="35.676200"`,
		`data-longitude="139.650300"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected body to contain %q", want)
		}
	}
}

func TestArtistSlugRouteInvalidConcertDateReturnsServerError(t *testing.T) {
	var lookupCalls []string
	mux := artistRouteMuxForGeocodingTest(
		[]model.Relation{
			{
				ID: 1,
				DatesLocations: map[string][]string{
					"seattle-washington-usa": {"not-a-date"},
				},
			},
		},
		func(rawLocation string) (geocode.Coordinate, bool) {
			lookupCalls = append(lookupCalls, rawLocation)
			return geocode.Coordinate{Latitude: 47.6062, Longitude: -122.3321}, true
		},
	)

	res, body := performArtistRouteRequest(t, mux, "/echo-lane")
	if res.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500; body: %s", res.StatusCode, body)
	}
	if len(lookupCalls) != 0 {
		t.Fatalf("lookup should not run after invalid date, got calls %v", lookupCalls)
	}
	if strings.Contains(body, `class="concert-location`) || strings.Contains(body, `not-a-date`) {
		t.Fatalf("invalid date caused partial or leaked render: %s", body)
	}
}

func artistRouteMuxForGeocodingTest(relations []model.Relation, lookup coordinateLookupFunc) *http.ServeMux {
	deps := testDependenciesWithCatalog(testCatalog(), nil)
	deps.loadRelations = func() ([]model.Relation, error) {
		return relations, nil
	}
	deps.lookupCoordinate = lookup
	return initRoutes(deps)
}

func performArtistRouteRequest(t *testing.T, mux *http.ServeMux, path string) (*http.Response, string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	res := rec.Result()
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	return res, string(body)
}

func firstConcertCard(t *testing.T, body string) string {
	t.Helper()
	start := strings.Index(body, `class="concert-location`)
	if start < 0 {
		t.Fatalf("concert card not found: %s", body)
	}
	open := strings.LastIndex(body[:start], "<article")
	if open < 0 {
		t.Fatalf("concert card opening article not found: %s", body)
	}
	end := strings.Index(body[open:], "</article>")
	if end < 0 {
		t.Fatalf("concert card closing article not found: %s", body)
	}
	return body[open : open+end]
}
