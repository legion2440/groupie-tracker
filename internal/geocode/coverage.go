package geocode

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"

	"groupie-tracker/internal/model"
)

type CoverageReport struct {
	Total            int
	FromCache        int
	AutoFound        int
	FuzzyFound       int
	Missing          int
	MissingLocations []string
}

type CoverageError struct {
	MissingLocations []string
}

func (e CoverageError) Error() string {
	return "geocoding coordinates missing for: " + strings.Join(e.MissingLocations, ", ")
}

func UniqueLocationSpecs(relations []model.Relation) []LocationSpec {
	byKey := make(map[string]LocationSpec)
	for _, relation := range relations {
		for rawLocation := range relation.DatesLocations {
			spec := ParseLocation(rawLocation)
			if spec.Key == "" {
				continue
			}
			existing, exists := byKey[spec.Key]
			if !exists || spec.Raw < existing.Raw {
				byKey[spec.Key] = spec
			}
		}
	}
	specs := make([]LocationSpec, 0, len(byKey))
	for _, spec := range byKey {
		specs = append(specs, spec)
	}
	sort.Slice(specs, func(i, j int) bool {
		return specs[i].Key < specs[j].Key
	})
	return specs
}

func EnsureCoverage(ctx context.Context, relations []model.Relation, store *Store, client *NominatimClient, logger *log.Logger) (CoverageReport, error) {
	if store == nil {
		return CoverageReport{}, fmt.Errorf("geocoding store is nil")
	}
	if err := ctx.Err(); err != nil {
		return CoverageReport{}, err
	}
	if logger == nil {
		logger = log.Default()
	}

	specs := UniqueLocationSpecs(relations)
	report := CoverageReport{Total: len(specs)}
	added := false

	for _, spec := range specs {
		if err := ctx.Err(); err != nil {
			return report, err
		}
		if _, ok := store.LookupKey(spec.Key); ok {
			report.FromCache++
			continue
		}

		logger.Printf("geocoding unknown location %q (%s)", spec.Display, spec.Key)
		if client == nil {
			report.MissingLocations = append(report.MissingLocations, spec.Key)
			logger.Printf("location without coordinates and no Nominatim client: %s", spec.Key)
			continue
		}

		match, err := client.Geocode(ctx, spec)
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return report, ctxErr
			}
			report.MissingLocations = append(report.MissingLocations, spec.Key)
			logger.Printf("location without coordinates: %s: %v", spec.Key, err)
			continue
		}
		if err := ctx.Err(); err != nil {
			return report, err
		}
		entry := Entry{
			Key:       spec.Key,
			Location:  spec.Display,
			Latitude:  match.Latitude,
			Longitude: match.Longitude,
			Source:    "nominatim",
			Match:     string(match.Method),
		}
		if err := store.Set(entry); err != nil {
			report.MissingLocations = append(report.MissingLocations, spec.Key)
			logger.Printf("location without valid coordinates: %s: %v", spec.Key, err)
			continue
		}
		added = true
		switch match.Method {
		case MatchFuzzy:
			report.FuzzyFound++
		default:
			report.AutoFound++
		}
	}

	if added {
		if err := ctx.Err(); err != nil {
			return report, err
		}
		if err := store.Save(); err != nil {
			return report, err
		}
	}

	if len(report.MissingLocations) > 0 {
		sort.Strings(report.MissingLocations)
		report.Missing = len(report.MissingLocations)
		return report, CoverageError{MissingLocations: append([]string(nil), report.MissingLocations...)}
	}
	return report, nil
}

func MissingLocations(relations []model.Relation, store *Store) []string {
	specs := UniqueLocationSpecs(relations)
	missing := make([]string, 0)
	for _, spec := range specs {
		if _, ok := store.LookupKey(spec.Key); !ok {
			missing = append(missing, spec.Key)
		}
	}
	sort.Strings(missing)
	return missing
}
