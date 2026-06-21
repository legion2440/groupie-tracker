package geocode

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

const cacheFileVersion = 1

type Coordinate struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type Entry struct {
	Key       string  `json:"key"`
	Location  string  `json:"location,omitempty"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Source    string  `json:"source,omitempty"`
	Match     string  `json:"match,omitempty"`
}

type Store struct {
	path    string
	entries map[string]Entry
}

type cacheFile struct {
	Version   int     `json:"version"`
	Locations []Entry `json:"locations"`
}

func DefaultCachePath() string {
	if path := os.Getenv("GEOCODING_CACHE_PATH"); path != "" {
		return path
	}
	if cwd, err := os.Getwd(); err == nil {
		for dir := cwd; ; dir = filepath.Dir(dir) {
			if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
				return filepath.Join(dir, "data", "geocoding-cache.json")
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
		}
	}
	return filepath.Join("data", "geocoding-cache.json")
}

func LoadStore(path string) (*Store, error) {
	if path == "" {
		path = DefaultCachePath()
	}
	store := &Store{
		path:    path,
		entries: make(map[string]Entry),
	}

	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return store, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read geocoding cache: %w", err)
	}
	if len(data) == 0 {
		return store, nil
	}

	var file cacheFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("decode geocoding cache: %w", err)
	}
	if file.Version != cacheFileVersion {
		return nil, fmt.Errorf("unsupported geocoding cache version %d", file.Version)
	}

	for _, entry := range file.Locations {
		if err := store.Set(entry); err != nil {
			return nil, err
		}
	}
	return store, nil
}

func (s *Store) Path() string {
	if s == nil {
		return ""
	}
	return s.path
}

func (s *Store) Lookup(rawLocation string) (Coordinate, bool) {
	return s.LookupKey(NormalizeLocationKey(rawLocation))
}

func (s *Store) LookupKey(key string) (Coordinate, bool) {
	if s == nil {
		return Coordinate{}, false
	}
	entry, ok := s.entries[NormalizeLocationKey(key)]
	if !ok {
		return Coordinate{}, false
	}
	return Coordinate{
		Latitude:  entry.Latitude,
		Longitude: entry.Longitude,
	}, true
}

func (s *Store) Set(entry Entry) error {
	if s == nil {
		return fmt.Errorf("geocoding store is nil")
	}
	key := NormalizeLocationKey(entry.Key)
	if key == "" {
		return fmt.Errorf("invalid geocoding entry with empty key")
	}
	if !validLatitude(entry.Latitude) || !validLongitude(entry.Longitude) {
		return fmt.Errorf("invalid coordinates for %s: latitude=%f longitude=%f", key, entry.Latitude, entry.Longitude)
	}
	if existing, exists := s.entries[key]; exists && existing.Key != entry.Key {
		return fmt.Errorf("duplicate geocoding key %s", key)
	}
	entry.Key = key
	if entry.Location == "" {
		entry.Location = DisplayLocation(key)
	}
	s.entries[key] = entry
	return nil
}

func (s *Store) Entries() []Entry {
	if s == nil {
		return nil
	}
	entries := make([]Entry, 0, len(s.entries))
	for _, entry := range s.entries {
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Key < entries[j].Key
	})
	return entries
}

func (s *Store) Save() error {
	if s == nil {
		return fmt.Errorf("geocoding store is nil")
	}
	if s.path == "" {
		s.path = DefaultCachePath()
	}
	file := cacheFile{
		Version:   cacheFileVersion,
		Locations: s.Entries(),
	}
	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return fmt.Errorf("encode geocoding cache: %w", err)
	}
	data = append(data, '\n')

	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create geocoding cache directory: %w", err)
	}
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(s.path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create geocoding cache temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write geocoding cache temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close geocoding cache temp file: %w", err)
	}
	if err := replaceFile(tmpName, s.path); err != nil {
		return fmt.Errorf("replace geocoding cache: %w", err)
	}
	return nil
}

func replaceFile(tmpName string, target string) error {
	if err := os.Rename(tmpName, target); err == nil {
		return nil
	}

	backup := target + ".bak"
	_ = os.Remove(backup)
	targetExists := false
	if _, err := os.Stat(target); err == nil {
		targetExists = true
		if err := os.Rename(target, backup); err != nil {
			return err
		}
	}
	if err := os.Rename(tmpName, target); err != nil {
		if targetExists {
			_ = os.Rename(backup, target)
		}
		return err
	}
	if targetExists {
		_ = os.Remove(backup)
	}
	return nil
}

func validLatitude(latitude float64) bool {
	return latitude >= -90 && latitude <= 90
}

func validLongitude(longitude float64) bool {
	return longitude >= -180 && longitude <= 180
}
