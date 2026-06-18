package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"groupie-tracker/internal/catalog"
)

func TestDeezerPreviewFindsExactNormalizedArtist(t *testing.T) {
	service, closeServer := testDeezerPreviewService(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/search/artist":
			if got := r.URL.Query().Get("q"); got != "Echo   Lane" {
				t.Fatalf("query = %q, want Echo   Lane", got)
			}
			writeTestDeezerResponse(t, w, `{"data":[{"id":41,"name":"Echo Lane Extended"},{"id":42,"name":"Echo-Lane"}]}`)
		case "/artist/42/top":
			writeTestDeezerResponse(t, w, `{"data":[{"id":123,"title":"Example Track","preview":"https://cdn.example/echo.mp3","artist":{"id":42,"name":"Echo Lane"},"contributors":[{"id":42,"name":"Echo Lane"}]}]}`)
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}), time.Second)
	defer closeServer()

	preview, err := service.Preview(context.Background(), "Echo   Lane")
	if err != nil {
		t.Fatalf("Preview returned error: %v", err)
	}
	assertPreviewResult(t, preview, "https://cdn.example/echo.mp3", "Example Track", "Echo Lane", 123)
}

func TestDeezerPreviewUsesMusicBrainzCanonicalArtistWhenScoreIs100(t *testing.T) {
	service, closeServer := testDeezerPreviewServiceWithMusicBrainz(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ws/2/artist/":
			if got := r.URL.Query().Get("query"); got != "ACDC" {
				t.Fatalf("musicbrainz query = %q, want ACDC", got)
			}
			writeTestDeezerResponse(t, w, `{"artists":[{"score":100,"name":"AC/DC","aliases":[{"name":"ACDC"}]}]}`)
		case "/search/artist":
			if got := r.URL.Query().Get("q"); got != "AC/DC" {
				t.Fatalf("deezer query = %q, want AC/DC", got)
			}
			writeTestDeezerResponse(t, w, `{"data":[{"id":115,"name":"AC/DC"}]}`)
		case "/artist/115/top":
			writeTestDeezerResponse(t, w, `{"data":[{"id":92719900,"title":"Highway to Hell","preview":"https://cdn.example/highway.mp3","artist":{"id":115,"name":"AC/DC"},"contributors":[{"id":115,"name":"AC/DC"}]}]}`)
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}), time.Second)
	defer closeServer()

	preview, err := service.Preview(context.Background(), "ACDC")
	if err != nil {
		t.Fatalf("Preview returned error: %v", err)
	}
	assertPreviewResult(t, preview, "https://cdn.example/highway.mp3", "Highway to Hell", "AC/DC", 92719900)
}

func TestDeezerPreviewFallsBackToLocalFuzzyWhenMusicBrainzHasNoTrustedMatch(t *testing.T) {
	service, closeServer := testDeezerPreviewServiceWithMusicBrainz(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ws/2/artist/":
			if got := r.URL.Query().Get("query"); got != "Bobby McFerrins" {
				t.Fatalf("musicbrainz query = %q, want Bobby McFerrins", got)
			}
			writeTestDeezerResponse(t, w, `{"artists":[{"score":100,"name":"Bobby Cole"},{"score":99,"name":"Bobby Darin"}]}`)
		case "/search/artist":
			if got := r.URL.Query().Get("q"); got != "Bobby McFerrins" {
				t.Fatalf("deezer fallback query = %q, want Bobby McFerrins", got)
			}
			writeTestDeezerResponse(t, w, `{"data":[{"id":3059,"name":"Bobby McFerrin"},{"id":4414790,"name":"Bobby Mc Ferrin"}]}`)
		case "/artist/3059/top":
			writeTestDeezerResponse(t, w, `{"data":[{"id":3127387,"title":"Don't Worry Be Happy","preview":"https://cdn.example/happy.mp3","artist":{"id":3059,"name":"Bobby McFerrin"},"contributors":[{"id":3059,"name":"Bobby McFerrin"}]}]}`)
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}), time.Second)
	defer closeServer()

	preview, err := service.Preview(context.Background(), "Bobby McFerrins")
	if err != nil {
		t.Fatalf("Preview returned error: %v", err)
	}
	assertPreviewResult(t, preview, "https://cdn.example/happy.mp3", "Don't Worry Be Happy", "Bobby McFerrin", 3127387)
}

func TestDeezerPreviewReturnsEmptyWhenExactArtistIsMissing(t *testing.T) {
	service, closeServer := testDeezerPreviewService(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search/artist" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		writeTestDeezerResponse(t, w, `{"data":[{"id":42,"name":"Echo Lane Extended"}]}`)
	}), time.Second)
	defer closeServer()

	preview, err := service.Preview(context.Background(), "Echo Lane")
	if err != nil {
		t.Fatalf("Preview returned error: %v", err)
	}
	assertPreviewResult(t, preview, "", "", "", 0)
}

func TestDeezerPreviewSkipsTrackWithDifferentPrimaryArtist(t *testing.T) {
	service, closeServer := testDeezerPreviewService(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/search/artist":
			writeTestDeezerResponse(t, w, `{"data":[{"id":42,"name":"Echo Lane"}]}`)
		case "/artist/42/top":
			writeTestDeezerResponse(t, w, `{"data":[{"id":901,"title":"Wrong Primary","preview":"https://cdn.example/wrong-primary.mp3","artist":{"id":99,"name":"Guest Artist"},"contributors":[{"id":99,"name":"Guest Artist"}]},{"id":124,"title":"Solo Track","preview":"https://cdn.example/solo.mp3","artist":{"id":42,"name":"Echo Lane"},"contributors":[{"id":42,"name":"Echo Lane"}]}]}`)
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}), time.Second)
	defer closeServer()

	preview, err := service.Preview(context.Background(), "Echo Lane")
	if err != nil {
		t.Fatalf("Preview returned error: %v", err)
	}
	assertPreviewResult(t, preview, "https://cdn.example/solo.mp3", "Solo Track", "Echo Lane", 124)
}

func TestDeezerPreviewSkipsTrackWithOtherContributor(t *testing.T) {
	service, closeServer := testDeezerPreviewService(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/search/artist":
			writeTestDeezerResponse(t, w, `{"data":[{"id":42,"name":"Echo Lane"}]}`)
		case "/artist/42/top":
			writeTestDeezerResponse(t, w, `{"data":[{"id":902,"title":"Collab Track","preview":"https://cdn.example/collab.mp3","artist":{"id":42,"name":"Echo Lane"},"contributors":[{"id":42,"name":"Echo Lane"},{"id":99,"name":"Guest Artist"}]},{"id":125,"title":"Solo Track","preview":"https://cdn.example/solo.mp3","artist":{"id":42,"name":"Echo Lane"},"contributors":[{"id":42,"name":"Echo Lane"}]}]}`)
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}), time.Second)
	defer closeServer()

	preview, err := service.Preview(context.Background(), "Echo Lane")
	if err != nil {
		t.Fatalf("Preview returned error: %v", err)
	}
	assertPreviewResult(t, preview, "https://cdn.example/solo.mp3", "Solo Track", "Echo Lane", 125)
}

func TestDeezerPreviewUsesFirstSoloTrackWithPreview(t *testing.T) {
	service, closeServer := testDeezerPreviewService(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/search/artist":
			writeTestDeezerResponse(t, w, `{"data":[{"id":42,"name":"Echo Lane"}]}`)
		case "/artist/42/top":
			if got := r.URL.Query().Get("limit"); got != "50" {
				t.Fatalf("top limit = %q, want 50", got)
			}
			writeTestDeezerResponse(t, w, `{"data":[{"id":901,"title":"Silent Track","preview":"","artist":{"id":42,"name":"Echo Lane"},"contributors":[{"id":42,"name":"Echo Lane"}]},{"id":126,"title":"Second Track","preview":"https://cdn.example/second.mp3","artist":{"id":42,"name":"Echo Lane"},"contributors":[{"id":42,"name":"Echo Lane"}]},{"id":127,"title":"Third Track","preview":"https://cdn.example/third.mp3","artist":{"id":42,"name":"Echo Lane"},"contributors":[{"id":42,"name":"Echo Lane"}]}]}`)
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}), time.Second)
	defer closeServer()

	preview, err := service.Preview(context.Background(), "Echo Lane")
	if err != nil {
		t.Fatalf("Preview returned error: %v", err)
	}
	assertPreviewResult(t, preview, "https://cdn.example/second.mp3", "Second Track", "Echo Lane", 126)
}

func TestDeezerPreviewReturnsEmptyWhenNoSoloTracksHavePreview(t *testing.T) {
	service, closeServer := testDeezerPreviewService(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/search/artist":
			writeTestDeezerResponse(t, w, `{"data":[{"id":42,"name":"Echo Lane"}]}`)
		case "/artist/42/top":
			writeTestDeezerResponse(t, w, `{"data":[{"id":901,"title":"Silent Track","preview":"","artist":{"id":42,"name":"Echo Lane"},"contributors":[{"id":42,"name":"Echo Lane"}]},{"id":902,"title":"Wrong Primary","preview":"https://cdn.example/wrong-primary.mp3","artist":{"id":99,"name":"Guest Artist"},"contributors":[{"id":99,"name":"Guest Artist"}]},{"id":903,"title":"Collab Track","preview":"https://cdn.example/collab.mp3","artist":{"id":42,"name":"Echo Lane"},"contributors":[{"id":42,"name":"Echo Lane"},{"id":99,"name":"Guest Artist"}]}]}`)
		case "/artist/42/albums":
			writeTestDeezerResponse(t, w, `{"data":[]}`)
		case "/search/playlist":
			writeTestDeezerResponse(t, w, `{"data":[]}`)
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}), time.Second)
	defer closeServer()

	preview, err := service.Preview(context.Background(), "Echo Lane")
	if err != nil {
		t.Fatalf("Preview returned error: %v", err)
	}
	assertPreviewResult(t, preview, "", "", "", 0)
}

func TestDeezerPreviewFallsBackToHydratedAlbumSoloTrack(t *testing.T) {
	service, closeServer := testDeezerPreviewService(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/search/artist":
			writeTestDeezerResponse(t, w, `{"data":[{"id":42,"name":"Echo Lane"}]}`)
		case "/artist/42/top":
			writeTestDeezerResponse(t, w, `{"data":[{"id":901,"title":"Collab Top","preview":"https://cdn.example/top.mp3","artist":{"id":42,"name":"Echo Lane"},"contributors":[{"id":42,"name":"Echo Lane"},{"id":99,"name":"Guest Artist"}]}]}`)
		case "/artist/42/albums":
			if got := r.URL.Query().Get("limit"); got != "20" {
				t.Fatalf("album limit = %q, want 20", got)
			}
			writeTestDeezerResponse(t, w, `{"data":[{"id":11,"title":"Echo Album"}]}`)
		case "/album/11/tracks":
			if got := r.URL.Query().Get("limit"); got != "50" {
				t.Fatalf("track limit = %q, want 50", got)
			}
			writeTestDeezerResponse(t, w, `{"data":[{"id":501,"title":"Looks Solo But Is Collab","preview":"https://cdn.example/collab.mp3","artist":{"id":42,"name":"Echo Lane"}},{"id":502,"title":"Album Solo","preview":"https://cdn.example/album-solo.mp3","artist":{"id":42,"name":"Echo Lane"}}]}`)
		case "/track/501":
			writeTestDeezerResponse(t, w, `{"id":501,"title":"Looks Solo But Is Collab","preview":"https://cdn.example/collab.mp3","artist":{"id":42,"name":"Echo Lane"},"contributors":[{"id":42,"name":"Echo Lane"},{"id":99,"name":"Guest Artist"}]}`)
		case "/track/502":
			writeTestDeezerResponse(t, w, `{"id":502,"title":"Album Solo","preview":"https://cdn.example/album-solo.mp3","artist":{"id":42,"name":"Echo Lane"},"contributors":[{"id":42,"name":"Echo Lane"}]}`)
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}), time.Second)
	defer closeServer()

	preview, err := service.Preview(context.Background(), "Echo Lane")
	if err != nil {
		t.Fatalf("Preview returned error: %v", err)
	}
	assertPreviewResult(t, preview, "https://cdn.example/album-solo.mp3", "Album Solo", "Echo Lane", 502)
}

func TestDeezerPreviewFallsBackToHydratedPlaylistSoloTrack(t *testing.T) {
	service, closeServer := testDeezerPreviewService(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/search/artist":
			writeTestDeezerResponse(t, w, `{"data":[{"id":42,"name":"Echo Lane"}]}`)
		case "/artist/42/top":
			writeTestDeezerResponse(t, w, `{"data":[]}`)
		case "/artist/42/albums":
			writeTestDeezerResponse(t, w, `{"data":[]}`)
		case "/search/playlist":
			if got := r.URL.Query().Get("q"); got != "Echo Lane" {
				t.Fatalf("playlist query = %q, want Echo Lane", got)
			}
			if got := r.URL.Query().Get("limit"); got != "10" {
				t.Fatalf("playlist limit = %q, want 10", got)
			}
			writeTestDeezerResponse(t, w, `{"data":[{"id":77,"title":"100% Echo Lane"}]}`)
		case "/playlist/77":
			writeTestDeezerResponse(t, w, `{"tracks":{"data":[{"id":601,"title":"Playlist Collab","preview":"https://cdn.example/playlist-collab.mp3","artist":{"id":42,"name":"Echo Lane"}},{"id":602,"title":"Playlist Solo","preview":"https://cdn.example/playlist-solo.mp3","artist":{"id":42,"name":"Echo Lane"}}]}}`)
		case "/track/601":
			writeTestDeezerResponse(t, w, `{"id":601,"title":"Playlist Collab","preview":"https://cdn.example/playlist-collab.mp3","artist":{"id":42,"name":"Echo Lane"},"contributors":[{"id":42,"name":"Echo Lane"},{"id":99,"name":"Guest Artist"}]}`)
		case "/track/602":
			writeTestDeezerResponse(t, w, `{"id":602,"title":"Playlist Solo","preview":"https://cdn.example/playlist-solo.mp3","artist":{"id":42,"name":"Echo Lane"},"contributors":[{"id":42,"name":"Echo Lane"}]}`)
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}), time.Second)
	defer closeServer()

	preview, err := service.Preview(context.Background(), "Echo Lane")
	if err != nil {
		t.Fatalf("Preview returned error: %v", err)
	}
	assertPreviewResult(t, preview, "https://cdn.example/playlist-solo.mp3", "Playlist Solo", "Echo Lane", 602)
}

func TestDeezerPreviewErrorIsCachedAsMissing(t *testing.T) {
	var requests int32
	service, closeServer := testDeezerPreviewService(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requests, 1)
		if r.URL.Path != "/search/artist" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		http.Error(w, "deezer unavailable", http.StatusBadGateway)
	}), time.Second)
	defer closeServer()

	preview, err := service.Preview(context.Background(), "Echo Lane")
	if err == nil {
		t.Fatal("expected error")
	}
	assertPreviewResult(t, preview, "", "", "", 0)
	preview, err = service.Preview(context.Background(), "echo lane")
	if err != nil {
		t.Fatalf("cache hit should not return original error: %v", err)
	}
	assertPreviewResult(t, preview, "", "", "", 0)
	if got := atomic.LoadInt32(&requests); got != 1 {
		t.Fatalf("requests = %d, want 1", got)
	}
}

func TestDeezerPreviewTimeoutIsCachedAsMissing(t *testing.T) {
	var requests int32
	service, closeServer := testDeezerPreviewService(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requests, 1)
		time.Sleep(120 * time.Millisecond)
		writeTestDeezerResponse(t, w, `{"data":[{"id":42,"name":"Slow Artist"}]}`)
	}), 20*time.Millisecond)
	defer closeServer()

	start := time.Now()
	preview, err := service.Preview(context.Background(), "Slow Artist")
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if elapsed := time.Since(start); elapsed > 100*time.Millisecond {
		t.Fatalf("timeout took too long: %s", elapsed)
	}
	assertPreviewResult(t, preview, "", "", "", 0)

	preview, err = service.Preview(context.Background(), "slow artist")
	if err != nil {
		t.Fatalf("cache hit should not return original timeout: %v", err)
	}
	assertPreviewResult(t, preview, "", "", "", 0)
	if got := atomic.LoadInt32(&requests); got != 1 {
		t.Fatalf("requests = %d, want 1", got)
	}
}

func TestDeezerPreviewCacheHit(t *testing.T) {
	var requests int32
	service, closeServer := testDeezerPreviewService(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requests, 1)
		switch r.URL.Path {
		case "/search/artist":
			writeTestDeezerResponse(t, w, `{"data":[{"id":42,"name":"Echo Lane"}]}`)
		case "/artist/42/top":
			writeTestDeezerResponse(t, w, `{"data":[{"id":128,"title":"Cached Track","preview":"https://cdn.example/cache.mp3","artist":{"id":42,"name":"Echo Lane"},"contributors":[{"id":42,"name":"Echo Lane"}]}]}`)
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}), time.Second)
	defer closeServer()

	for _, artist := range []string{"Echo   Lane", " echo-lane "} {
		preview, err := service.Preview(context.Background(), artist)
		if err != nil {
			t.Fatalf("Preview(%q) returned error: %v", artist, err)
		}
		assertPreviewResult(t, preview, "https://cdn.example/cache.mp3", "Cached Track", "Echo Lane", 128)
	}
	if got := atomic.LoadInt32(&requests); got != 2 {
		t.Fatalf("requests = %d, want 2", got)
	}
}

func TestDeezerPreviewMissingResultCacheHit(t *testing.T) {
	var requests int32
	service, closeServer := testDeezerPreviewService(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requests, 1)
		switch r.URL.Path {
		case "/search/artist":
			writeTestDeezerResponse(t, w, `{"data":[{"id":42,"name":"No Preview"}]}`)
		case "/artist/42/top":
			writeTestDeezerResponse(t, w, `{"data":[{"id":129,"title":"Silent Track","preview":"","artist":{"id":42,"name":"No Preview"},"contributors":[{"id":42,"name":"No Preview"}]}]}`)
		case "/artist/42/albums":
			writeTestDeezerResponse(t, w, `{"data":[]}`)
		case "/search/playlist":
			writeTestDeezerResponse(t, w, `{"data":[]}`)
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}), time.Second)
	defer closeServer()

	for i := 0; i < 2; i++ {
		preview, err := service.Preview(context.Background(), "No Preview")
		if err != nil {
			t.Fatalf("Preview returned error: %v", err)
		}
		assertPreviewResult(t, preview, "", "", "", 0)
	}
	if got := atomic.LoadInt32(&requests); got != 4 {
		t.Fatalf("requests = %d, want 4", got)
	}
}

func TestDeezerPreviewEndpoint(t *testing.T) {
	mux := initRoutes(dependencies{
		updateNow: func() error {
			return nil
		},
		loadCatalog: func() (catalog.Catalog, error) {
			return testCatalog(), nil
		},
		previewLookup: func(ctx context.Context, artist string) (deezerPreviewResult, error) {
			if artist != "Echo Lane" {
				t.Fatalf("artist = %q", artist)
			}
			return deezerPreviewResult{
				Preview: "https://cdn.example/echo.mp3",
				Title:   "Example Track",
				Artist:  "Echo Lane",
				TrackID: 123,
			}, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/deezer-preview?artist=Echo+Lane", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	res := rec.Result()
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	var got previewResponse
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("decode preview response: %v", err)
	}
	if got.Preview != "https://cdn.example/echo.mp3" {
		t.Fatalf("preview = %q", got.Preview)
	}
	if got.Title != "Example Track" || got.Artist != "Echo Lane" || got.TrackID != 123 {
		t.Fatalf("metadata = title:%q artist:%q track_id:%d", got.Title, got.Artist, got.TrackID)
	}
}

func TestUnavailablePreviewDoesNotBreakCards(t *testing.T) {
	mux := initRoutes(dependencies{
		updateNow: func() error {
			return nil
		},
		loadCatalog: func() (catalog.Catalog, error) {
			return testCatalog(), nil
		},
		previewLookup: func(context.Context, string) (deezerPreviewResult, error) {
			return deezerPreviewResult{}, errors.New("deezer unavailable")
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/deezer-preview?artist=Echo+Lane", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	res := rec.Result()
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if !strings.Contains(string(body), `"preview":""`) {
		t.Fatalf("expected empty preview response, got %s", string(body))
	}
	if !strings.Contains(string(body), `"title":""`) ||
		!strings.Contains(string(body), `"artist":""`) ||
		!strings.Contains(string(body), `"track_id":0`) {
		t.Fatalf("expected empty metadata response, got %s", string(body))
	}

	page := renderRootBody(t, "/")
	for _, want := range []string{
		`href="/echo-lane"`,
		`class="record" aria-hidden="true" data-preview-artist="Echo Lane"`,
	} {
		if !strings.Contains(page, want) {
			t.Fatalf("expected card markup %q", want)
		}
	}
}

func testDeezerPreviewService(t *testing.T, handler http.Handler, timeout time.Duration) (*deezerPreviewService, func()) {
	t.Helper()
	server := httptest.NewServer(handler)
	service := newDeezerPreviewService(server.Client(), server.URL, timeout)
	service.musicBrainzBaseURL = ""
	return service, server.Close
}

func testDeezerPreviewServiceWithMusicBrainz(t *testing.T, handler http.Handler, timeout time.Duration) (*deezerPreviewService, func()) {
	t.Helper()
	server := httptest.NewServer(handler)
	service := newDeezerPreviewService(server.Client(), server.URL, timeout)
	service.musicBrainzBaseURL = server.URL
	return service, server.Close
}

func writeTestDeezerResponse(t *testing.T, w http.ResponseWriter, body string) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write([]byte(body)); err != nil {
		t.Fatalf("write response: %v", err)
	}
}

func assertPreviewResult(t *testing.T, got deezerPreviewResult, preview string, title string, artist string, trackID int64) {
	t.Helper()
	if got.Preview != preview {
		t.Fatalf("preview = %q, want %q", got.Preview, preview)
	}
	if got.Title != title {
		t.Fatalf("title = %q, want %q", got.Title, title)
	}
	if got.Artist != artist {
		t.Fatalf("artist = %q, want %q", got.Artist, artist)
	}
	if got.TrackID != trackID {
		t.Fatalf("track_id = %d, want %d", got.TrackID, trackID)
	}
}
