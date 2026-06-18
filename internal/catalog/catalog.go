package catalog

import (
	"fmt"
	"strconv"
	"time"

	"groupie-tracker/internal/model"
)

const firstAlbumLayout = "02-01-2006"

type Catalog struct {
	Artists        []ArtistEntry
	ArtistByID     map[int]ArtistEntry
	ArtistIDBySlug map[string]int
	ArtistSlugByID map[int]string
}

type ArtistEntry struct {
	ID           int
	Slug         string
	Name         string
	Image        string
	Members      []string
	CreationYear int

	FirstAlbumRaw string
	FirstAlbum    time.Time

	Locations []Location

	NormalizedName       string
	NormalizedMembers    []string
	NormalizedFirstAlbum string
	CreationYearText     string
}

type Location struct {
	Raw        string
	Display    string
	Normalized string
	Hierarchy  []string
}

func Build(artists []model.Artist, relations []model.Relation) (Catalog, error) {
	locationsByArtistID := collectRelationLocations(relations)
	catalog := Catalog{
		Artists:        make([]ArtistEntry, 0, len(artists)),
		ArtistByID:     make(map[int]ArtistEntry, len(artists)),
		ArtistIDBySlug: make(map[string]int, len(artists)),
		ArtistSlugByID: make(map[int]string, len(artists)),
	}
	baseSlugsByArtistID, slugCounts := collectArtistSlugBases(artists)

	for _, artist := range artists {
		firstAlbum, err := parseFirstAlbum(artist.FirstAlbum)
		if err != nil {
			return Catalog{}, fmt.Errorf(
				"parse first album for artist %d %q: %w",
				artist.ID,
				artist.Name,
				err,
			)
		}

		slug := baseSlugsByArtistID[artist.ID]
		if slugCounts[slug] > 1 {
			slug = fmt.Sprintf("%s-%d", slug, artist.ID)
		}

		entry := ArtistEntry{
			ID:                   artist.ID,
			Slug:                 slug,
			Name:                 artist.Name,
			Image:                artist.Image,
			Members:              append([]string(nil), artist.Members...),
			CreationYear:         artist.CreationDate,
			FirstAlbumRaw:        artist.FirstAlbum,
			FirstAlbum:           firstAlbum,
			Locations:            buildLocations(locationsByArtistID[artist.ID]),
			NormalizedName:       normalizeText(artist.Name),
			NormalizedMembers:    normalizeMembers(artist.Members),
			NormalizedFirstAlbum: normalizeText(artist.FirstAlbum),
			CreationYearText:     strconv.Itoa(artist.CreationDate),
		}
		catalog.Artists = append(catalog.Artists, entry)
		catalog.ArtistByID[artist.ID] = entry
		catalog.ArtistIDBySlug[slug] = artist.ID
		catalog.ArtistSlugByID[artist.ID] = slug
	}

	return catalog, nil
}

func collectArtistSlugBases(artists []model.Artist) (map[int]string, map[string]int) {
	baseSlugsByArtistID := make(map[int]string, len(artists))
	slugCounts := make(map[string]int, len(artists))
	for _, artist := range artists {
		slug := ArtistSlug(artist.Name)
		if slug == "" {
			slug = "artist"
		}
		baseSlugsByArtistID[artist.ID] = slug
		slugCounts[slug]++
	}
	return baseSlugsByArtistID, slugCounts
}

func collectRelationLocations(relations []model.Relation) map[int][]string {
	locationsByArtistID := make(map[int][]string)
	for _, relation := range relations {
		for rawLocation := range relation.DatesLocations {
			locationsByArtistID[relation.ID] = append(locationsByArtistID[relation.ID], rawLocation)
		}
	}
	return locationsByArtistID
}

func parseFirstAlbum(raw string) (time.Time, error) {
	if raw == "" {
		return time.Time{}, nil
	}
	firstAlbum, err := time.ParseInLocation(firstAlbumLayout, raw, time.UTC)
	if err != nil {
		return time.Time{}, err
	}
	return firstAlbum, nil
}

func normalizeMembers(members []string) []string {
	normalized := make([]string, len(members))
	for i, member := range members {
		normalized[i] = normalizeText(member)
	}
	return normalized
}
