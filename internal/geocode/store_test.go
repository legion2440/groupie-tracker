package geocode

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadStoreReadsJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "geocoding-cache.json")
	data := `{
  "version": 1,
  "locations": [
    {
      "key": "london-uk",
      "location": "London, UK",
      "latitude": 51.5074,
      "longitude": -0.1278
    }
  ]
}
`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	store, err := LoadStore(path)
	if err != nil {
		t.Fatalf("LoadStore returned error: %v", err)
	}
	coordinate, ok := store.Lookup("london-uk")
	if !ok {
		t.Fatal("expected london-uk lookup")
	}
	if coordinate.Latitude != 51.5074 || coordinate.Longitude != -0.1278 {
		t.Fatalf("coordinate mismatch: %#v", coordinate)
	}
}

func TestStoreSaveWritesDeterministicJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "geocoding-cache.json")
	store, err := LoadStore(path)
	if err != nil {
		t.Fatalf("LoadStore returned error: %v", err)
	}
	if err := store.Set(Entry{Key: "zurich-switzerland", Location: "Zurich, Switzerland", Latitude: 47.3769, Longitude: 8.5417}); err != nil {
		t.Fatalf("set zurich: %v", err)
	}
	if err := store.Set(Entry{Key: "amsterdam-netherlands", Location: "Amsterdam, Netherlands", Latitude: 52.3676, Longitude: 4.9041}); err != nil {
		t.Fatalf("set amsterdam: %v", err)
	}
	if err := store.Save(); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read saved cache: %v", err)
	}
	text := string(data)
	if strings.Index(text, `"key": "amsterdam-netherlands"`) > strings.Index(text, `"key": "zurich-switzerland"`) {
		t.Fatalf("entries are not sorted deterministically: %s", text)
	}
	if !strings.HasSuffix(text, "\n") {
		t.Fatalf("cache should end with newline: %q", text)
	}
}

func TestLoadStoreRejectsCorruptedJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "geocoding-cache.json")
	if err := os.WriteFile(path, []byte(`{"version": 1,`), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	if _, err := LoadStore(path); err == nil {
		t.Fatal("expected corrupted JSON error")
	}
}

func TestLoadStoreRejectsInvalidEntry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "geocoding-cache.json")
	data := `{
  "version": 1,
  "locations": [
    {
      "key": "bad-location",
      "latitude": 120,
      "longitude": 8
    }
  ]
}
`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	if _, err := LoadStore(path); err == nil {
		t.Fatal("expected invalid coordinate error")
	}
}
