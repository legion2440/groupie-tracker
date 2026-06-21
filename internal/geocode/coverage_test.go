package geocode

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

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
