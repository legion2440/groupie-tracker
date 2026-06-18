package server

import (
	"log"
	"net/http"
	"strconv"
	"time"

	"groupie-tracker/internal/catalog"
)

const (
	fallbackCreationMin = 1950
	fallbackCreationMax = 2024
	fallbackAlbumMin    = 1960
	fallbackAlbumMax    = 2024
)

type RangeView struct {
	Min         int
	Max         int
	SelectedMin int
	SelectedMax int
}

type LocationOptionView struct {
	Value     string
	Label     string
	SearchKey string
	Selected  bool
}

type MemberOptionView struct {
	Value    string
	Label    string
	Selected bool
}

type FilterOptionsView struct {
	Creation        RangeView
	FirstAlbum      RangeView
	MemberOptions   []MemberOptionView
	LocationOptions []LocationOptionView
}

type filterOptionsResponse struct {
	CreationYear   yearRangeResponse        `json:"creation_year"`
	FirstAlbumYear yearRangeResponse        `json:"first_album_year"`
	MemberCounts   []int                    `json:"member_counts"`
	Locations      []locationOptionResponse `json:"locations"`
}

type yearRangeResponse struct {
	Min int `json:"min"`
	Max int `json:"max"`
}

type locationOptionResponse struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

func serveFilterOptions(w http.ResponseWriter, r *http.Request, loadCatalog catalogLoaderFunc) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		writeJSON(w, http.StatusMethodNotAllowed, apiErrorResponse{Error: "method not allowed"})
		return
	}

	catalogData, err := loadCatalog()
	if err != nil {
		log.Printf("load catalog for filter options: %v", err)
		writeJSON(w, http.StatusInternalServerError, apiErrorResponse{Error: "internal server error"})
		return
	}

	options := catalog.BuildFilterOptions(catalogData)
	writeJSON(w, http.StatusOK, filterOptionsResponseFromOptions(options))
}

func filterOptionsView(options catalog.FilterOptions, criteria catalog.Criteria) FilterOptionsView {
	creationMin, creationMax := yearRangeBounds(options.CreationYears, fallbackCreationMin, fallbackCreationMax)
	albumMin, albumMax := yearRangeBounds(options.FirstAlbumYears, fallbackAlbumMin, fallbackAlbumMax)

	return FilterOptionsView{
		Creation:        rangeView(creationMin, creationMax, criteria.CreationFrom, criteria.CreationTo),
		FirstAlbum:      rangeView(albumMin, albumMax, yearFromTime(criteria.FirstAlbumFrom), yearFromTime(criteria.FirstAlbumTo)),
		MemberOptions:   memberOptionViews(criteria),
		LocationOptions: locationOptionViews(options.Locations, criteria.Locations),
	}
}

func filterOptionsResponseFromOptions(options catalog.FilterOptions) filterOptionsResponse {
	creationMin, creationMax := yearRangeBounds(options.CreationYears, fallbackCreationMin, fallbackCreationMax)
	albumMin, albumMax := yearRangeBounds(options.FirstAlbumYears, fallbackAlbumMin, fallbackAlbumMax)

	return filterOptionsResponse{
		CreationYear:   yearRangeResponse{Min: creationMin, Max: creationMax},
		FirstAlbumYear: yearRangeResponse{Min: albumMin, Max: albumMax},
		MemberCounts:   fixedMemberCounts(),
		Locations:      locationOptionResponses(options.Locations),
	}
}

func yearRangeBounds(yearRange catalog.YearRange, fallbackMin int, fallbackMax int) (int, int) {
	if !yearRange.Available {
		return fallbackMin, fallbackMax
	}
	return yearRange.Min, yearRange.Max
}

func rangeView(min int, max int, selectedMin *int, selectedMax *int) RangeView {
	view := RangeView{
		Min:         min,
		Max:         max,
		SelectedMin: min,
		SelectedMax: max,
	}
	if selectedMin != nil {
		view.SelectedMin = clampInt(*selectedMin, min, max)
	}
	if selectedMax != nil {
		view.SelectedMax = clampInt(*selectedMax, min, max)
	}
	if view.SelectedMin > view.SelectedMax {
		view.SelectedMax = view.SelectedMin
	}
	return view
}

func yearFromTime(value *time.Time) *int {
	if value == nil {
		return nil
	}
	year := value.Year()
	return &year
}

func clampInt(value int, min int, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func fixedMemberCounts() []int {
	return []int{1, 2, 3, 4, 5, 6, 7, 8}
}

func memberOptionViews(criteria catalog.Criteria) []MemberOptionView {
	selectedSet := make(map[int]struct{}, len(criteria.MemberCounts))
	for _, value := range criteria.MemberCounts {
		selectedSet[value] = struct{}{}
	}

	counts := fixedMemberCounts()
	views := make([]MemberOptionView, 0, len(counts))
	for _, value := range counts {
		valueText := strconv.Itoa(value)
		label := valueText
		selected := false
		if value == 8 {
			valueText = "8+"
			label = "8+"
			selected = criteria.MinMemberCount != nil && *criteria.MinMemberCount == 8
		} else {
			_, selected = selectedSet[value]
		}
		views = append(views, MemberOptionView{
			Value:    valueText,
			Label:    label,
			Selected: selected,
		})
	}
	return views
}

func locationOptionViews(options []catalog.LocationOption, selected []string) []LocationOptionView {
	selectedSet := make(map[string]struct{}, len(selected))
	for _, value := range selected {
		key := canonicalLocationSelection(value)
		if key != "" {
			selectedSet[key] = struct{}{}
		}
	}

	views := make([]LocationOptionView, 0, len(options))
	for _, option := range options {
		_, selected := selectedSet[canonicalLocationSelection(option.Value)]
		views = append(views, LocationOptionView{
			Value:     option.Value,
			Label:     option.Label,
			SearchKey: catalog.NormalizeSearchText(option.Label),
			Selected:  selected,
		})
	}
	return views
}

func canonicalLocationSelection(value string) string {
	return catalog.NormalizeSearchText(value)
}

func locationOptionResponses(options []catalog.LocationOption) []locationOptionResponse {
	responses := make([]locationOptionResponse, 0, len(options))
	for _, option := range options {
		responses = append(responses, locationOptionResponse{
			Value: option.Value,
			Label: option.Label,
		})
	}
	return responses
}
