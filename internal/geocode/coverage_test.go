package geocode

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"groupie-tracker/internal/model"
)

func TestEnsureCoverageReportsMissingLocations(t *testing.T) {
	store, err := LoadStore(filepath.Join(t.TempDir(), "geocoding-cache.json"))
	if err != nil {
		t.Fatalf("LoadStore returned error: %v", err)
	}
	relations := []model.Relation{
		{
			ID: 1,
			DatesLocations: map[string][]string{
				"london-uk": {"01-01-2020"},
			},
		},
	}

	report, err := EnsureCoverage(context.Background(), relations, store, nil, nil)
	var coverageErr CoverageError
	if !errors.As(err, &coverageErr) {
		t.Fatalf("error = %v, want CoverageError", err)
	}
	if report.Total != 1 || report.Missing != 1 || report.MissingLocations[0] != "london-uk" {
		t.Fatalf("unexpected report: %#v", report)
	}
}

func TestMissingLocationsUsesUniqueNormalizedKeys(t *testing.T) {
	store, err := LoadStore(filepath.Join(t.TempDir(), "geocoding-cache.json"))
	if err != nil {
		t.Fatalf("LoadStore returned error: %v", err)
	}
	if err := store.Set(Entry{Key: "london-uk", Latitude: 51.5074, Longitude: -0.1278}); err != nil {
		t.Fatalf("set london: %v", err)
	}

	missing := MissingLocations([]model.Relation{
		{
			ID: 1,
			DatesLocations: map[string][]string{
				"london-uk":      {"01-01-2020"},
				"London, UK":     {"02-01-2020"},
				"berlin-germany": {"03-01-2020"},
			},
		},
	}, store)

	if len(missing) != 1 || missing[0] != "berlin-germany" {
		t.Fatalf("missing = %v, want [berlin-germany]", missing)
	}
}

func TestEnsureCoverageCanceledBeforeStart(t *testing.T) {
	store, cacheBefore := coverageStoreWithExistingEntry(t)
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		writeExactCoverageResult(w, r)
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	report, err := EnsureCoverage(
		ctx,
		coverageRelations("berlin-germany"),
		store,
		coverageClient(server),
		nil,
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	if report.Total != 0 {
		t.Fatalf("report = %#v, want zero report before start", report)
	}
	if requests.Load() != 0 {
		t.Fatalf("requests = %d, want 0", requests.Load())
	}
	assertCoverageCacheUnchanged(t, store, cacheBefore)
}

func TestEnsureCoverageCanceledDuringRateLimitWait(t *testing.T) {
	store, _ := coverageStoreWithExistingEntry(t)
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		writeExactCoverageResult(w, r)
	}))
	defer server.Close()

	client := NewNominatimClient(NominatimClientConfig{
		BaseURL:     server.URL,
		HTTPClient:  server.Client(),
		MinInterval: time.Hour,
	})
	if _, err := client.Geocode(context.Background(), ParseLocation("london-uk")); err != nil {
		t.Fatalf("prime rate limiter: %v", err)
	}

	ctx := newObservedCancelContext()
	done := make(chan error, 1)
	go func() {
		_, err := EnsureCoverage(ctx, coverageRelations("berlin-germany"), store, client, nil)
		done <- err
	}()

	select {
	case <-ctx.doneObserved:
	case <-time.After(time.Second):
		t.Fatal("coverage did not enter rate-limit wait")
	}
	ctx.cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("coverage did not stop after rate-limit cancellation")
	}
	if requests.Load() != 1 {
		t.Fatalf("requests = %d, want only the priming request", requests.Load())
	}
}

func TestEnsureCoverageCanceledDuringExactRequestStopsFollowingLocations(t *testing.T) {
	store, cacheBefore := coverageStoreWithExistingEntry(t)
	requestStarted := make(chan struct{})
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requests.Add(1) == 1 {
			close(requestStarted)
		}
		<-r.Context().Done()
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := EnsureCoverage(
			ctx,
			coverageRelations("berlin-germany", "paris-france"),
			store,
			coverageClient(server),
			nil,
		)
		done <- err
	}()

	select {
	case <-requestStarted:
	case <-time.After(time.Second):
		t.Fatal("exact request did not start")
	}
	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("coverage did not stop after exact request cancellation")
	}
	if requests.Load() != 1 {
		t.Fatalf("requests = %d, want 1; next location must not be requested", requests.Load())
	}
	assertCoverageCacheUnchanged(t, store, cacheBefore)
}

func TestEnsureCoverageCanceledBeforeFuzzyFallback(t *testing.T) {
	store, _ := coverageStoreWithExistingEntry(t)
	ctx, cancel := context.WithCancel(context.Background())
	var requests atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		_, _ = io.WriteString(w, `[]`)
	}))
	defer server.Close()

	baseTransport := server.Client().Transport
	client := NewNominatimClient(NominatimClientConfig{
		BaseURL: server.URL,
		HTTPClient: &http.Client{
			Transport: cancelAfterFirstReadTransport{
				base:   baseTransport,
				cancel: cancel,
			},
		},
		MinInterval: -1,
	})

	_, err := EnsureCoverage(ctx, coverageRelations("berlin-germany"), store, client, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	if requests.Load() != 1 {
		t.Fatalf("requests = %d, want exact request only", requests.Load())
	}
}

func TestEnsureCoverageBackgroundContextCompletes(t *testing.T) {
	store, _ := coverageStoreWithExistingEntry(t)
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		writeExactCoverageResult(w, r)
	}))
	defer server.Close()

	report, err := EnsureCoverage(
		context.Background(),
		coverageRelations("berlin-germany", "paris-france"),
		store,
		coverageClient(server),
		nil,
	)
	if err != nil {
		t.Fatalf("EnsureCoverage returned error: %v", err)
	}
	if report.Total != 2 || report.AutoFound != 2 || report.Missing != 0 {
		t.Fatalf("unexpected report: %#v", report)
	}
	if requests.Load() != 2 {
		t.Fatalf("requests = %d, want 2", requests.Load())
	}
	for _, key := range []string{"berlin-germany", "paris-france"} {
		if _, ok := store.LookupKey(key); !ok {
			t.Fatalf("missing stored coordinate for %s", key)
		}
	}
}

func coverageClient(server *httptest.Server) *NominatimClient {
	return NewNominatimClient(NominatimClientConfig{
		BaseURL:     server.URL,
		HTTPClient:  server.Client(),
		MinInterval: -1,
	})
}

func coverageRelations(locations ...string) []model.Relation {
	datesLocations := make(map[string][]string, len(locations))
	for _, location := range locations {
		datesLocations[location] = []string{"01-01-2020"}
	}
	return []model.Relation{{ID: 1, DatesLocations: datesLocations}}
}

func coverageStoreWithExistingEntry(t *testing.T) (*Store, []byte) {
	t.Helper()

	store, err := LoadStore(filepath.Join(t.TempDir(), "geocoding-cache.json"))
	if err != nil {
		t.Fatalf("LoadStore returned error: %v", err)
	}
	if err := store.Set(Entry{
		Key:       "london-uk",
		Location:  "London, UK",
		Latitude:  51.5074,
		Longitude: -0.1278,
		Source:    "fixture",
	}); err != nil {
		t.Fatalf("set existing entry: %v", err)
	}
	if err := store.Save(); err != nil {
		t.Fatalf("save existing entry: %v", err)
	}
	data, err := os.ReadFile(store.Path())
	if err != nil {
		t.Fatalf("read existing cache: %v", err)
	}
	return store, data
}

func assertCoverageCacheUnchanged(t *testing.T, store *Store, wantFile []byte) {
	t.Helper()

	coordinate, ok := store.LookupKey("london-uk")
	if !ok || coordinate.Latitude != 51.5074 || coordinate.Longitude != -0.1278 {
		t.Fatalf("existing coordinate changed: %#v, present=%t", coordinate, ok)
	}
	gotFile, err := os.ReadFile(store.Path())
	if err != nil {
		t.Fatalf("read cache after cancellation: %v", err)
	}
	if !bytes.Equal(gotFile, wantFile) {
		t.Fatalf("cache file changed after cancellation:\ngot:\n%s\nwant:\n%s", gotFile, wantFile)
	}
}

func writeExactCoverageResult(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Query().Get("q") {
	case "London, United Kingdom":
		_, _ = io.WriteString(w, `[{"lat":"51.5074","lon":"-0.1278","address":{"city":"London","country":"United Kingdom","country_code":"gb"}}]`)
	case "Berlin, Germany":
		_, _ = io.WriteString(w, `[{"lat":"52.5200","lon":"13.4050","address":{"city":"Berlin","country":"Germany","country_code":"de"}}]`)
	case "Paris, France":
		_, _ = io.WriteString(w, `[{"lat":"48.8566","lon":"2.3522","address":{"city":"Paris","country":"France","country_code":"fr"}}]`)
	default:
		http.Error(w, "unexpected query", http.StatusBadRequest)
	}
}

type observedCancelContext struct {
	context.Context
	done         chan struct{}
	doneObserved chan struct{}
	observeOnce  sync.Once
	cancelOnce   sync.Once
}

func newObservedCancelContext() *observedCancelContext {
	return &observedCancelContext{
		Context:      context.Background(),
		done:         make(chan struct{}),
		doneObserved: make(chan struct{}),
	}
}

func (ctx *observedCancelContext) Done() <-chan struct{} {
	ctx.observeOnce.Do(func() {
		close(ctx.doneObserved)
	})
	return ctx.done
}

func (ctx *observedCancelContext) Err() error {
	select {
	case <-ctx.done:
		return context.Canceled
	default:
		return nil
	}
}

func (ctx *observedCancelContext) cancel() {
	ctx.cancelOnce.Do(func() {
		close(ctx.done)
	})
}

type cancelAfterFirstReadTransport struct {
	base   http.RoundTripper
	cancel context.CancelFunc
}

func (transport cancelAfterFirstReadTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := transport.base.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	resp.Body = &cancelAfterFirstReadBody{
		ReadCloser: resp.Body,
		cancel:     transport.cancel,
	}
	return resp, nil
}

type cancelAfterFirstReadBody struct {
	io.ReadCloser
	cancel context.CancelFunc
	once   sync.Once
}

func (body *cancelAfterFirstReadBody) Read(p []byte) (int, error) {
	n, err := body.ReadCloser.Read(p)
	if n > 0 {
		body.once.Do(body.cancel)
	}
	return n, err
}
