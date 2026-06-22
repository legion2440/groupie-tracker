package core

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"

	"groupie-tracker/internal/model"
)

const (
	connectTimeout      = 3 * time.Second  // Dial (нет сети → ошибка)
	headerTimeout       = 8 * time.Second  // сервер «просыпается»
	overallRequestLimit = 30 * time.Second // всё вместе
)

var (
	artistsURL   = "https://groupietrackers.herokuapp.com/api/artists"
	locationsURL = "https://groupietrackers.herokuapp.com/api/locations"
	datesURL     = "https://groupietrackers.herokuapp.com/api/dates"
	relationsURL = "https://groupietrackers.herokuapp.com/api/relation"
)

// httpClient ограничивает время подключения и полный отклик.
var httpClient = &http.Client{
	Transport: &http.Transport{
		DialContext: (&net.Dialer{
			Timeout: connectTimeout,
		}).DialContext,
		ResponseHeaderTimeout: headerTimeout,
	},
	Timeout: overallRequestLimit,
}

func FetchArtists(ctx context.Context) ([]model.Artist, error) {
	var artists []model.Artist
	if err := fetchJSON(ctx, httpClient, "artists", artistsURL, &artists); err != nil {
		return nil, err
	}
	return artists, nil
}

func FetchLocations(ctx context.Context) ([]model.Location, error) {
	// В API locations данные лежат в поле "locations"
	var result struct {
		Locations []model.Location `json:"locations"`
	}
	if err := fetchJSON(ctx, httpClient, "locations", locationsURL, &result); err != nil {
		return nil, err
	}
	return result.Locations, nil
}

func FetchDates(ctx context.Context) ([]model.Date, error) {
	// В API dates данные лежат в поле "dates"
	var result struct {
		Dates []model.Date `json:"dates"`
	}
	if err := fetchJSON(ctx, httpClient, "dates", datesURL, &result); err != nil {
		return nil, err
	}
	return result.Dates, nil
}

func FetchRelations(ctx context.Context) ([]model.Relation, error) {
	var wrapper struct {
		Index []model.Relation `json:"index"` // <-- ключ правильный
	}
	if err := fetchJSON(ctx, httpClient, "relations", relationsURL, &wrapper); err != nil {
		return nil, err
	}
	return wrapper.Index, nil
}

func fetchJSON(ctx context.Context, client *http.Client, endpointName, endpointURL string, dst any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpointURL, nil)
	if err != nil {
		return fmt.Errorf("fetch %s: create request: %w", endpointName, err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("fetch %s: %w", endpointName, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("fetch %s: upstream returned status %d", endpointName, resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
		return fmt.Errorf("decode %s: %w", endpointName, err)
	}
	return nil
}
