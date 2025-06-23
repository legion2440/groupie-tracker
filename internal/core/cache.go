package core

import (
	"context"
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

// ----- инициализация и фоновое обновление -----

// StartCache запускает воркер; вызывает refresh() сразу и потом каждые interval.
func StartCache(ctx context.Context, interval time.Duration) {
	once.Do(func() {
		refresh() // первичное заполнение
		go func() {
			ticker := time.NewTicker(interval)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					refresh()
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

func refresh() {
	var cd cacheData
	if arts, err := FetchArtists(); err == nil {
		cd.Artists = arts
	}
	if rel, err := FetchRelations(); err == nil {
		cd.Relations = rel
	}
	if loc, err := FetchLocations(); err == nil {
		cd.Locations = loc
	}
	if d, err := FetchDates(); err == nil {
		cd.Dates = d
	}
	cd.UpdatedAt = time.Now()
	store.Store(cd)
	log.Println("cache refreshed at", cd.UpdatedAt.Format(time.RFC822))
}

func refreshField(fn func(*cacheData)) {
	if v, ok := store.Load().(cacheData); ok {
		fn(&v)
		store.Store(v)
	}
}
