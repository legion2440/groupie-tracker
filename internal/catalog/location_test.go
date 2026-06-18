package catalog

import (
	"reflect"
	"testing"
)

func TestNormalizeText(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "trims and collapses spaces",
			in:   "  Phil   COLLINS ",
			want: "phil collins",
		},
		{
			name: "unicode lowercase",
			in:   "  ПрИвЕт   МИР ",
			want: "привет мир",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeText(tt.in); got != tt.want {
				t.Fatalf("normalizeText(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestParseLocationFormatting(t *testing.T) {
	tests := []struct {
		raw              string
		wantDisplay      string
		wantNormalized   string
		wantHierarchy    []string
		requiredContains string
	}{
		{
			raw:            "los_angeles-usa",
			wantDisplay:    "Los Angeles, USA",
			wantNormalized: "los angeles, usa",
			wantHierarchy:  []string{"los angeles, usa", "usa"},
		},
		{
			raw:              "seattle-washington-usa",
			wantDisplay:      "Seattle, Washington, USA",
			wantNormalized:   "seattle, washington, usa",
			wantHierarchy:    []string{"seattle, washington, usa", "washington, usa", "usa"},
			requiredContains: "washington, usa",
		},
		{
			raw:            "london-uk",
			wantDisplay:    "London, UK",
			wantNormalized: "london, uk",
			wantHierarchy:  []string{"london, uk", "uk"},
		},
		{
			raw:            "new_york-u.s.a",
			wantDisplay:    "New York, U.S.A",
			wantNormalized: "new york, u.s.a",
			wantHierarchy:  []string{"new york, u.s.a", "u.s.a"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			got := parseLocation(tt.raw)
			if got.Raw != tt.raw {
				t.Fatalf("raw mismatch: got %q want %q", got.Raw, tt.raw)
			}
			if got.Display != tt.wantDisplay {
				t.Fatalf("display mismatch: got %q want %q", got.Display, tt.wantDisplay)
			}
			if got.Normalized != tt.wantNormalized {
				t.Fatalf("normalized mismatch: got %q want %q", got.Normalized, tt.wantNormalized)
			}
			if !reflect.DeepEqual(got.Hierarchy, tt.wantHierarchy) {
				t.Fatalf("hierarchy mismatch: got %v want %v", got.Hierarchy, tt.wantHierarchy)
			}
			if tt.requiredContains != "" && !containsString(got.Hierarchy, tt.requiredContains) {
				t.Fatalf("hierarchy %v does not contain %q", got.Hierarchy, tt.requiredContains)
			}
		})
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
