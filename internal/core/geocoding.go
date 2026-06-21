package core

import (
	"context"
	"fmt"
	"log"
	"sync"

	"groupie-tracker/internal/geocode"
	"groupie-tracker/internal/model"
)

var (
	geocodingMu    sync.Mutex
	geocodingStore *geocode.Store
)

func loadGeocodingStore() (*geocode.Store, error) {
	geocodingMu.Lock()
	defer geocodingMu.Unlock()

	return loadGeocodingStoreLocked()
}

func loadGeocodingStoreLocked() (*geocode.Store, error) {

	if geocodingStore != nil {
		return geocodingStore, nil
	}
	store, err := geocode.LoadStore(geocode.DefaultCachePath())
	if err != nil {
		return nil, err
	}
	geocodingStore = store
	return geocodingStore, nil
}

func LookupLocationCoordinate(rawLocation string) (geocode.Coordinate, bool) {
	geocodingMu.Lock()
	defer geocodingMu.Unlock()

	store, err := loadGeocodingStoreLocked()
	if err != nil {
		log.Printf("load geocoding cache: %v", err)
		return geocode.Coordinate{}, false
	}
	return store.Lookup(rawLocation)
}

func EnsureGeocodingCoverage(relations []model.Relation) (geocode.CoverageReport, error) {
	geocodingMu.Lock()
	defer geocodingMu.Unlock()

	store, err := loadGeocodingStoreLocked()
	if err != nil {
		return geocode.CoverageReport{}, fmt.Errorf("load geocoding cache: %w", err)
	}
	client := geocode.NewNominatimClient(geocode.NominatimClientConfig{
		Logger: log.Default(),
	})
	report, err := geocode.EnsureCoverage(context.Background(), relations, store, client, log.Default())
	if err != nil {
		return report, err
	}
	return report, nil
}
