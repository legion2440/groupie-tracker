package catalog

import "testing"

func TestBoundedLevenshtein(t *testing.T) {
	tests := []struct {
		name        string
		left        string
		right       string
		limit       int
		want        int
		wantMatched bool
	}{
		{name: "equal", left: "queen", right: "queen", limit: 1, want: 0, wantMatched: true},
		{name: "one substitution", left: "queen", right: "qaeen", limit: 1, want: 1, wantMatched: true},
		{name: "one insertion", left: "queen", right: "queeen", limit: 1, want: 1, wantMatched: true},
		{name: "one deletion", left: "queen", right: "qeen", limit: 1, want: 1, wantMatched: true},
		{name: "multiple edits", left: "pink floyd", right: "pink fluid", limit: 2, want: 2, wantMatched: true},
		{name: "over limit", left: "queen", right: "abcdn", limit: 2, wantMatched: false},
		{name: "length difference rejection", left: "queen", right: "queenxxxx", limit: 2, wantMatched: false},
		{name: "unicode substitution", left: "zoë", right: "zoe", limit: 1, want: 1, wantMatched: true},
		{name: "empty equal", left: "", right: "", limit: 0, want: 0, wantMatched: true},
		{name: "empty insertion", left: "", right: "a", limit: 1, want: 1, wantMatched: true},
		{name: "empty over limit", left: "", right: "ab", limit: 1, wantMatched: false},
		{name: "no transposition shortcut", left: "ab", right: "ba", limit: 1, wantMatched: false},
		{name: "transposition standard distance", left: "ab", right: "ba", limit: 2, want: 2, wantMatched: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, matched := boundedLevenshtein(tt.left, tt.right, tt.limit)
			if matched != tt.wantMatched {
				t.Fatalf("matched mismatch: got %v want %v (distance %d)", matched, tt.wantMatched, got)
			}
			if matched && got != tt.want {
				t.Fatalf("distance mismatch: got %d want %d", got, tt.want)
			}
		})
	}
}

func TestFuzzyDistanceLimit(t *testing.T) {
	tests := []struct {
		query string
		want  int
	}{
		{query: "", want: 0},
		{query: "qen", want: 0},
		{query: "1971", want: 0},
		{query: "05 08 1968", want: 0},
		{query: "qeen", want: 1},
		{query: "queeen", want: 1},
		{query: "pink floid", want: 2},
		{query: "thirteenchars", want: 3},
	}

	for _, tt := range tests {
		if got := fuzzyDistanceLimit(tt.query); got != tt.want {
			t.Fatalf("fuzzyDistanceLimit(%q) = %d, want %d", tt.query, got, tt.want)
		}
	}
}

func TestFuzzyMatchDistance(t *testing.T) {
	tests := []struct {
		name        string
		query       string
		candidate   string
		limit       int
		want        int
		wantMatched bool
	}{
		{name: "single token against candidate token", query: "londn", candidate: "london uk", limit: 1, want: 1, wantMatched: true},
		{name: "multi token contiguous window", query: "phil colins", candidate: "phil collins band", limit: 2, want: 1, wantMatched: true},
		{name: "no token reordering", query: "colins phil", candidate: "phil collins", limit: 2, wantMatched: false},
		{name: "full phrase comparison", query: "pink floid", candidate: "pink floyd", limit: 2, want: 1, wantMatched: true},
		{name: "numeric query disabled", query: "1971", candidate: "1970", limit: 1, wantMatched: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, matched := fuzzyMatchDistance(tt.query, searchQueryTokens(tt.query), tt.candidate, tt.limit)
			if matched != tt.wantMatched {
				t.Fatalf("matched mismatch: got %v want %v (distance %d)", matched, tt.wantMatched, got)
			}
			if matched && got != tt.want {
				t.Fatalf("distance mismatch: got %d want %d", got, tt.want)
			}
		})
	}
}

func TestSearchTextFuzzyDistance(t *testing.T) {
	tests := []struct {
		name        string
		query       string
		candidate   string
		want        int
		wantMatched bool
	}{
		{name: "punctuated artist name", query: "ACDC", candidate: "AC/DC", want: 1, wantMatched: true},
		{name: "plural typo", query: "Bobby McFerrins", candidate: "Bobby McFerrin", want: 1, wantMatched: true},
		{name: "possessive typo", query: "Bobby McFerrin's", candidate: "Bobby McFerrin", want: 2, wantMatched: true},
		{name: "reject longer different artist", query: "Echo Lane", candidate: "Echo Lane Extended", wantMatched: false},
		{name: "short acronym stays strict", query: "NWA", candidate: "N.W.A", wantMatched: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, matched := SearchTextFuzzyDistance(tt.query, tt.candidate)
			if matched != tt.wantMatched {
				t.Fatalf("matched mismatch: got %v want %v (distance %d)", matched, tt.wantMatched, got)
			}
			if matched && got != tt.want {
				t.Fatalf("distance mismatch: got %d want %d", got, tt.want)
			}
		})
	}
}
