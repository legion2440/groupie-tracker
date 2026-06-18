package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"groupie-tracker/internal/catalog"
)

const (
	deezerAPIBaseURL      = "https://api.deezer.com"
	musicBrainzAPIBaseURL = "https://musicbrainz.org"
	deezerTimeout         = 15 * time.Second
	deezerTopLimit        = 50
	deezerAlbumLimit      = 20
	deezerTrackLimit      = 50
	deezerPlaylistLimit   = 10
)

type previewLookupFunc func(context.Context, string) (deezerPreviewResult, error)

type previewResponse struct {
	Preview string `json:"preview"`
	Title   string `json:"title"`
	Artist  string `json:"artist"`
	TrackID int64  `json:"track_id"`
}

type deezerPreviewResult struct {
	Preview string
	Title   string
	Artist  string
	TrackID int64
}

type deezerArtist struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type deezerContributor struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type deezerTrack struct {
	ID           int64               `json:"id"`
	Title        string              `json:"title"`
	Preview      string              `json:"preview"`
	Artist       deezerArtist        `json:"artist"`
	Contributors []deezerContributor `json:"contributors"`
}

type deezerAlbum struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`
}

type deezerPlaylist struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`
}

type musicBrainzArtist struct {
	Score   int    `json:"score"`
	Name    string `json:"name"`
	Aliases []struct {
		Name string `json:"name"`
	} `json:"aliases"`
}

type deezerPreviewService struct {
	client             *http.Client
	baseURL            string
	musicBrainzBaseURL string
	timeout            time.Duration

	mu    sync.Mutex
	cache map[string]deezerPreviewResult
}

var defaultDeezerPreviewService = newDeezerPreviewService(http.DefaultClient, deezerAPIBaseURL, deezerTimeout)

func newDeezerPreviewService(client *http.Client, baseURL string, timeout time.Duration) *deezerPreviewService {
	if client == nil {
		client = http.DefaultClient
	}
	if timeout <= 0 {
		timeout = deezerTimeout
	}
	return &deezerPreviewService{
		client:             client,
		baseURL:            strings.TrimRight(baseURL, "/"),
		musicBrainzBaseURL: musicBrainzAPIBaseURL,
		timeout:            timeout,
		cache:              make(map[string]deezerPreviewResult),
	}
}

func (service *deezerPreviewService) Preview(ctx context.Context, artist string) (deezerPreviewResult, error) {
	normalizedArtist := catalog.NormalizeSearchText(artist)
	if normalizedArtist == "" {
		return deezerPreviewResult{}, nil
	}

	service.mu.Lock()
	preview, ok := service.cache[normalizedArtist]
	service.mu.Unlock()
	if ok {
		return preview, nil
	}

	preview, err := service.fetchPreview(ctx, artist, normalizedArtist)
	if err != nil {
		preview = deezerPreviewResult{}
	}

	service.mu.Lock()
	service.cache[normalizedArtist] = preview
	service.mu.Unlock()

	return preview, err
}

func (service *deezerPreviewService) fetchPreview(ctx context.Context, artist string, normalizedArtist string) (deezerPreviewResult, error) {
	requestCtx, cancel := context.WithTimeout(ctx, service.timeout)
	defer cancel()

	resolvedArtist, ok, err := service.fetchArtist(requestCtx, artist, normalizedArtist)
	if err != nil || !ok {
		return deezerPreviewResult{}, err
	}

	preview, err := service.fetchArtistTopPreview(requestCtx, resolvedArtist.ID)
	if err != nil {
		return deezerPreviewResult{}, err
	}
	if preview.Preview == "" {
		preview, err = service.fetchArtistAlbumPreview(requestCtx, resolvedArtist.ID)
		if err != nil {
			return deezerPreviewResult{}, err
		}
	}
	if preview.Preview == "" {
		preview, err = service.fetchArtistPlaylistPreview(requestCtx, artist, resolvedArtist)
		if err != nil {
			return deezerPreviewResult{}, err
		}
	}
	if preview.Preview != "" {
		log.Printf("deezer preview selected: requested_artist=%q track_artist=%q title=%q track_id=%d", artist, preview.Artist, preview.Title, preview.TrackID)
	}
	return preview, nil
}

func (service *deezerPreviewService) fetchArtist(ctx context.Context, artist string, normalizedArtist string) (deezerArtist, bool, error) {
	lookupNames := service.musicBrainzArtistLookupNames(ctx, artist, normalizedArtist)
	for _, lookupName := range lookupNames {
		resolvedArtist, ok, err := service.fetchExactDeezerArtist(ctx, lookupName)
		if err != nil || ok {
			return resolvedArtist, ok, err
		}
	}
	return service.fetchFallbackDeezerArtist(ctx, artist)
}

func (service *deezerPreviewService) musicBrainzArtistLookupNames(ctx context.Context, artist string, normalizedArtist string) []string {
	if service.musicBrainzBaseURL == "" {
		return nil
	}

	endpoint, err := url.Parse(service.musicBrainzBaseURL + "/ws/2/artist/")
	if err != nil {
		return nil
	}
	query := endpoint.Query()
	query.Set("query", artist)
	query.Set("fmt", "json")
	query.Set("limit", "5")
	endpoint.RawQuery = query.Encode()

	var body struct {
		Artists []musicBrainzArtist `json:"artists"`
	}
	if err := service.getJSON(ctx, endpoint.String(), &body, "musicbrainz artist lookup"); err != nil {
		return nil
	}

	names := make([]string, 0)
	for _, result := range body.Artists {
		if result.Score != 100 || !musicBrainzArtistMatches(result, artist, normalizedArtist) {
			continue
		}
		names = appendUniqueString(names, result.Name)
		for _, alias := range result.Aliases {
			if catalog.NormalizeSearchText(alias.Name) == normalizedArtist {
				names = appendUniqueString(names, alias.Name)
			}
		}
	}
	return names
}

func musicBrainzArtistMatches(result musicBrainzArtist, artist string, normalizedArtist string) bool {
	if catalog.NormalizeSearchText(result.Name) == normalizedArtist {
		return true
	}
	if _, ok := catalog.SearchTextFuzzyDistance(artist, result.Name); ok {
		return true
	}
	for _, alias := range result.Aliases {
		if catalog.NormalizeSearchText(alias.Name) == normalizedArtist {
			return true
		}
		if _, ok := catalog.SearchTextFuzzyDistance(artist, alias.Name); ok {
			return true
		}
	}
	return false
}

func appendUniqueString(values []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return values
	}
	for _, existing := range values {
		if catalog.NormalizeSearchText(existing) == catalog.NormalizeSearchText(value) {
			return values
		}
	}
	return append(values, value)
}

func (service *deezerPreviewService) fetchExactDeezerArtist(ctx context.Context, artist string) (deezerArtist, bool, error) {
	results, err := service.searchDeezerArtists(ctx, artist)
	if err != nil {
		return deezerArtist{}, false, err
	}
	normalizedArtist := catalog.NormalizeSearchText(artist)
	for _, result := range results {
		if result.ID != 0 && catalog.NormalizeSearchText(result.Name) == normalizedArtist {
			return result, true, nil
		}
	}
	return deezerArtist{}, false, nil
}

func (service *deezerPreviewService) fetchFallbackDeezerArtist(ctx context.Context, artist string) (deezerArtist, bool, error) {
	results, err := service.searchDeezerArtists(ctx, artist)
	if err != nil {
		return deezerArtist{}, false, err
	}
	normalizedArtist := catalog.NormalizeSearchText(artist)
	for _, result := range results {
		if result.ID != 0 && catalog.NormalizeSearchText(result.Name) == normalizedArtist {
			return result, true, nil
		}
	}
	bestDistance := -1
	var bestArtist deezerArtist
	for _, result := range results {
		if result.ID == 0 {
			continue
		}
		distance, ok := catalog.SearchTextFuzzyDistance(artist, result.Name)
		if !ok {
			continue
		}
		if bestDistance == -1 || distance < bestDistance {
			bestDistance = distance
			bestArtist = result
		}
	}
	if bestArtist.ID != 0 {
		return bestArtist, true, nil
	}
	return deezerArtist{}, false, nil
}

func (service *deezerPreviewService) searchDeezerArtists(ctx context.Context, artist string) ([]deezerArtist, error) {
	endpoint, err := url.Parse(service.baseURL + "/search/artist")
	if err != nil {
		return nil, fmt.Errorf("parse deezer artist search URL: %w", err)
	}
	query := endpoint.Query()
	query.Set("q", artist)
	endpoint.RawQuery = query.Encode()

	var body struct {
		Data  []deezerArtist  `json:"data"`
		Error json.RawMessage `json:"error"`
	}
	if err := service.getDeezerJSON(ctx, endpoint.String(), &body); err != nil {
		return nil, err
	}
	if hasDeezerError(body.Error) {
		return nil, fmt.Errorf("deezer returned artist search error")
	}
	return body.Data, nil
}

func (service *deezerPreviewService) fetchArtistTopPreview(ctx context.Context, artistID int64) (deezerPreviewResult, error) {
	endpoint, err := url.Parse(fmt.Sprintf("%s/artist/%d/top", service.baseURL, artistID))
	if err != nil {
		return deezerPreviewResult{}, fmt.Errorf("parse deezer artist top URL: %w", err)
	}
	query := endpoint.Query()
	query.Set("limit", fmt.Sprintf("%d", deezerTopLimit))
	endpoint.RawQuery = query.Encode()

	var body struct {
		Data  []deezerTrack   `json:"data"`
		Error json.RawMessage `json:"error"`
	}
	if err := service.getDeezerJSON(ctx, endpoint.String(), &body); err != nil {
		return deezerPreviewResult{}, err
	}
	if hasDeezerError(body.Error) {
		return deezerPreviewResult{}, fmt.Errorf("deezer returned artist top error")
	}
	return firstSoloTrackPreview(body.Data, artistID), nil
}

func (service *deezerPreviewService) fetchArtistAlbumPreview(ctx context.Context, artistID int64) (deezerPreviewResult, error) {
	endpoint, err := url.Parse(fmt.Sprintf("%s/artist/%d/albums", service.baseURL, artistID))
	if err != nil {
		return deezerPreviewResult{}, fmt.Errorf("parse deezer artist albums URL: %w", err)
	}
	query := endpoint.Query()
	query.Set("limit", fmt.Sprintf("%d", deezerAlbumLimit))
	endpoint.RawQuery = query.Encode()

	var body struct {
		Data  []deezerAlbum   `json:"data"`
		Error json.RawMessage `json:"error"`
	}
	if err := service.getDeezerJSON(ctx, endpoint.String(), &body); err != nil {
		return deezerPreviewResult{}, err
	}
	if hasDeezerError(body.Error) {
		return deezerPreviewResult{}, fmt.Errorf("deezer returned artist albums error")
	}

	for _, album := range body.Data {
		if album.ID == 0 {
			continue
		}
		preview, err := service.fetchAlbumTrackPreview(ctx, album.ID, artistID)
		if err != nil || preview.Preview != "" {
			return preview, err
		}
	}
	return deezerPreviewResult{}, nil
}

func (service *deezerPreviewService) fetchAlbumTrackPreview(ctx context.Context, albumID int64, artistID int64) (deezerPreviewResult, error) {
	endpoint, err := url.Parse(fmt.Sprintf("%s/album/%d/tracks", service.baseURL, albumID))
	if err != nil {
		return deezerPreviewResult{}, fmt.Errorf("parse deezer album tracks URL: %w", err)
	}
	query := endpoint.Query()
	query.Set("limit", fmt.Sprintf("%d", deezerTrackLimit))
	endpoint.RawQuery = query.Encode()

	var body struct {
		Data  []deezerTrack   `json:"data"`
		Error json.RawMessage `json:"error"`
	}
	if err := service.getDeezerJSON(ctx, endpoint.String(), &body); err != nil {
		return deezerPreviewResult{}, err
	}
	if hasDeezerError(body.Error) {
		return deezerPreviewResult{}, fmt.Errorf("deezer returned album tracks error")
	}
	return service.firstHydratedSoloTrackPreview(ctx, body.Data, artistID)
}

func (service *deezerPreviewService) fetchArtistPlaylistPreview(ctx context.Context, requestedArtist string, resolvedArtist deezerArtist) (deezerPreviewResult, error) {
	for _, query := range deezerPlaylistQueries(requestedArtist, resolvedArtist.Name) {
		preview, err := service.fetchPlaylistSearchPreview(ctx, query, resolvedArtist.ID)
		if err != nil || preview.Preview != "" {
			return preview, err
		}
	}
	return deezerPreviewResult{}, nil
}

func deezerPlaylistQueries(requestedArtist string, resolvedArtist string) []string {
	queries := make([]string, 0, 2)
	queries = appendUniqueString(queries, requestedArtist)
	queries = appendUniqueString(queries, resolvedArtist)
	return queries
}

func (service *deezerPreviewService) fetchPlaylistSearchPreview(ctx context.Context, artist string, artistID int64) (deezerPreviewResult, error) {
	endpoint, err := url.Parse(service.baseURL + "/search/playlist")
	if err != nil {
		return deezerPreviewResult{}, fmt.Errorf("parse deezer playlist search URL: %w", err)
	}
	query := endpoint.Query()
	query.Set("q", artist)
	query.Set("limit", fmt.Sprintf("%d", deezerPlaylistLimit))
	endpoint.RawQuery = query.Encode()

	var body struct {
		Data  []deezerPlaylist `json:"data"`
		Error json.RawMessage  `json:"error"`
	}
	if err := service.getDeezerJSON(ctx, endpoint.String(), &body); err != nil {
		return deezerPreviewResult{}, err
	}
	if hasDeezerError(body.Error) {
		return deezerPreviewResult{}, fmt.Errorf("deezer returned playlist search error")
	}
	for _, playlist := range body.Data {
		if playlist.ID == 0 {
			continue
		}
		preview, err := service.fetchPlaylistTrackPreview(ctx, playlist.ID, artistID)
		if err != nil || preview.Preview != "" {
			return preview, err
		}
	}
	return deezerPreviewResult{}, nil
}

func (service *deezerPreviewService) fetchPlaylistTrackPreview(ctx context.Context, playlistID int64, artistID int64) (deezerPreviewResult, error) {
	endpoint, err := url.Parse(fmt.Sprintf("%s/playlist/%d", service.baseURL, playlistID))
	if err != nil {
		return deezerPreviewResult{}, fmt.Errorf("parse deezer playlist URL: %w", err)
	}

	var body struct {
		Tracks struct {
			Data []deezerTrack `json:"data"`
		} `json:"tracks"`
		Error json.RawMessage `json:"error"`
	}
	if err := service.getDeezerJSON(ctx, endpoint.String(), &body); err != nil {
		return deezerPreviewResult{}, err
	}
	if hasDeezerError(body.Error) {
		return deezerPreviewResult{}, fmt.Errorf("deezer returned playlist error")
	}
	return service.firstHydratedSoloTrackPreview(ctx, body.Tracks.Data, artistID)
}

func (service *deezerPreviewService) firstHydratedSoloTrackPreview(ctx context.Context, tracks []deezerTrack, artistID int64) (deezerPreviewResult, error) {
	for _, track := range tracks {
		if strings.TrimSpace(track.Preview) == "" || track.Artist.ID != artistID || track.ID == 0 {
			continue
		}
		detailedTrack, err := service.fetchTrack(ctx, track.ID)
		if err != nil {
			return deezerPreviewResult{}, err
		}
		preview := firstSoloTrackPreview([]deezerTrack{detailedTrack}, artistID)
		if preview.Preview != "" {
			return preview, nil
		}
	}
	return deezerPreviewResult{}, nil
}

func (service *deezerPreviewService) fetchTrack(ctx context.Context, trackID int64) (deezerTrack, error) {
	endpoint, err := url.Parse(fmt.Sprintf("%s/track/%d", service.baseURL, trackID))
	if err != nil {
		return deezerTrack{}, fmt.Errorf("parse deezer track URL: %w", err)
	}

	var body struct {
		ID           int64               `json:"id"`
		Title        string              `json:"title"`
		Preview      string              `json:"preview"`
		Artist       deezerArtist        `json:"artist"`
		Contributors []deezerContributor `json:"contributors"`
		Error        json.RawMessage     `json:"error"`
	}
	if err := service.getDeezerJSON(ctx, endpoint.String(), &body); err != nil {
		return deezerTrack{}, err
	}
	if hasDeezerError(body.Error) {
		return deezerTrack{}, fmt.Errorf("deezer returned track error")
	}
	return deezerTrack{
		ID:           body.ID,
		Title:        body.Title,
		Preview:      body.Preview,
		Artist:       body.Artist,
		Contributors: body.Contributors,
	}, nil
}

func firstSoloTrackPreview(tracks []deezerTrack, artistID int64) deezerPreviewResult {
	for _, track := range tracks {
		if strings.TrimSpace(track.Preview) == "" || track.Artist.ID != artistID {
			continue
		}
		if hasOtherContributor(track.Contributors, artistID) {
			continue
		}
		return deezerPreviewResult{
			Preview: track.Preview,
			Title:   track.Title,
			Artist:  track.Artist.Name,
			TrackID: track.ID,
		}
	}
	return deezerPreviewResult{}
}

func (service *deezerPreviewService) getDeezerJSON(ctx context.Context, endpoint string, target any) error {
	return service.getJSON(ctx, endpoint, target, "deezer preview")
}

func (service *deezerPreviewService) getJSON(ctx context.Context, endpoint string, target any, description string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("build %s request: %w", description, err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "groupie-tracker/1.0")

	resp, err := service.client.Do(req)
	if err != nil {
		return fmt.Errorf("request %s: %w", description, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s returned status %d", description, resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return fmt.Errorf("decode %s response: %w", description, err)
	}
	return nil
}

func hasDeezerError(raw json.RawMessage) bool {
	return len(raw) > 0 && string(raw) != "null"
}

func hasOtherContributor(contributors []deezerContributor, artistID int64) bool {
	for _, contributor := range contributors {
		if contributor.ID != artistID {
			return true
		}
	}
	return false
}

func serveDeezerPreview(w http.ResponseWriter, r *http.Request, lookup previewLookupFunc) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		writeJSON(w, http.StatusMethodNotAllowed, apiErrorResponse{Error: "method not allowed"})
		return
	}

	artist := strings.TrimSpace(r.URL.Query().Get("artist"))
	if artist == "" {
		writeJSON(w, http.StatusOK, previewResponse{})
		return
	}

	preview, err := lookup(r.Context(), artist)
	if err != nil {
		log.Printf("load deezer preview for %q: %v", artist, err)
		preview = deezerPreviewResult{}
	}
	writeJSON(w, http.StatusOK, previewResponse{
		Preview: preview.Preview,
		Title:   preview.Title,
		Artist:  preview.Artist,
		TrackID: preview.TrackID,
	})
}
