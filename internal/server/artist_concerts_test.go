package server

import (
	"bytes"
	"strings"
	"testing"

	"groupie-tracker/internal/geocode"
	"groupie-tracker/internal/model"
)

func TestBuildConcertsSortsByEarliestDate(t *testing.T) {
	relations := []model.Relation{
		{
			ID: 1,
			DatesLocations: map[string][]string{
				"tokyo-japan":            {"03-01-2020"},
				"london-uk":              {"01-01-2020", "05-01-2020"},
				"seattle-washington-usa": {"02-01-2020"},
			},
		},
	}

	concerts, err := buildConcertsByLocation(1, relations, testCoordinateLookup)
	if err != nil {
		t.Fatalf("buildConcertsByLocation returned error: %v", err)
	}
	got := []string{concerts[0].Location, concerts[1].Location, concerts[2].Location}
	want := []string{"London, UK", "Seattle, Washington, USA", "Tokyo, Japan"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order = %v, want %v", got, want)
		}
	}
	if gotDates := concerts[0].Dates; gotDates[0] != "01-01-2020" || gotDates[1] != "05-01-2020" {
		t.Fatalf("dates were not sorted: %v", gotDates)
	}
}

func TestBuildConcertsTieBreakerUsesNormalizedLocation(t *testing.T) {
	relations := []model.Relation{
		{
			ID: 1,
			DatesLocations: map[string][]string{
				"berlin-germany":        {"01-01-2020"},
				"amsterdam-netherlands": {"01-01-2020"},
			},
		},
	}

	concerts, err := buildConcertsByLocation(1, relations, testCoordinateLookup)
	if err != nil {
		t.Fatalf("buildConcertsByLocation returned error: %v", err)
	}
	if got := concerts[0].Location; got != "Amsterdam, Netherlands" {
		t.Fatalf("first location = %q, want Amsterdam, Netherlands", got)
	}
}

func TestBuildConcertsReturnsErrorForInvalidDate(t *testing.T) {
	relations := []model.Relation{
		{
			ID: 1,
			DatesLocations: map[string][]string{
				"london-uk": {"not-a-date"},
			},
		},
	}

	_, err := buildConcertsByLocation(1, relations, testCoordinateLookup)
	if err == nil || !strings.Contains(err.Error(), "not-a-date") {
		t.Fatalf("error = %v, want date parse context", err)
	}
}

func TestBuildConcertsRequiresCoordinates(t *testing.T) {
	relations := []model.Relation{
		{
			ID: 1,
			DatesLocations: map[string][]string{
				"london-uk": {"01-01-2020"},
			},
		},
	}

	_, err := buildConcertsByLocation(1, relations, func(string) (geocode.Coordinate, bool) {
		return geocode.Coordinate{}, false
	})
	if err == nil || !strings.Contains(err.Error(), "missing coordinates") {
		t.Fatalf("error = %v, want missing coordinates", err)
	}
}

func TestArtistTemplateIncludesCoordinateDataAttributes(t *testing.T) {
	var buf bytes.Buffer
	err := tmplAll.ExecuteTemplate(&buf, "artist.html", ArtistDetailPage{
		Artist: model.Artist{
			ID:           1,
			Name:         "Echo Lane",
			Image:        "echo.jpeg",
			Members:      []string{"Ava Stone"},
			CreationDate: 1990,
			FirstAlbum:   "10-01-1995",
		},
		Concerts: []ConcertsByLocation{
			{
				Location:  "Seattle, Washington, USA",
				Dates:     []string{"10-01-1995"},
				Latitude:  47.6062,
				Longitude: -122.3321,
			},
		},
	})
	if err != nil {
		t.Fatalf("render artist template: %v", err)
	}
	body := buf.String()
	for _, want := range []string{
		`data-latitude="47.606200"`,
		`data-longitude="-122.332100"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected coordinate attribute %q in body", want)
		}
	}
}

func TestArtistTemplateIncludesOpenStreetMapGeocodingAttribution(t *testing.T) {
	var buf bytes.Buffer
	err := tmplAll.ExecuteTemplate(&buf, "artist.html", ArtistDetailPage{
		Artist: model.Artist{
			ID:           1,
			Name:         "Echo Lane",
			Image:        "echo.jpeg",
			Members:      []string{"Ava Stone"},
			CreationDate: 1990,
			FirstAlbum:   "10-01-1995",
		},
		Concerts: []ConcertsByLocation{
			{
				Location:  "Seattle, Washington, USA",
				Dates:     []string{"10-01-1995"},
				Latitude:  47.6062,
				Longitude: -122.3321,
			},
		},
	})
	if err != nil {
		t.Fatalf("render artist template: %v", err)
	}
	body := buf.String()
	for _, want := range []string{
		`class="tour-map__geocoding-attribution"`,
		`Geocoding &copy;`,
		`href="https://www.openstreetmap.org/copyright"`,
		`target="_blank"`,
		`rel="noopener noreferrer"`,
		`OpenStreetMap contributors`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected attribution marker %q in body", want)
		}
	}
}

func testCoordinateLookup(rawLocation string) (geocode.Coordinate, bool) {
	coordinates := map[string]geocode.Coordinate{
		"amsterdam-netherlands":  {Latitude: 52.3676, Longitude: 4.9041},
		"berlin-germany":         {Latitude: 52.52, Longitude: 13.405},
		"london-uk":              {Latitude: 51.5074, Longitude: -0.1278},
		"seattle-washington-usa": {Latitude: 47.6062, Longitude: -122.3321},
		"tokyo-japan":            {Latitude: 35.6762, Longitude: 139.6503},
	}
	coordinate, ok := coordinates[geocode.NormalizeLocationKey(rawLocation)]
	return coordinate, ok
}
