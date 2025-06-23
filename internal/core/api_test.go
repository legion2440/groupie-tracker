package core

import "testing"

// TestFetchRelations проверяет, что парсер /relation
// возвращает непустой срез и валидные ID > 0.
func TestFetchRelations(t *testing.T) {
	rels, err := FetchRelations()
	if err != nil {
		t.Fatalf("FetchRelations error: %v", err)
	}
	if len(rels) == 0 {
		t.Fatalf("expected at least 1 relation, got 0")
	}
	if rels[0].ID <= 0 {
		t.Fatalf("unexpected first ID: %d", rels[0].ID)
	}
}
