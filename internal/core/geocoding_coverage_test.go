package core

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"groupie-tracker/internal/geocode"
	"groupie-tracker/internal/model"
)

func TestGeocodingCoverageValidatorUsesOfflineFixture(t *testing.T) {
	relations := loadGeocodingCoverageFixture(t)
	store := geocodingCoverageTestStore(t, []geocode.Entry{
		{Key: "london-uk", Latitude: 51.5074, Longitude: -0.1278},
		{Key: "berlin-germany", Latitude: 52.52, Longitude: 13.405},
		{Key: "seattle-washington-usa", Latitude: 47.6062, Longitude: -122.3321},
	})

	missing := geocode.MissingLocations(relations, store)
	if len(missing) != 0 {
		t.Fatalf("missing = %v, want none", missing)
	}

	report, err := geocode.EnsureCoverage(context.Background(), relations, store, nil, nil)
	if err != nil {
		t.Fatalf("EnsureCoverage returned error: %v", err)
	}
	if report.Total != 3 || report.FromCache != 3 || report.Missing != 0 {
		t.Fatalf("unexpected full-cache report: %#v", report)
	}
}

func TestGeocodingCoverageValidatorReportsMissingAndNormalizesDuplicates(t *testing.T) {
	relations := loadGeocodingCoverageFixture(t)
	store := geocodingCoverageTestStore(t, []geocode.Entry{
		{Key: "london-uk", Latitude: 51.5074, Longitude: -0.1278},
		{Key: "seattle-washington-usa", Latitude: 47.6062, Longitude: -122.3321},
	})

	missing := geocode.MissingLocations(relations, store)
	if len(missing) != 1 || missing[0] != "berlin-germany" {
		t.Fatalf("missing = %v, want [berlin-germany]", missing)
	}

	report, err := geocode.EnsureCoverage(context.Background(), relations, store, nil, nil)
	var coverageErr geocode.CoverageError
	if !errors.As(err, &coverageErr) {
		t.Fatalf("error = %v, want CoverageError", err)
	}
	if report.Total != 3 || report.FromCache != 2 || report.Missing != 1 {
		t.Fatalf("unexpected missing-cache report: %#v", report)
	}
	if len(report.MissingLocations) != 1 || report.MissingLocations[0] != "berlin-germany" {
		t.Fatalf("missing locations = %v, want [berlin-germany]", report.MissingLocations)
	}
}

func loadGeocodingCoverageFixture(t *testing.T) []model.Relation {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "geocoding_relations.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var relations []model.Relation
	if err := json.Unmarshal(data, &relations); err != nil {
		t.Fatalf("decode fixture: %v", err)
	}
	return relations
}

func geocodingCoverageTestStore(t *testing.T, entries []geocode.Entry) *geocode.Store {
	t.Helper()
	store, err := geocode.LoadStore(filepath.Join(t.TempDir(), "geocoding-cache.json"))
	if err != nil {
		t.Fatalf("LoadStore returned error: %v", err)
	}
	for _, entry := range entries {
		if err := store.Set(entry); err != nil {
			t.Fatalf("set %s: %v", entry.Key, err)
		}
	}
	return store
}
