package geocode

import "testing"

func TestNormalizeLocationKeyAndNominatimQuery(t *testing.T) {
	tests := []struct {
		raw       string
		wantKey   string
		wantQuery string
		wantCity  string
	}{
		{
			raw:       "los_angeles-usa",
			wantKey:   "los_angeles-usa",
			wantQuery: "Los Angeles, United States",
			wantCity:  "Los Angeles",
		},
		{
			raw:       "seattle-washington-usa",
			wantKey:   "seattle-washington-usa",
			wantQuery: "Seattle, Washington, United States",
			wantCity:  "Seattle",
		},
		{
			raw:       "london-uk",
			wantKey:   "london-uk",
			wantQuery: "London, United Kingdom",
			wantCity:  "London",
		},
		{
			raw:       "abu_dhabi-uae",
			wantKey:   "abu_dhabi-uae",
			wantQuery: "Abu Dhabi, United Arab Emirates",
			wantCity:  "Abu Dhabi",
		},
	}

	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			spec := ParseLocation(tt.raw)
			if spec.Key != tt.wantKey {
				t.Fatalf("key = %q, want %q", spec.Key, tt.wantKey)
			}
			if spec.Query != tt.wantQuery {
				t.Fatalf("query = %q, want %q", spec.Query, tt.wantQuery)
			}
			if spec.City != tt.wantCity {
				t.Fatalf("city = %q, want %q", spec.City, tt.wantCity)
			}
		})
	}
}
