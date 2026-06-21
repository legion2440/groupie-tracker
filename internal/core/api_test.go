package core

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestFetchRelations проверяет парсер /relation без live Groupie Tracker API.
func TestFetchRelations(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/relation" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{
			"index": [
				{
					"id": 1,
					"datesLocations": {
						"seattle-washington-usa": ["10-01-1995"]
					}
				}
			]
		}`)
	}))
	defer server.Close()

	originalRelationsURL := relationsURL
	relationsURL = server.URL + "/api/relation"
	t.Cleanup(func() {
		relationsURL = originalRelationsURL
	})

	rels, err := FetchRelations()
	if err != nil {
		t.Fatalf("FetchRelations error: %v", err)
	}
	if len(rels) != 1 {
		t.Fatalf("expected 1 relation, got %d", len(rels))
	}
	if rels[0].ID != 1 {
		t.Fatalf("unexpected first ID: %d", rels[0].ID)
	}
	if got := rels[0].DatesLocations["seattle-washington-usa"]; len(got) != 1 || got[0] != "10-01-1995" {
		t.Fatalf("unexpected datesLocations: %#v", rels[0].DatesLocations)
	}
}
