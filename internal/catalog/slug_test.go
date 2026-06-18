package catalog

import (
	"reflect"
	"testing"

	"groupie-tracker/internal/model"
)

func TestArtistSlug(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{name: "Queen", want: "queen"},
		{name: "Pink Floyd", want: "pink-floyd"},
		{name: "AC/DC", want: "ac-dc"},
		{name: "  AC---DC  ", want: "ac-dc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ArtistSlug(tt.name); got != tt.want {
				t.Fatalf("ArtistSlug(%q) = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}

func TestArtistSlugStable(t *testing.T) {
	name := "Pink/Floyd"
	first := ArtistSlug(name)
	for i := 0; i < 10; i++ {
		if got := ArtistSlug(name); got != first {
			t.Fatalf("slug changed between calls: got %q want %q", got, first)
		}
	}
}

func TestBuildArtistSlugs(t *testing.T) {
	cat, err := Build([]model.Artist{
		{ID: 1, Name: "Queen", FirstAlbum: "01-01-1973"},
		{ID: 2, Name: "Pink Floyd", FirstAlbum: "01-01-1967"},
		{ID: 3, Name: "AC/DC", FirstAlbum: "01-01-1975"},
	}, nil)
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}

	got := []string{cat.Artists[0].Slug, cat.Artists[1].Slug, cat.Artists[2].Slug}
	want := []string{"queen", "pink-floyd", "ac-dc"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("slugs mismatch: got %v want %v", got, want)
	}
	for id, slug := range map[int]string{1: "queen", 2: "pink-floyd", 3: "ac-dc"} {
		if got := cat.ArtistSlugByID[id]; got != slug {
			t.Fatalf("ArtistSlugByID[%d] = %q, want %q", id, got, slug)
		}
		if got := cat.ArtistIDBySlug[slug]; got != id {
			t.Fatalf("ArtistIDBySlug[%q] = %d, want %d", slug, got, id)
		}
	}
}

func TestBuildArtistSlugCollisionUsesStableIDSuffix(t *testing.T) {
	cat, err := Build([]model.Artist{
		{ID: 12, Name: "Same Name", FirstAlbum: "01-01-2000"},
		{ID: 7, Name: "Same/Name", FirstAlbum: "01-01-2001"},
		{ID: 20, Name: "Unique Name", FirstAlbum: "01-01-2002"},
	}, nil)
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}

	got := []string{cat.Artists[0].Slug, cat.Artists[1].Slug, cat.Artists[2].Slug}
	want := []string{"same-name-12", "same-name-7", "unique-name"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("collision slugs mismatch: got %v want %v", got, want)
	}
	for slug, id := range map[string]int{"same-name-12": 12, "same-name-7": 7, "unique-name": 20} {
		if got := cat.ArtistIDBySlug[slug]; got != id {
			t.Fatalf("ArtistIDBySlug[%q] = %d, want %d", slug, got, id)
		}
	}
	if _, exists := cat.ArtistIDBySlug["same-name"]; exists {
		t.Fatalf("colliding base slug must not be routable without ID suffix")
	}
}
