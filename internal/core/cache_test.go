package core

import (
	"reflect"
	"testing"

	"groupie-tracker/internal/model"
)

func TestArtistRelationsSnapshotFromCacheClonesData(t *testing.T) {
	source := cacheData{
		Artists: []model.Artist{
			{
				ID:      20,
				Name:    "Second",
				Members: []string{"Member B", "Member C"},
			},
			{
				ID:      10,
				Name:    "First",
				Members: []string{"Member A"},
			},
		},
		Relations: []model.Relation{
			{
				ID: 20,
				DatesLocations: map[string][]string{
					"seattle-washington-usa": {"01-01-2020", "02-01-2020"},
				},
			},
			{
				ID: 10,
				DatesLocations: map[string][]string{
					"london-uk": {"03-01-2020"},
				},
			},
		},
	}

	snapshot, complete := artistRelationsSnapshotFromCache(source)
	if !complete {
		t.Fatal("expected complete snapshot")
	}

	if got, want := []int{snapshot.Artists[0].ID, snapshot.Artists[1].ID}, []int{20, 10}; !reflect.DeepEqual(got, want) {
		t.Fatalf("artist order mismatch: got %v want %v", got, want)
	}
	if got, want := []int{snapshot.Relations[0].ID, snapshot.Relations[1].ID}, []int{20, 10}; !reflect.DeepEqual(got, want) {
		t.Fatalf("relation order mismatch: got %v want %v", got, want)
	}

	snapshot.Artists[0].Members[0] = "changed member"
	snapshot.Relations[0].DatesLocations["seattle-washington-usa"][0] = "changed date"
	snapshot.Relations[0].DatesLocations["new-location"] = []string{"04-01-2020"}

	if got := source.Artists[0].Members[0]; got != "Member B" {
		t.Fatalf("source member mutated through snapshot: %q", got)
	}
	if got := source.Relations[0].DatesLocations["seattle-washington-usa"][0]; got != "01-01-2020" {
		t.Fatalf("source relation date mutated through snapshot: %q", got)
	}
	if _, exists := source.Relations[0].DatesLocations["new-location"]; exists {
		t.Fatal("source relation map mutated through snapshot")
	}
}

func TestArtistRelationsSnapshotFromCacheRequiresCompleteData(t *testing.T) {
	tests := []struct {
		name string
		data cacheData
	}{
		{
			name: "missing artists",
			data: cacheData{
				Relations: []model.Relation{{ID: 1}},
			},
		},
		{
			name: "missing relations",
			data: cacheData{
				Artists: []model.Artist{{ID: 1}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, complete := artistRelationsSnapshotFromCache(tt.data); complete {
				t.Fatal("expected incomplete snapshot")
			}
		})
	}
}
