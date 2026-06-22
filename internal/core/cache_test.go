package core

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

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

func TestUpdateNowDoesNotPublishFailedRefresh(t *testing.T) {
	original := preserveCacheStore(t)
	store.Store(original)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/artists" {
			http.Error(w, "upstream failure", http.StatusInternalServerError)
			return
		}
		writeCacheTestAPIResponse(w, r)
	}))
	defer server.Close()
	setAPIURLsForTest(t, server.URL)

	if err := UpdateNow(context.Background()); err == nil {
		t.Fatal("expected refresh error")
	}

	got, ok := store.Load().(cacheData)
	if !ok {
		t.Fatal("cache store is empty")
	}
	if !reflect.DeepEqual(got, original) {
		t.Fatalf("failed refresh changed cache: got %#v want %#v", got, original)
	}
}

func TestUpdateNowCancellationDoesNotPublishPartialRefresh(t *testing.T) {
	original := preserveCacheStore(t)
	store.Store(original)

	requestStarted := make(chan struct{}, 4)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestStarted <- struct{}{}
		<-r.Context().Done()
	}))
	defer server.Close()
	setAPIURLsForTest(t, server.URL)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- UpdateNow(ctx)
	}()

	for i := 0; i < 4; i++ {
		select {
		case <-requestStarted:
		case <-time.After(time.Second):
			t.Fatalf("only %d of 4 parallel API requests started", i)
		}
	}
	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context cancellation, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("refresh did not stop after context cancellation")
	}

	got, ok := store.Load().(cacheData)
	if !ok {
		t.Fatal("cache store is empty")
	}
	if !reflect.DeepEqual(got, original) {
		t.Fatalf("canceled refresh changed cache: got %#v want %#v", got, original)
	}
}

func preserveCacheStore(t *testing.T) cacheData {
	t.Helper()

	previous := store.Load()
	t.Cleanup(func() {
		if previous == nil {
			store.Store(cacheData{})
			return
		}
		store.Store(previous.(cacheData))
	})

	return cacheData{
		Artists:   []model.Artist{{ID: 99, Name: "Cached Artist"}},
		Relations: []model.Relation{{ID: 99}},
		UpdatedAt: time.Unix(123, 0),
	}
}

func writeCacheTestAPIResponse(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch r.URL.Path {
	case "/api/artists":
		_, _ = io.WriteString(w, `[{"id":1,"name":"Fresh Artist"}]`)
	case "/api/locations":
		_, _ = io.WriteString(w, `{"locations":[{"id":1,"locations":[]}]}`)
	case "/api/dates":
		_, _ = io.WriteString(w, `{"dates":[{"id":1,"dates":[]}]}`)
	case "/api/relation":
		_, _ = io.WriteString(w, `{"index":[{"id":1,"datesLocations":{}}]}`)
	default:
		http.NotFound(w, r)
	}
}
