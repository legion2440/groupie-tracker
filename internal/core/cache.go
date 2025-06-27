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
