package server

import (
	"fmt"

	"groupie-tracker/internal/catalog"
	"groupie-tracker/internal/core"
)

type catalogLoaderFunc func() (catalog.Catalog, error)

func loadCatalog() (catalog.Catalog, error) {
	snapshot, err := core.GetArtistRelationsSnapshot()
	if err != nil {
		return catalog.Catalog{}, fmt.Errorf("get artist-relations snapshot: %w", err)
	}

	result, err := catalog.Build(snapshot.Artists, snapshot.Relations)
	if err != nil {
		return catalog.Catalog{}, fmt.Errorf("build catalog: %w", err)
	}

	return result, nil
}
