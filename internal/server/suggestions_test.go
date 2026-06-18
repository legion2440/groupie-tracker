package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"groupie-tracker/internal/catalog"
)

func TestSuggestionsHandlerValidSuggestions(t *testing.T) {
	cat := testCatalog()
	mux := initRoutes(testDependenciesWithCatalog(cat, nil))

	res, body := performSuggestionsRequest(t, mux, http.MethodGet, "/api/search/suggestions?q=echo+lane")

	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}
	if ct := res.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("expected JSON Content-Type, got %q", ct)
	}
	got := decodeSuggestionsResponse(t, body)
	want := suggestionResponsesFromCatalog(cat, "echo lane")
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("suggestions mismatch:\ngot  %#v\nwant %#v", got, want)
	}
}

func TestSuggestionsHandlerEmptyQuery(t *testing.T) {
	mux := initRoutes(testDependenciesWithCatalog(testCatalog(), nil))

	res, body := performSuggestionsRequest(t, mux, http.MethodGet, "/api/search/suggestions?q=+")

	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}
	if strings.TrimSpace(string(body)) == "null" {
		t.Fatal("expected empty JSON array, got null")
	}
	got := decodeSuggestionsResponse(t, body)
	if len(got) != 0 {
		t.Fatalf("expected empty suggestions, got %#v", got)
	}
}

func TestSuggestionsHandlerAllTypes(t *testing.T) {
	tests := []struct {
		name      string
		query     string
		wantValue string
		wantType  string
	}{
		{name: "artist", query: "echo", wantValue: "Echo Lane", wantType: "artist"},
		{name: "member", query: "solo", wantValue: "Solo Star", wantType: "member"},
		{name: "location", query: "washington", wantValue: "Seattle, Washington, USA", wantType: "location"},
		{name: "first album", query: "05-05-1984", wantValue: "05-05-1984", wantType: "first_album"},
		{name: "creation date", query: "1980", wantValue: "1980", wantType: "creation_date"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := initRoutes(testDependenciesWithCatalog(testCatalog(), nil))
			escaped := strings.ReplaceAll(tt.query, " ", "+")
			res, body := performSuggestionsRequest(t, mux, http.MethodGet, "/api/search/suggestions?q="+escaped)

			if res.StatusCode != http.StatusOK {
				t.Fatalf("expected 200, got %d", res.StatusCode)
			}
			got := decodeSuggestionsResponse(t, body)
			if !containsSuggestionResponse(got, tt.wantValue, tt.wantType) {
				t.Fatalf("expected %q/%q in %#v", tt.wantValue, tt.wantType, got)
			}
		})
	}
}

func TestSuggestionsHandlerSameValueDifferentTypes(t *testing.T) {
	mux := initRoutes(testDependenciesWithCatalog(testCatalog(), nil))

	res, body := performSuggestionsRequest(t, mux, http.MethodGet, "/api/search/suggestions?q=echo+lane")

	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}
	got := decodeSuggestionsResponse(t, body)
	if !containsSuggestionResponse(got, "Echo Lane", "artist") {
		t.Fatalf("expected artist suggestion in %#v", got)
	}
	if !containsSuggestionResponse(got, "Echo Lane", "member") {
		t.Fatalf("expected member suggestion in %#v", got)
	}
}

func TestSuggestionsHandlerMethodValidation(t *testing.T) {
	loadCalls := 0
	mux := initRoutes(testDependenciesWithCatalog(testCatalog(), &loadCalls))

	res, body := performSuggestionsRequest(t, mux, http.MethodPost, "/api/search/suggestions?q=echo")

	if res.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", res.StatusCode)
	}
	if allow := res.Header.Get("Allow"); allow != http.MethodGet {
		t.Fatalf("Allow header mismatch: got %q", allow)
	}
	assertJSONError(t, body)
	if loadCalls != 0 {
		t.Fatalf("expected loader not to be called, got %d", loadCalls)
	}
}

func TestSuggestionsHandlerQueryTooLong(t *testing.T) {
	loadCalls := 0
	mux := initRoutes(testDependenciesWithCatalog(testCatalog(), &loadCalls))

	res, body := performSuggestionsRequest(t, mux, http.MethodGet, "/api/search/suggestions?q="+strings.Repeat("ж", 201))

	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", res.StatusCode)
	}
	assertJSONError(t, body)
	if loadCalls != 0 {
		t.Fatalf("expected loader not to be called, got %d", loadCalls)
	}
}

func TestSuggestionsHandlerLoaderFailure(t *testing.T) {
	mux := initRoutes(dependencies{
		updateNow: func() error {
			return nil
		},
		loadCatalog: func() (catalog.Catalog, error) {
			return catalog.Catalog{}, errors.New("private fixture loader failure")
		},
	})

	res, body := performSuggestionsRequest(t, mux, http.MethodGet, "/api/search/suggestions?q=echo")

	if res.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", res.StatusCode)
	}
	assertJSONError(t, body)
	if strings.Contains(string(body), "private fixture") {
		t.Fatalf("internal error leaked in response: %s", string(body))
	}
}

func TestSuggestionsHandlerNoMatches(t *testing.T) {
	mux := initRoutes(testDependenciesWithCatalog(testCatalog(), nil))

	res, body := performSuggestionsRequest(t, mux, http.MethodGet, "/api/search/suggestions?q=not-present")

	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}
	got := decodeSuggestionsResponse(t, body)
	if len(got) != 0 {
		t.Fatalf("expected no suggestions, got %#v", got)
	}
}

func performSuggestionsRequest(t *testing.T, mux http.Handler, method string, path string) (*http.Response, []byte) {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	res := rec.Result()
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	return res, body
}

func decodeSuggestionsResponse(t *testing.T, body []byte) []suggestionResponse {
	t.Helper()
	var got []suggestionResponse
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("decode suggestions response: %v; body=%s", err, string(body))
	}
	if got == nil {
		return []suggestionResponse{}
	}
	return got
}

func suggestionResponsesFromCatalog(cat catalog.Catalog, query string) []suggestionResponse {
	suggestions := catalog.Suggest(cat, query)
	response := make([]suggestionResponse, 0, len(suggestions))
	for _, suggestion := range suggestions {
		response = append(response, suggestionResponse{
			Value: suggestion.Value,
			Label: suggestion.Label,
			Type:  string(suggestion.Field),
		})
	}
	return response
}

func containsSuggestionResponse(suggestions []suggestionResponse, value string, typ string) bool {
	for _, suggestion := range suggestions {
		if suggestion.Value == value && suggestion.Type == typ {
			return true
		}
	}
	return false
}

func assertJSONError(t *testing.T, body []byte) {
	t.Helper()
	var got apiErrorResponse
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("decode JSON error: %v; body=%s", err, string(body))
	}
	if got.Error == "" {
		t.Fatalf("expected JSON error, got %#v", got)
	}
}
