package core

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestFetchEndpointsSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/artists":
			_, _ = io.WriteString(w, `[{"id":1,"name":"Queen"}]`)
		case "/api/locations":
			_, _ = io.WriteString(w, `{"locations":[{"id":1,"locations":["london-uk"]}]}`)
		case "/api/dates":
			_, _ = io.WriteString(w, `{"dates":[{"id":1,"dates":["*01-01-2020"]}]}`)
		case "/api/relation":
			_, _ = io.WriteString(w, `{"index":[{"id":1,"datesLocations":{"london-uk":["01-01-2020"]}}]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	setAPIURLsForTest(t, server.URL)

	artists, err := FetchArtists(context.Background())
	if err != nil {
		t.Fatalf("FetchArtists error: %v", err)
	}
	if len(artists) != 1 || artists[0].ID != 1 || artists[0].Name != "Queen" {
		t.Fatalf("unexpected artists: %#v", artists)
	}

	locations, err := FetchLocations(context.Background())
	if err != nil {
		t.Fatalf("FetchLocations error: %v", err)
	}
	if len(locations) != 1 || locations[0].ID != 1 || locations[0].Locations[0] != "london-uk" {
		t.Fatalf("unexpected locations: %#v", locations)
	}

	dates, err := FetchDates(context.Background())
	if err != nil {
		t.Fatalf("FetchDates error: %v", err)
	}
	if len(dates) != 1 || dates[0].ID != 1 || dates[0].Dates[0] != "*01-01-2020" {
		t.Fatalf("unexpected dates: %#v", dates)
	}

	relations, err := FetchRelations(context.Background())
	if err != nil {
		t.Fatalf("FetchRelations error: %v", err)
	}
	if len(relations) != 1 || relations[0].ID != 1 {
		t.Fatalf("unexpected relations: %#v", relations)
	}
	if got := relations[0].DatesLocations["london-uk"]; len(got) != 1 || got[0] != "01-01-2020" {
		t.Fatalf("unexpected datesLocations: %#v", relations[0].DatesLocations)
	}
}

func TestFetchJSONRejectsUpstreamStatus(t *testing.T) {
	for _, status := range []int{http.StatusNotFound, http.StatusInternalServerError} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(status)
				_, _ = io.WriteString(w, `{"valid":"json"}`)
			}))
			defer server.Close()

			var result map[string]any
			err := fetchJSON(context.Background(), server.Client(), "artists", server.URL, &result)
			if err == nil {
				t.Fatal("expected status error")
			}
			want := "fetch artists: upstream returned status " + strconv.Itoa(status)
			if err.Error() != want {
				t.Fatalf("error = %q, want %q", err, want)
			}
		})
	}
}

func TestFetchJSONMalformedJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"broken":`)
	}))
	defer server.Close()

	var result map[string]any
	err := fetchJSON(context.Background(), server.Client(), "artists", server.URL, &result)
	if err == nil || !strings.Contains(err.Error(), "decode artists:") {
		t.Fatalf("expected decode artists error, got %v", err)
	}
}

func TestFetchJSONCanceledContext(t *testing.T) {
	requestStarted := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(requestStarted)
		<-r.Context().Done()
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		var result map[string]any
		done <- fetchJSON(ctx, server.Client(), "artists", server.URL, &result)
	}()

	select {
	case <-requestStarted:
	case <-time.After(time.Second):
		t.Fatal("request did not reach upstream")
	}
	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context cancellation, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("fetch did not stop after context cancellation")
	}
}

func TestFetchJSONTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()

	client := server.Client()
	client.Timeout = 50 * time.Millisecond

	var result map[string]any
	startedAt := time.Now()
	err := fetchJSON(context.Background(), client, "artists", server.URL, &result)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got %v", err)
	}
	if elapsed := time.Since(startedAt); elapsed > time.Second {
		t.Fatalf("fetch exceeded test deadline: %s", elapsed)
	}
}

func TestFetchJSONClosesResponseBody(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusInternalServerError} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			body := &trackingBody{Reader: strings.NewReader(`{"ok":true}`)}
			client := &http.Client{
				Transport: apiRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: status,
						Header:     make(http.Header),
						Body:       body,
						Request:    req,
					}, nil
				}),
			}

			var result map[string]any
			_ = fetchJSON(context.Background(), client, "artists", "http://groupie.test/artists", &result)

			if !body.closed {
				t.Fatalf("response body was not closed for status %d", status)
			}
		})
	}
}

func TestFetchJSONDoesNotDecodeNon2xxBody(t *testing.T) {
	body := &trackingBody{Reader: strings.NewReader(`{"id":1}`)}
	client := &http.Client{
		Transport: apiRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusInternalServerError,
				Header:     make(http.Header),
				Body:       body,
				Request:    req,
			}, nil
		}),
	}

	var result map[string]any
	err := fetchJSON(context.Background(), client, "artists", "http://groupie.test/artists", &result)
	if err == nil {
		t.Fatal("expected status error")
	}
	if body.readCalls != 0 {
		t.Fatalf("non-2xx body was read %d times", body.readCalls)
	}
	if !body.closed {
		t.Fatal("non-2xx body was not closed")
	}
	if result != nil {
		t.Fatalf("non-2xx body was decoded: %#v", result)
	}
}

func setAPIURLsForTest(t *testing.T, baseURL string) {
	t.Helper()

	originalArtistsURL := artistsURL
	originalLocationsURL := locationsURL
	originalDatesURL := datesURL
	originalRelationsURL := relationsURL

	artistsURL = baseURL + "/api/artists"
	locationsURL = baseURL + "/api/locations"
	datesURL = baseURL + "/api/dates"
	relationsURL = baseURL + "/api/relation"

	t.Cleanup(func() {
		artistsURL = originalArtistsURL
		locationsURL = originalLocationsURL
		datesURL = originalDatesURL
		relationsURL = originalRelationsURL
	})
}

type apiRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn apiRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

type trackingBody struct {
	io.Reader
	readCalls int
	closed    bool
}

func (body *trackingBody) Read(p []byte) (int, error) {
	body.readCalls++
	return body.Reader.Read(p)
}

func (body *trackingBody) Close() error {
	body.closed = true
	return nil
}
