package catalog

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"groupie-tracker/internal/model"
)

func TestBuildBasic(t *testing.T) {
	artists := []model.Artist{
		{
			ID:           7,
			Name:         "Phil Collins",
			Image:        "phil.jpeg",
			Members:      []string{"Phil Collins", " Chester   Thompson "},
			CreationDate: 1981,
			FirstAlbum:   "05-02-1981",
		},
	}

	catalog, err := Build(artists, nil)
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}
	if len(catalog.Artists) != 1 {
		t.Fatalf("expected 1 artist, got %d", len(catalog.Artists))
	}

	got := catalog.Artists[0]
	if got.ID != 7 || got.Name != "Phil Collins" || got.Image != "phil.jpeg" {
		t.Fatalf("artist fields mismatch: %#v", got)
	}
	if !reflect.DeepEqual(got.Members, artists[0].Members) {
		t.Fatalf("members mismatch: got %v want %v", got.Members, artists[0].Members)
	}
	if got.CreationYear != 1981 {
		t.Fatalf("creation year mismatch: got %d", got.CreationYear)
	}
	if got.FirstAlbumRaw != "05-02-1981" {
		t.Fatalf("first album raw mismatch: got %q", got.FirstAlbumRaw)
	}
	wantDate := time.Date(1981, 2, 5, 0, 0, 0, 0, time.UTC)
	if !got.FirstAlbum.Equal(wantDate) {
		t.Fatalf("first album mismatch: got %v want %v", got.FirstAlbum, wantDate)
	}
	if got.NormalizedName != "phil collins" {
		t.Fatalf("normalized name mismatch: %q", got.NormalizedName)
	}
	if want := []string{"phil collins", "chester thompson"}; !reflect.DeepEqual(got.NormalizedMembers, want) {
		t.Fatalf("normalized members mismatch: got %v want %v", got.NormalizedMembers, want)
	}
	if got.NormalizedFirstAlbum != "05-02-1981" {
		t.Fatalf("normalized first album mismatch: %q", got.NormalizedFirstAlbum)
	}
	if got.CreationYearText != "1981" {
		t.Fatalf("creation year text mismatch: %q", got.CreationYearText)
	}
}

func TestBuildEmptyArtists(t *testing.T) {
	catalog, err := Build(nil, []model.Relation{
		{ID: 1, DatesLocations: map[string][]string{"los_angeles-usa": {"01-01-2020"}}},
	})
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}
	if len(catalog.Artists) != 0 {
		t.Fatalf("expected empty catalog, got %#v", catalog.Artists)
	}
}

func TestBuildPreservesArtistOrder(t *testing.T) {
	artists := []model.Artist{
		{ID: 20, Name: "Second", FirstAlbum: "01-01-2000"},
		{ID: 5, Name: "First", FirstAlbum: "01-01-2001"},
		{ID: 99, Name: "Third", FirstAlbum: "01-01-2002"},
	}

	catalog, err := Build(artists, nil)
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}

	got := []int{
		catalog.Artists[0].ID,
		catalog.Artists[1].ID,
		catalog.Artists[2].ID,
	}
	want := []int{20, 5, 99}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("artist order mismatch: got %v want %v", got, want)
	}
}

func TestBuildMapsRelationsByArtistID(t *testing.T) {
	artists := []model.Artist{
		{ID: 2, Name: "Two", FirstAlbum: "01-01-2000"},
		{ID: 9, Name: "Nine", FirstAlbum: "01-01-2001"},
	}
	relations := []model.Relation{
		{ID: 9, DatesLocations: map[string][]string{"seattle-washington-usa": {"01-01-2020"}}},
		{ID: 2, DatesLocations: map[string][]string{"los_angeles-usa": {"02-01-2020"}}},
	}

	catalog, err := Build(artists, relations)
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}

	if got := catalog.Artists[0].Locations[0].Normalized; got != "los angeles, usa" {
		t.Fatalf("artist 2 location mismatch: %q", got)
	}
	if got := catalog.Artists[1].Locations[0].Normalized; got != "seattle, washington, usa" {
		t.Fatalf("artist 9 location mismatch: %q", got)
	}
}

func TestBuildDeterministicLocations(t *testing.T) {
	artists := []model.Artist{{ID: 1, Name: "Artist", FirstAlbum: "01-01-2000"}}
	locationsA := make(map[string][]string)
	locationsA["seattle-washington-usa"] = []string{"01-01-2020"}
	locationsA["los_angeles-usa"] = []string{"02-01-2020"}
	locationsA["london-uk"] = []string{"03-01-2020"}

	locationsB := make(map[string][]string)
	locationsB["london-uk"] = []string{"03-01-2020"}
	locationsB["los_angeles-usa"] = []string{"02-01-2020"}
	locationsB["seattle-washington-usa"] = []string{"01-01-2020"}

	catalogA, err := Build(artists, []model.Relation{{ID: 1, DatesLocations: locationsA}})
	if err != nil {
		t.Fatalf("Build A returned error: %v", err)
	}
	catalogB, err := Build(artists, []model.Relation{{ID: 1, DatesLocations: locationsB}})
	if err != nil {
		t.Fatalf("Build B returned error: %v", err)
	}

	if !reflect.DeepEqual(catalogA.Artists[0].Locations, catalogB.Artists[0].Locations) {
		t.Fatalf("locations are not deterministic:\nA=%#v\nB=%#v", catalogA.Artists[0].Locations, catalogB.Artists[0].Locations)
	}
}

func TestBuildDeduplicatesLocations(t *testing.T) {
	artists := []model.Artist{{ID: 1, Name: "Artist", FirstAlbum: "01-01-2000"}}
	relations := []model.Relation{
		{
			ID: 1,
			DatesLocations: map[string][]string{
				"los_angeles-usa": {"01-01-2020"},
				"los_angeles-USA": {"02-01-2020"},
			},
		},
		{
			ID: 1,
			DatesLocations: map[string][]string{
				"los_angeles-usa": {"03-01-2020"},
			},
		},
	}

	catalog, err := Build(artists, relations)
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}

	locations := catalog.Artists[0].Locations
	if len(locations) != 1 {
		t.Fatalf("expected 1 deduplicated location, got %d: %#v", len(locations), locations)
	}
	if locations[0].Normalized != "los angeles, usa" {
		t.Fatalf("deduplicated location mismatch: %#v", locations[0])
	}
}

func TestBuildKeepsArtistWithMissingRelation(t *testing.T) {
	artists := []model.Artist{{ID: 1, Name: "Artist", FirstAlbum: "01-01-2000"}}

	catalog, err := Build(artists, nil)
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}
	if len(catalog.Artists) != 1 {
		t.Fatalf("expected 1 artist, got %d", len(catalog.Artists))
	}
	if len(catalog.Artists[0].Locations) != 0 {
		t.Fatalf("expected no locations, got %#v", catalog.Artists[0].Locations)
	}
}

func TestBuildIgnoresUnknownRelationArtist(t *testing.T) {
	artists := []model.Artist{{ID: 1, Name: "Artist", FirstAlbum: "01-01-2000"}}
	relations := []model.Relation{
		{ID: 999, DatesLocations: map[string][]string{"seattle-washington-usa": {"01-01-2020"}}},
	}

	catalog, err := Build(artists, relations)
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}
	if len(catalog.Artists) != 1 {
		t.Fatalf("expected 1 source artist only, got %d", len(catalog.Artists))
	}
	if len(catalog.Artists[0].Locations) != 0 {
		t.Fatalf("expected unknown relation to be ignored, got %#v", catalog.Artists[0].Locations)
	}
}

func TestBuildInvalidFirstAlbumIncludesArtistContext(t *testing.T) {
	_, err := Build([]model.Artist{
		{ID: 42, Name: "Bad Date", FirstAlbum: "not-a-date"},
	}, nil)
	if err == nil {
		t.Fatal("expected error")
	}

	msg := err.Error()
	if !strings.Contains(msg, "42") || !strings.Contains(msg, "Bad Date") {
		t.Fatalf("expected artist context in error, got %q", msg)
	}
}

func TestBuildDoesNotMutateInputs(t *testing.T) {
	artists := []model.Artist{
		{ID: 1, Name: "Artist", Members: []string{"Original Member"}, FirstAlbum: "01-01-2000"},
	}
	relations := []model.Relation{
		{
			ID: 1,
			DatesLocations: map[string][]string{
				"seattle-washington-usa": {"01-01-2020"},
			},
		},
	}

	catalog, err := Build(artists, relations)
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}

	catalog.Artists[0].Members[0] = "Changed Member"
	catalog.Artists[0].Locations[0].Hierarchy[0] = "changed hierarchy"

	if got := artists[0].Members[0]; got != "Original Member" {
		t.Fatalf("source member mutated: %q", got)
	}
	if got := relations[0].DatesLocations["seattle-washington-usa"][0]; got != "01-01-2020" {
		t.Fatalf("source relation dates mutated: %q", got)
	}
}
