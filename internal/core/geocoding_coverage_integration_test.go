//go:build integration

package core

import (
	"testing"

	"groupie-tracker/internal/geocode"
)

func TestGeocodingCacheCoversFetchedRelations(t *testing.T) {
	relations, err := FetchRelations()
	if err != nil {
		t.Fatalf("FetchRelations returned error: %v", err)
	}
	store, err := geocode.LoadStore(geocode.DefaultCachePath())
	if err != nil {
		t.Fatalf("LoadStore returned error: %v", err)
	}
	missing := geocode.MissingLocations(relations, store)
	if len(missing) > 0 {
		t.Fatalf("geocoding cache missing %d locations: %v", len(missing), missing)
	}
}
