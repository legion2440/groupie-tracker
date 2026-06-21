package core

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"groupie-tracker/internal/model"
)

// ----- структура кеша -----

type cacheData struct {
	Artists   []model.Artist
	Relations []model.Relation
	Locations []model.Location
	Dates     []model.Date
	UpdatedAt time.Time
}

type ArtistRelationsSnapshot struct {
	Artists   []model.Artist
	Relations []model.Relation
}

var store atomic.Value // хранит cacheData
var once sync.Once

// ----- публичные геттеры -----

func GetArtists() ([]model.Artist, error) {
	if c, ok := store.Load().(cacheData); ok && len(c.Artists) > 0 {
		return c.Artists, nil
	}
	// первый запуск или ошибка – тянем напрямую
	arts, err := FetchArtists()
	if err == nil {
		refreshField(func(c *cacheData) { c.Artists = arts })
	}
	return arts, err
}

func GetRelations() ([]model.Relation, error) {
	if c, ok := store.Load().(cacheData); ok && len(c.Relations) > 0 {
		return c.Relations, nil
	}
	rel, err := FetchRelations()
	if err == nil {
		refreshField(func(c *cacheData) { c.Relations = rel })
	}
	return rel, err
}

func GetArtistRelationsSnapshot() (ArtistRelationsSnapshot, error) {
	if c, ok := store.Load().(cacheData); ok {
		if snapshot, complete := artistRelationsSnapshotFromCache(c); complete {
			return snapshot, nil
		}
	}

	if err := UpdateNow(); err != nil {
		return ArtistRelationsSnapshot{}, fmt.Errorf("refresh artist relations snapshot: %w", err)
	}

	c, ok := store.Load().(cacheData)
	if !ok {
		return ArtistRelationsSnapshot{}, fmt.Errorf("artist relations snapshot unavailable after refresh")
	}
	snapshot, complete := artistRelationsSnapshotFromCache(c)
	if !complete {
		return ArtistRelationsSnapshot{}, fmt.Errorf(
			"artist relations snapshot incomplete after refresh: artists=%d relations=%d",
			len(c.Artists),
			len(c.Relations),
		)
	}
	return snapshot, nil
}

func artistRelationsSnapshotFromCache(c cacheData) (ArtistRelationsSnapshot, bool) {
	if len(c.Artists) == 0 || len(c.Relations) == 0 {
		return ArtistRelationsSnapshot{}, false
	}
	return ArtistRelationsSnapshot{
		Artists:   cloneArtists(c.Artists),
		Relations: cloneRelations(c.Relations),
	}, true
}

func cloneArtists(artists []model.Artist) []model.Artist {
	cloned := make([]model.Artist, len(artists))
	for i, artist := range artists {
		cloned[i] = artist
		cloned[i].Members = append([]string(nil), artist.Members...)
	}
	return cloned
}

func cloneRelations(relations []model.Relation) []model.Relation {
	cloned := make([]model.Relation, len(relations))
	for i, relation := range relations {
		cloned[i] = relation
		cloned[i].DatesLocations = cloneDatesLocations(relation.DatesLocations)
	}
	return cloned
}

func cloneDatesLocations(src map[string][]string) map[string][]string {
	if src == nil {
		return nil
	}
	cloned := make(map[string][]string, len(src))
	for location, dates := range src {
		cloned[location] = append([]string(nil), dates...)
	}
	return cloned
}

// ----- инициализация и фоновое обновление -----

// StartCache запускает воркер; вызывает refresh() сразу и потом каждые interval.
func StartCache(ctx context.Context, interval time.Duration) {
	once.Do(func() {
		if err := refresh(); err != nil {
			log.Printf("initial cache refresh failed: %v", err)
		}
		go func() {
			ticker := time.NewTicker(interval)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					if err := refresh(); err != nil {
						log.Printf("cache refresh failed: %v", err)
					}
				case <-ctx.Done():
					return
				}
			}
		}()
	})
}

// ForceUpdate вручную перезапускает обновление (используем в /api/refresh).
func ForceUpdate() { go refresh() }

// ----- внутреннее -----

func refresh() error {
	var (
		cd   cacheData
		mu   sync.Mutex
		wg   sync.WaitGroup
		errs = make(chan error, 4) // буфер на все горутины
	)

	// helper-wrapper: сохраняет данные или шлёт ошибку
	add := func(setter func(), err error) {
		if err != nil {
			errs <- err
			return
		}
		mu.Lock()
		setter()
		mu.Unlock()
	}

	wg.Add(4)

	go func() {
		defer wg.Done()
		arts, err := FetchArtists()
		add(func() { cd.Artists = arts }, err)
	}()
	go func() {
		defer wg.Done()
		rel, err := FetchRelations()
		add(func() { cd.Relations = rel }, err)
	}()
	go func() {
		defer wg.Done()
		loc, err := FetchLocations()
		add(func() { cd.Locations = loc }, err)
	}()
	go func() {
		defer wg.Done()
		d, err := FetchDates()
		add(func() { cd.Dates = d }, err)
	}()

	wg.Wait()
	close(errs)

	if err, failed := <-errs; failed {
		return err // хотя бы один запрос упал
	}

	if len(cd.Relations) > 0 {
		report, err := EnsureGeocodingCoverage(cd.Relations)
		if err != nil {
			return fmt.Errorf("geocoding coverage: %w", err)
		}
		log.Printf(
			"geocoding coverage: total=%d cache=%d automatic=%d fuzzy=%d missing=%d",
			report.Total,
			report.FromCache,
			report.AutoFound,
			report.FuzzyFound,
			report.Missing,
		)
	}

	cd.UpdatedAt = time.Now()
	store.Store(cd)
	log.Println("cache refreshed at", cd.UpdatedAt.Format(time.RFC822))
	return nil
}

func refreshField(fn func(*cacheData)) {
	if v, ok := store.Load().(cacheData); ok {
		fn(&v)
		store.Store(v)
	}
}

// UpdateNow скачивает данные синхронно.
// Если хотя бы один запрос упал — кеш не трогаем, возвращаем ошибку.
func UpdateNow() error {
	if err := refresh(); err != nil {
		return err
	}
	return nil
}
