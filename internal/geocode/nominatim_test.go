package geocode

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestSelectMatchExactBeforeFuzzy(t *testing.T) {
	spec := ParseLocation("london-uk")
	match, err := SelectMatch(spec, []NominatimResult{
		nominatimFixture("Lonndon", "United Kingdom", "1", "1"),
		nominatimFixture("London", "United Kingdom", "51.5074", "-0.1278"),
	})
	if err != nil {
		t.Fatalf("SelectMatch returned error: %v", err)
	}
	if match.Method != MatchExact {
		t.Fatalf("method = %q, want exact", match.Method)
	}
	if match.Latitude != 51.5074 || match.Longitude != -0.1278 {
		t.Fatalf("coordinate mismatch: %#v", match.Coordinate)
	}
}

func TestSelectMatchFuzzyFallbackAfterExactFails(t *testing.T) {
	spec := ParseLocation("london-uk")
	match, err := SelectMatch(spec, []NominatimResult{
		nominatimFixture("Lonndon", "United Kingdom", "51.5074", "-0.1278"),
	})
	if err != nil {
		t.Fatalf("SelectMatch returned error: %v", err)
	}
	if match.Method != MatchFuzzy {
		t.Fatalf("method = %q, want fuzzy", match.Method)
	}
}

func TestSelectMatchRejectsWrongCityOrCountry(t *testing.T) {
	tests := []struct {
		name   string
		result NominatimResult
	}{
		{name: "wrong city", result: nominatimFixture("Paris", "United Kingdom", "48.8566", "2.3522")},
		{name: "wrong country", result: nominatimFixture("London", "Canada", "42.9849", "-81.2453")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := SelectMatch(ParseLocation("london-uk"), []NominatimResult{tt.result})
			if !errors.Is(err, ErrNoMatch) {
				t.Fatalf("error = %v, want ErrNoMatch", err)
			}
		})
	}
}

func TestSelectMatchRejectsAmbiguousFuzzyResult(t *testing.T) {
	_, err := SelectMatch(ParseLocation("london-uk"), []NominatimResult{
		nominatimFixture("Lonndon", "United Kingdom", "51.5", "-0.1"),
		nominatimFixture("Londom", "United Kingdom", "52.0", "-0.2"),
	})
	if !errors.Is(err, ErrAmbiguousMatch) {
		t.Fatalf("error = %v, want ErrAmbiguousMatch", err)
	}
}

func TestNominatimClientHTTPScenarios(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		delay      time.Duration
		wantErr    string
	}{
		{
			name:       "exact response",
			statusCode: http.StatusOK,
			body:       `[{"lat":"51.5074","lon":"-0.1278","display_name":"London, United Kingdom","address":{"city":"London","country":"United Kingdom","country_code":"gb"}}]`,
		},
		{
			name:       "http error",
			statusCode: http.StatusBadGateway,
			body:       `bad gateway`,
			wantErr:    "HTTP status",
		},
		{
			name:       "malformed response",
			statusCode: http.StatusOK,
			body:       `[`,
			wantErr:    "decode",
		},
		{
			name:       "empty response",
			statusCode: http.StatusOK,
			body:       `[]`,
			wantErr:    ErrNoResults.Error(),
		},
		{
			name:       "timeout",
			statusCode: http.StatusOK,
			body:       `[]`,
			delay:      40 * time.Millisecond,
			wantErr:    "context deadline exceeded",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var requestCount int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				atomic.AddInt32(&requestCount, 1)
				if got := r.Header.Get("User-Agent"); !strings.Contains(got, "groupie-tracker-geolocalization") {
					t.Fatalf("unexpected User-Agent %q", got)
				}
				if tt.delay > 0 {
					time.Sleep(tt.delay)
				}
				w.WriteHeader(tt.statusCode)
				_, _ = fmt.Fprint(w, tt.body)
			}))
			defer server.Close()

			client := NewNominatimClient(NominatimClientConfig{
				BaseURL: server.URL,
				HTTPClient: &http.Client{
					Timeout: 10 * time.Millisecond,
				},
				MinInterval: -1,
			})
			match, err := client.Geocode(context.Background(), ParseLocation("london-uk"))
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("Geocode returned error: %v", err)
				}
				if match.Method != MatchExact {
					t.Fatalf("method = %q, want exact", match.Method)
				}
			} else if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v, want containing %q", err, tt.wantErr)
			}
			wantRequests := int32(1)
			if tt.name == "empty response" {
				wantRequests = 2
			}
			if requestCount != wantRequests {
				t.Fatalf("request count = %d, want %d", requestCount, wantRequests)
			}
		})
	}
}

func TestNominatimClientExactFoundDoesNotFallback(t *testing.T) {
	var queries []url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		queries = append(queries, r.URL.Query())
		_, _ = io.WriteString(w, `[{"lat":"47.6062","lon":"-122.3321","display_name":"Seattle, Washington, United States","address":{"city":"Seattle","state":"Washington","country":"United States","country_code":"us"}}]`)
	}))
	defer server.Close()

	client := NewNominatimClient(NominatimClientConfig{BaseURL: server.URL, MinInterval: -1})
	match, err := client.Geocode(context.Background(), ParseLocation("seattle-washington-usa"))
	if err != nil {
		t.Fatalf("Geocode returned error: %v", err)
	}
	if match.Method != MatchExact {
		t.Fatalf("method = %q, want exact", match.Method)
	}
	if len(queries) != 1 {
		t.Fatalf("request count = %d, want 1", len(queries))
	}
	if got := queries[0].Get("q"); got != "Seattle, Washington, United States" {
		t.Fatalf("primary q = %q", got)
	}
}

func TestNominatimClientFallbackAfterEmptyPrimary(t *testing.T) {
	var queries []url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		queries = append(queries, query)
		if len(queries) == 1 {
			_, _ = io.WriteString(w, `[]`)
			return
		}
		if query.Get("city") != "Seattle" || query.Get("country") != "USA" || query.Get("q") != "" {
			t.Fatalf("unexpected fallback query: %s", r.URL.RawQuery)
		}
		_, _ = io.WriteString(w, `[{"lat":"47.6062","lon":"-122.3321","display_name":"Seattl, Washington, United States","address":{"city":"Seattl","country":"United States","country_code":"us"}}]`)
	}))
	defer server.Close()

	client := NewNominatimClient(NominatimClientConfig{BaseURL: server.URL, MinInterval: -1})
	match, err := client.Geocode(context.Background(), ParseLocation("seattle-washington-usa"))
	if err != nil {
		t.Fatalf("Geocode returned error: %v", err)
	}
	if match.Method != MatchFuzzy {
		t.Fatalf("method = %q, want fuzzy", match.Method)
	}
	if len(queries) != 2 {
		t.Fatalf("request count = %d, want 2", len(queries))
	}
}

func TestNominatimClientFallbackAfterPrimaryCandidatesWithoutExact(t *testing.T) {
	var queries []url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		queries = append(queries, query)
		if len(queries) == 1 {
			_, _ = io.WriteString(w, `[{"lat":"47.6062","lon":"-122.3321","display_name":"Seattl, Washington, United States","address":{"city":"Seattl","country":"United States","country_code":"us"}}]`)
			return
		}
		if query.Get("city") != "Seattle" || query.Get("country") != "USA" {
			t.Fatalf("unexpected fallback query: %s", r.URL.RawQuery)
		}
		_, _ = io.WriteString(w, `[{"lat":"47.6062","lon":"-122.3321","display_name":"Seattl, Washington, United States","address":{"city":"Seattl","country":"United States","country_code":"us"}}]`)
	}))
	defer server.Close()

	client := NewNominatimClient(NominatimClientConfig{BaseURL: server.URL, MinInterval: -1})
	match, err := client.Geocode(context.Background(), ParseLocation("seattle-washington-usa"))
	if err != nil {
		t.Fatalf("Geocode returned error: %v", err)
	}
	if match.Method != MatchFuzzy {
		t.Fatalf("method = %q, want fuzzy", match.Method)
	}
	if len(queries) != 2 {
		t.Fatalf("request count = %d, want 2", len(queries))
	}
}

func TestNominatimClientFallbackRejectsWrongCountry(t *testing.T) {
	server := twoStepNominatimServer(t, `[]`, `[{"lat":"47.6062","lon":"-122.3321","display_name":"Seattl, British Columbia, Canada","address":{"city":"Seattl","state":"USA","country":"Canada","country_code":"ca"}}]`)
	defer server.Close()

	client := NewNominatimClient(NominatimClientConfig{BaseURL: server.URL, MinInterval: -1})
	_, err := client.Geocode(context.Background(), ParseLocation("seattle-washington-usa"))
	if !errors.Is(err, ErrNoMatch) {
		t.Fatalf("error = %v, want ErrNoMatch", err)
	}
}

func TestNominatimClientFallbackRejectsAmbiguousFuzzy(t *testing.T) {
	server := twoStepNominatimServer(t, `[]`, `[
		{"lat":"47.6062","lon":"-122.3321","display_name":"Seattl, Washington, United States","address":{"city":"Seattl","country":"United States","country_code":"us"}},
		{"lat":"47.7000","lon":"-122.4000","display_name":"Seatlle, Washington, United States","address":{"city":"Seatlle","country":"United States","country_code":"us"}}
	]`)
	defer server.Close()

	client := NewNominatimClient(NominatimClientConfig{BaseURL: server.URL, MinInterval: -1})
	_, err := client.Geocode(context.Background(), ParseLocation("seattle-washington-usa"))
	if !errors.Is(err, ErrAmbiguousMatch) {
		t.Fatalf("error = %v, want ErrAmbiguousMatch", err)
	}
}

func TestNominatimClientBothQueriesEmptyReturnsNoResults(t *testing.T) {
	server := twoStepNominatimServer(t, `[]`, `[]`)
	defer server.Close()

	client := NewNominatimClient(NominatimClientConfig{BaseURL: server.URL, MinInterval: -1})
	_, err := client.Geocode(context.Background(), ParseLocation("seattle-washington-usa"))
	if !errors.Is(err, ErrNoResults) {
		t.Fatalf("error = %v, want ErrNoResults", err)
	}
}

func TestNominatimClientHTTPErrorDoesNotFallback(t *testing.T) {
	var requestCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		http.Error(w, "bad gateway", http.StatusBadGateway)
	}))
	defer server.Close()

	client := NewNominatimClient(NominatimClientConfig{BaseURL: server.URL, MinInterval: -1})
	_, err := client.Geocode(context.Background(), ParseLocation("seattle-washington-usa"))
	if err == nil || !strings.Contains(err.Error(), "HTTP status 502") {
		t.Fatalf("error = %v, want HTTP status 502", err)
	}
	if requestCount != 1 {
		t.Fatalf("request count = %d, want 1", requestCount)
	}
}

func TestNominatimClientRateLimitAppliesToFallback(t *testing.T) {
	var requestTimes []time.Time
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestTimes = append(requestTimes, time.Now())
		if len(requestTimes) == 1 {
			_, _ = io.WriteString(w, `[]`)
			return
		}
		_, _ = io.WriteString(w, `[{"lat":"47.6062","lon":"-122.3321","display_name":"Seattl, Washington, United States","address":{"city":"Seattl","country":"United States","country_code":"us"}}]`)
	}))
	defer server.Close()

	client := NewNominatimClient(NominatimClientConfig{
		BaseURL:     server.URL,
		MinInterval: 25 * time.Millisecond,
	})
	if _, err := client.Geocode(context.Background(), ParseLocation("seattle-washington-usa")); err != nil {
		t.Fatalf("Geocode returned error: %v", err)
	}
	if len(requestTimes) != 2 {
		t.Fatalf("request count = %d, want 2", len(requestTimes))
	}
	if elapsed := requestTimes[1].Sub(requestTimes[0]); elapsed < 20*time.Millisecond {
		t.Fatalf("fallback request was not rate limited enough: %s", elapsed)
	}
}

func TestSelectMatchRejectsCountryFromStateOrRegion(t *testing.T) {
	_, err := SelectMatch(ParseLocation("georgia-usa"), []NominatimResult{
		{
			Lat:         "42.3154",
			Lon:         "43.3569",
			Name:        "Georgia",
			DisplayName: "Georgia, USA",
			Address: nominatimAddress{
				City:        "Georgia",
				State:       "USA",
				Region:      "USA",
				Country:     "Canada",
				CountryCode: "ca",
			},
		},
	})
	if !errors.Is(err, ErrNoMatch) {
		t.Fatalf("error = %v, want ErrNoMatch", err)
	}
}

func TestCityNameVariantsDoNotUseLastWordHeuristic(t *testing.T) {
	tests := []struct {
		name      string
		city      string
		forbidden string
	}{
		{name: "belo horizonte", city: "Belo Horizonte", forbidden: "Horizonte"},
		{name: "new york", city: "New York", forbidden: "York"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, variant := range cityNameVariants(tt.city) {
				if normalizeMatchText(variant) == normalizeMatchText(tt.forbidden) {
					t.Fatalf("cityNameVariants(%q) contains forbidden standalone variant %q: %v", tt.city, tt.forbidden, cityNameVariants(tt.city))
				}
			}
		})
	}
}

func TestCityNameVariantsKeepBoundedNormalizations(t *testing.T) {
	variants := cityNameVariants("City of São Paulo")
	if !containsNormalizedVariant(variants, "Sao Paulo") {
		t.Fatalf("expected diacritic/prefix normalization for São Paulo, got %v", variants)
	}
	if !containsNormalizedVariant(cityNameVariants("Saint Louis"), "St Louis") {
		t.Fatalf("expected Saint to St variant")
	}
	if !containsNormalizedVariant(cityNameVariants("St Louis"), "Saint Louis") {
		t.Fatalf("expected St to Saint variant")
	}
}

func twoStepNominatimServer(t *testing.T, firstBody string, secondBody string) *httptest.Server {
	t.Helper()
	var requestCount int32
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&requestCount, 1)
		switch count {
		case 1:
			_, _ = io.WriteString(w, firstBody)
		case 2:
			_, _ = io.WriteString(w, secondBody)
		default:
			t.Fatalf("unexpected request %d: %s", count, r.URL.RawQuery)
		}
	}))
}

func containsNormalizedVariant(variants []string, want string) bool {
	for _, variant := range variants {
		if normalizeMatchText(variant) == normalizeMatchText(want) {
			return true
		}
	}
	return false
}

func nominatimFixture(city string, country string, lat string, lon string) NominatimResult {
	return NominatimResult{
		Lat:         lat,
		Lon:         lon,
		DisplayName: city + ", " + country,
		Address: nominatimAddress{
			City:        city,
			Country:     country,
			CountryCode: country,
		},
	}
}
