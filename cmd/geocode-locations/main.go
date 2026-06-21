package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"groupie-tracker/internal/core"
	"groupie-tracker/internal/geocode"
)

func main() {
	cachePath := flag.String("cache", geocode.DefaultCachePath(), "path to geocoding JSON cache")
	baseURL := flag.String("base-url", geocode.DefaultNominatimURL, "Nominatim search endpoint")
	timeout := flag.Duration("timeout", 8*time.Second, "per-request HTTP timeout")
	minInterval := flag.Duration("interval", time.Second, "minimum interval between Nominatim requests")
	flag.Parse()

	relations, err := core.FetchRelations()
	if err != nil {
		log.Fatalf("fetch relations: %v", err)
	}
	store, err := geocode.LoadStore(*cachePath)
	if err != nil {
		log.Fatalf("load geocoding cache: %v", err)
	}
	client := geocode.NewNominatimClient(geocode.NominatimClientConfig{
		BaseURL: *baseURL,
		HTTPClient: &http.Client{
			Timeout: *timeout,
		},
		MinInterval: *minInterval,
		Logger:      log.Default(),
	})

	report, err := geocode.EnsureCoverage(context.Background(), relations, store, client, log.Default())
	fmt.Printf("Unique locations: %d\n", report.Total)
	fmt.Printf("Already in JSON: %d\n", report.FromCache)
	fmt.Printf("Found automatically: %d\n", report.AutoFound)
	fmt.Printf("Found through fuzzy fallback: %d\n", report.FuzzyFound)
	fmt.Printf("Missing or rejected: %d\n", report.Missing)
	stats := cacheStats(store.Entries())
	fmt.Printf("JSON entries: %d\n", stats.total)
	fmt.Printf("JSON exact matches: %d\n", stats.exact)
	fmt.Printf("JSON fuzzy matches: %d\n", stats.fuzzy)
	fmt.Printf("JSON manual entries: %d\n", stats.manual)
	if len(report.MissingLocations) > 0 {
		fmt.Println("Missing or ambiguous locations:")
		for _, location := range report.MissingLocations {
			fmt.Printf("- %s\n", location)
		}
	}
	if err != nil {
		log.Printf("geocoding coverage incomplete: %v", err)
		os.Exit(1)
	}
}

type stats struct {
	total  int
	exact  int
	fuzzy  int
	manual int
}

func cacheStats(entries []geocode.Entry) stats {
	var result stats
	result.total = len(entries)
	for _, entry := range entries {
		switch entry.Match {
		case string(geocode.MatchFuzzy):
			result.fuzzy++
		case string(geocode.MatchExact):
			result.exact++
		}
		if entry.Source == "manual" || entry.Match == "manual" {
			result.manual++
		}
	}
	return result
}
