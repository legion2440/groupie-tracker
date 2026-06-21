package core

import (
	"encoding/json"
	"fmt"
	"io"
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

func FetchArtists() ([]model.Artist, error) {
	resp, err := httpClient.Get(artistsURL)
	if err != nil {
		return nil, fmt.Errorf("fetch artists: %w", err)
	}
	defer resp.Body.Close()

	var artists []model.Artist
	if err := json.NewDecoder(resp.Body).Decode(&artists); err != nil {
		return nil, fmt.Errorf("decode artists: %w", err)
	}
	return artists, nil
}

func FetchLocations() ([]model.Location, error) {
	resp, err := httpClient.Get(locationsURL)
	if err != nil {
		return nil, fmt.Errorf("fetch locations: %w", err)
	}
	defer resp.Body.Close()

	// В API locations данные лежат в поле "locations"
	var result struct {
		Locations []model.Location `json:"locations"`
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read locations: %w", err)
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decode locations: %w", err)
	}
	return result.Locations, nil
}

func FetchDates() ([]model.Date, error) {
	resp, err := httpClient.Get(datesURL)
	if err != nil {
		return nil, fmt.Errorf("fetch dates: %w", err)
	}
	defer resp.Body.Close()

	// В API dates данные лежат в поле "dates"
	var result struct {
		Dates []model.Date `json:"dates"`
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read dates: %w", err)
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decode dates: %w", err)
	}
	return result.Dates, nil
}

func FetchRelations() ([]model.Relation, error) {
	resp, err := httpClient.Get(relationsURL)
	if err != nil {
		return nil, fmt.Errorf("fetch relations: %w", err)
	}
	defer resp.Body.Close()

	var wrapper struct {
		Index []model.Relation `json:"index"` // <-- ключ правильный
	}
	if err := json.NewDecoder(resp.Body).Decode(&wrapper); err != nil {
		return nil, fmt.Errorf("decode relations: %w", err)
	}
	return wrapper.Index, nil
}
