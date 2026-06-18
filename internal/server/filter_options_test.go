package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"groupie-tracker/internal/catalog"
)

func TestFilterOptionsHandlerDerivedCatalogOptions(t *testing.T) {
	loadCalls := 0
	mux := initRoutes(testDependenciesWithCatalog(testCatalog(), &loadCalls))

	res, body := performFilterOptionsRequest(t, mux, http.MethodGet, "/api/filter-options")

	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}
	if ct := res.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("expected JSON Content-Type, got %q", ct)
	}
	if loadCalls != 1 {
		t.Fatalf("expected loader to be called once, got %d", loadCalls)
	}

	got := decodeFilterOptionsResponse(t, body)
	want := filterOptionsResponse{
		CreationYear:   yearRangeResponse{Min: 1980, Max: 2001},
		FirstAlbumYear: yearRangeResponse{Min: 1984, Max: 2003},
		MemberCounts:   []int{1, 2, 3, 4, 5, 6, 7, 8},
		Locations: []locationOptionResponse{
			{Value: "London, UK", Label: "London, UK"},
			{Value: "Los Angeles, USA", Label: "Los Angeles, USA"},
			{Value: "Seattle, Washington, USA", Label: "Seattle, Washington, USA"},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("filter options mismatch:\ngot  %#v\nwant %#v", got, want)
	}
	if strings.Contains(string(body), "null") {
		t.Fatalf("expected non-null arrays, got %s", string(body))
	}
}

func TestFilterOptionsHandlerEmptyCatalogUsesFallbackAndEmptyArrays(t *testing.T) {
	loadCalls := 0
	mux := initRoutes(testDependenciesWithCatalog(catalog.Catalog{}, &loadCalls))

	res, body := performFilterOptionsRequest(t, mux, http.MethodGet, "/api/filter-options")

	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}
	if loadCalls != 1 {
		t.Fatalf("expected loader to be called once, got %d", loadCalls)
	}
	got := decodeFilterOptionsResponse(t, body)
	want := filterOptionsResponse{
		CreationYear:   yearRangeResponse{Min: fallbackCreationMin, Max: fallbackCreationMax},
		FirstAlbumYear: yearRangeResponse{Min: fallbackAlbumMin, Max: fallbackAlbumMax},
		MemberCounts:   []int{1, 2, 3, 4, 5, 6, 7, 8},
		Locations:      []locationOptionResponse{},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("filter options mismatch:\ngot  %#v\nwant %#v", got, want)
	}
	if strings.Contains(string(body), "null") {
		t.Fatalf("expected empty arrays instead of null: %s", string(body))
	}
}

func TestFilterOptionsHandlerMethodValidation(t *testing.T) {
	loadCalls := 0
	mux := initRoutes(testDependenciesWithCatalog(testCatalog(), &loadCalls))

	res, body := performFilterOptionsRequest(t, mux, http.MethodPost, "/api/filter-options")

	if res.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", res.StatusCode)
	}
	if allow := res.Header.Get("Allow"); allow != http.MethodGet {
		t.Fatalf("Allow header mismatch: got %q", allow)
	}
	assertJSONError(t, body)
	if loadCalls != 0 {
		t.Fatalf("expected loader not to be called, got %d", loadCalls)
	}
}

func TestFilterOptionsHandlerLoaderFailure(t *testing.T) {
	loadCalls := 0
	mux := initRoutes(dependencies{
		updateNow: func() error {
			return nil
		},
		loadCatalog: func() (catalog.Catalog, error) {
			loadCalls++
			return catalog.Catalog{}, errors.New("private filter options failure")
		},
	})

	res, body := performFilterOptionsRequest(t, mux, http.MethodGet, "/api/filter-options")

	if res.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", res.StatusCode)
	}
	if loadCalls != 1 {
		t.Fatalf("expected loader to be called once, got %d", loadCalls)
	}
	assertJSONError(t, body)
	if strings.Contains(string(body), "private filter") {
		t.Fatalf("internal error leaked in response: %s", string(body))
	}
}

func TestRootPageRendersCatalogDerivedFilterRanges(t *testing.T) {
	body := renderRootBodyWithCatalog(t, "/", testCatalog())

	assertRangeInputs(t, body, "creation", RangeView{Min: 1980, Max: 2001, SelectedMin: 1980, SelectedMax: 2001})
	assertRangeInputs(t, body, "first-album", RangeView{Min: 1984, Max: 2003, SelectedMin: 1984, SelectedMax: 2003})
	for _, want := range []string{
		`data-filter-range="creation"`,
		`data-filter-range="first-album"`,
		`/static/filters.js`,
		`/static/filter-options.js`,
		`data-dual-range-output-min>1980</span>`,
		`data-dual-range-output-max>2001</span>`,
		`data-dual-range-output-min>1984</span>`,
		`data-dual-range-output-max>2003</span>`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected %q in body", want)
		}
	}
	assertTemplateDoesNotDuplicateFallbackYears(t)
}

func TestRootPageRendersFilterFormControls(t *testing.T) {
	body := renderRootBodyWithCatalog(t, "/?members=1&locations=Seattle%2C+Washington%2C+USA", testCatalog())
	form := filterFormSection(t, body)

	for _, want := range []string{
		`class="filter-form"`,
		`method="get"`,
		`action="/"`,
		`data-filter-form`,
		`<fieldset class="filter-group filter-group--range">`,
		`<legend class="filter-title">CREATION YEAR</legend>`,
		`<legend class="filter-title">FIRST ALBUM YEAR</legend>`,
		`<legend class="filter-title">NUMBER OF MEMBERS</legend>`,
		`<legend class="filter-title">LOCATIONS</legend>`,
		`data-filter-param="creation-from"`,
		`data-filter-param="creation-to"`,
		`data-filter-param="album-from"`,
		`data-filter-param="album-to"`,
		`data-member-options`,
		`data-location-options`,
		`data-location-search`,
		`aria-describedby="creation-year-min-output"`,
		`aria-describedby="creation-year-max-output"`,
		`aria-describedby="first-album-year-min-output"`,
		`aria-describedby="first-album-year-max-output"`,
	} {
		if !strings.Contains(form, want) {
			t.Fatalf("expected %q in filter form", want)
		}
	}
	for _, forbidden := range []string{
		`Apply filters`,
		`Clear filters`,
		`Reset all`,
		`class="filter-actions"`,
	} {
		if strings.Contains(form, forbidden) {
			t.Fatalf("unexpected sidebar action %q in filter form: %s", forbidden, form)
		}
	}
	if strings.Contains(form, "aria-pressed") {
		t.Fatalf("member filters must use native checkbox state, got: %s", form)
	}
}

func TestRootPageRendersFixedMemberOptions(t *testing.T) {
	body := renderRootBodyWithCatalog(t, "/?members=1&members=3&members=8%2B", testCatalog())
	form := filterFormSection(t, body)

	memberOne := inputTagByNameValue(t, form, "members", "1")
	memberTwo := inputTagByNameValue(t, form, "members", "2")
	memberThree := inputTagByNameValue(t, form, "members", "3")
	memberEightPlus := inputTagByNameValue(t, form, "members", "8&#43;")

	assertInputContains(t, memberOne, []string{`type="checkbox"`, `checked`})
	assertInputContains(t, memberTwo, []string{`type="checkbox"`})
	assertInputContains(t, memberThree, []string{`type="checkbox"`, `checked`})
	assertInputContains(t, memberEightPlus, []string{`type="checkbox"`, `checked`})
	if strings.Contains(memberTwo, `checked`) {
		t.Fatalf("unselected member option should not be checked: %s", memberTwo)
	}
	for _, want := range []string{`value="4"`, `value="5"`, `value="6"`, `value="7"`, `<span>8&#43;</span>`} {
		if !strings.Contains(form, want) {
			t.Fatalf("expected fixed member option %q in form: %s", want, form)
		}
	}
	if strings.Contains(form, `value="8"`) {
		t.Fatalf("8+ member option must not submit as plain 8: %s", form)
	}
}

func TestRootPageRendersDynamicLocationOptions(t *testing.T) {
	body := renderRootBodyWithCatalog(t, "/?locations=Seattle%2C+Washington%2C+USA", testCatalog())
	form := filterFormSection(t, body)

	selected := inputTagByNameValue(t, form, "locations", "Seattle, Washington, USA")
	assertInputContains(t, selected, []string{`type="checkbox"`, `checked`})

	for _, want := range []string{
		`value="Seattle, Washington, USA"`,
		`value="Los Angeles, USA"`,
		`value="London, UK"`,
		`data-location-search-key="seattle washington usa"`,
	} {
		if !strings.Contains(form, want) {
			t.Fatalf("expected dynamic location option %q in filter form", want)
		}
	}
	for _, forbidden := range []string{
		`value="Washington, USA"`,
		`value="USA"`,
		`value="UK"`,
		`value="Tokyo"`,
		`value="Paris"`,
		`value="Berlin"`,
		`value="Amsterdam"`,
		`value="Madrid"`,
		`value="Chicago"`,
	} {
		if strings.Contains(form, forbidden) {
			t.Fatalf("unexpected placeholder location option %q in filter form", forbidden)
		}
	}
}

func TestRootPageEscapesDynamicFilterOptions(t *testing.T) {
	cat := catalog.Catalog{
		Artists: []catalog.ArtistEntry{
			testArtist(
				1,
				"Escaped Filter",
				"escaped.jpeg",
				[]string{"A&B"},
				1999,
				"01-01-2000",
				[]catalog.Location{
					testLocation("a_and_b-usa", "A&B, USA", "a&b, usa", []string{"a&b, usa", "usa"}),
				},
			),
		},
	}
	body := renderRootBodyWithCatalog(t, "/?locations=A%26B%2C+USA", cat)
	form := filterFormSection(t, body)

	for _, want := range []string{
		`value="A&amp;B, USA"`,
		`data-location-search-key="a&amp;b usa"`,
		`<span>A&amp;B, USA</span>`,
	} {
		if !strings.Contains(form, want) {
			t.Fatalf("expected escaped dynamic option %q in filter form: %s", want, form)
		}
	}
	if strings.Contains(form, `value="A&B, USA"`) || strings.Contains(form, `<span>A&B, USA</span>`) {
		t.Fatalf("unescaped dynamic location rendered in filter form: %s", form)
	}
}

func TestFilterFormPreservesMeaningfulSearchOnly(t *testing.T) {
	t.Run("typed search", func(t *testing.T) {
		form := filterFormSection(t, renderRootBodyWithCatalog(t, "/?q=Echo+Lane&search_type=artist&members=2", testCatalog()))

		inputTagByNameValue(t, form, "q", "Echo Lane")
		inputTagByNameValue(t, form, "search_type", "artist")
		inputTagByNameValue(t, form, "members", "2")
	})

	t.Run("broad search", func(t *testing.T) {
		form := filterFormSection(t, renderRootBodyWithCatalog(t, "/?q=echo&members=2", testCatalog()))

		inputTagByNameValue(t, form, "q", "echo")
		if strings.Contains(form, `name="search_type"`) {
			t.Fatalf("broad search should not preserve an empty search_type in filter form: %s", form)
		}
	})

	t.Run("empty search clears stale type", func(t *testing.T) {
		form := filterFormSection(t, renderRootBodyWithCatalog(t, "/?q=+++&search_type=artist&members=2", testCatalog()))

		if strings.Contains(form, `name="q"`) || strings.Contains(form, `name="search_type"`) {
			t.Fatalf("empty search state should not be preserved in filter form: %s", form)
		}
		inputTagByNameValue(t, form, "members", "2")
	})
}

func TestRootPageFilterRangeSelectedValuesFromCriteria(t *testing.T) {
	body := renderRootBodyWithCatalog(
		t,
		"/?creation_from=1980&creation_to=1990&album_from=1984-05-05&album_to=1995-01-10",
		testCatalog(),
	)

	assertRangeInputs(t, body, "creation", RangeView{Min: 1980, Max: 2001, SelectedMin: 1980, SelectedMax: 1990})
	assertRangeInputs(t, body, "first-album", RangeView{Min: 1984, Max: 2003, SelectedMin: 1984, SelectedMax: 1995})
}

func TestRootPageFilterRangeSelectedValuesClamp(t *testing.T) {
	body := renderRootBodyWithCatalog(
		t,
		"/?creation_from=1900&creation_to=2024&album_from=1900-01-01&album_to=2050-01-01",
		testCatalog(),
	)

	assertRangeInputs(t, body, "creation", RangeView{Min: 1980, Max: 2001, SelectedMin: 1980, SelectedMax: 2001})
	assertRangeInputs(t, body, "first-album", RangeView{Min: 1984, Max: 2003, SelectedMin: 1984, SelectedMax: 2003})
}

func TestRootPageFilterOptionsUseFullCatalogWhenCardsAreFiltered(t *testing.T) {
	body := renderRootBodyWithCatalog(t, "/?q=echo+lane&search_type=artist", testCatalog())

	assertRenderedArtists(t, body, []string{"Echo Lane"})
	assertRangeInputs(t, body, "creation", RangeView{Min: 1980, Max: 2001, SelectedMin: 1980, SelectedMax: 2001})
	assertRangeInputs(t, body, "first-album", RangeView{Min: 1984, Max: 2003, SelectedMin: 1984, SelectedMax: 2003})
}

func TestRootPageEmptyCatalogUsesFallbackFilterRanges(t *testing.T) {
	body := renderRootBodyWithCatalog(t, "/", catalog.Catalog{})

	assertRangeInputs(t, body, "creation", RangeView{Min: fallbackCreationMin, Max: fallbackCreationMax, SelectedMin: fallbackCreationMin, SelectedMax: fallbackCreationMax})
	assertRangeInputs(t, body, "first-album", RangeView{Min: fallbackAlbumMin, Max: fallbackAlbumMax, SelectedMin: fallbackAlbumMin, SelectedMax: fallbackAlbumMax})
}

func performFilterOptionsRequest(t *testing.T, mux http.Handler, method string, path string) (*http.Response, []byte) {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	res := rec.Result()
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	return res, body
}

func decodeFilterOptionsResponse(t *testing.T, body []byte) filterOptionsResponse {
	t.Helper()
	var got filterOptionsResponse
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("decode filter options response: %v; body=%s", err, string(body))
	}
	if got.MemberCounts == nil {
		got.MemberCounts = []int{}
	}
	if got.Locations == nil {
		got.Locations = []locationOptionResponse{}
	}
	return got
}

func renderRootBodyWithCatalog(t *testing.T, path string, cat catalog.Catalog) string {
	t.Helper()
	mux := initRoutes(testDependenciesWithCatalog(cat, nil))
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	res := rec.Result()
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	return string(body)
}

func assertRangeInputs(t *testing.T, body string, rangeName string, want RangeView) {
	t.Helper()
	section := rangeSection(t, body, rangeName)
	inputs := inputTagsInText(section)
	if len(inputs) < 2 {
		t.Fatalf("expected two range inputs for %q, got %d in %s", rangeName, len(inputs), section)
	}

	assertInputContains(t, inputs[0], []string{
		`type="range"`,
		`min="` + intStringForTest(want.Min) + `"`,
		`max="` + intStringForTest(want.Max) + `"`,
		`value="` + intStringForTest(want.SelectedMin) + `"`,
	})
	if strings.Contains(inputs[0], `name=`) {
		t.Fatalf("range input must not submit filters yet: %s", inputs[0])
	}
	assertInputContains(t, inputs[1], []string{
		`type="range"`,
		`min="` + intStringForTest(want.Min) + `"`,
		`max="` + intStringForTest(want.Max) + `"`,
		`value="` + intStringForTest(want.SelectedMax) + `"`,
	})
	if strings.Contains(inputs[1], `name=`) {
		t.Fatalf("range input must not submit filters yet: %s", inputs[1])
	}
}

func rangeSection(t *testing.T, body string, rangeName string) string {
	t.Helper()
	marker := `data-filter-range="` + rangeName + `"`
	start := strings.Index(body, marker)
	if start < 0 {
		t.Fatalf("range %q not found", rangeName)
	}
	end := strings.Index(body[start:], "</fieldset>")
	if end < 0 {
		t.Fatalf("range section %q not closed", rangeName)
	}
	return body[start : start+end]
}

func filterFormSection(t *testing.T, body string) string {
	t.Helper()
	marker := `data-filter-form`
	markerIndex := strings.Index(body, marker)
	if markerIndex < 0 {
		t.Fatal("filter form not found")
	}
	start := strings.LastIndex(body[:markerIndex], "<form")
	if start < 0 {
		t.Fatal("filter form opening tag not found")
	}
	end := strings.Index(body[start:], "</form>")
	if end < 0 {
		t.Fatal("filter form not closed")
	}
	return body[start : start+end]
}

func inputTagsInText(text string) []string {
	var tags []string
	remaining := text
	for {
		start := strings.Index(remaining, "<input")
		if start < 0 {
			return tags
		}
		end := strings.Index(remaining[start:], ">")
		if end < 0 {
			return tags
		}
		tags = append(tags, remaining[start:start+end+1])
		remaining = remaining[start+end+1:]
	}
}

func inputTagByNameValue(t *testing.T, text string, name string, value string) string {
	t.Helper()
	for _, input := range inputTagsInText(text) {
		if strings.Contains(input, `name="`+name+`"`) && strings.Contains(input, `value="`+value+`"`) {
			return input
		}
	}
	t.Fatalf("input name=%q value=%q not found in %s", name, value, text)
	return ""
}

func assertInputContains(t *testing.T, input string, wants []string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(input, want) {
			t.Fatalf("expected %q in input %s", want, input)
		}
	}
}

func assertTemplateDoesNotDuplicateFallbackYears(t *testing.T) {
	t.Helper()
	templateBytes, err := tmplFS.ReadFile("templates/index.html")
	if err != nil {
		t.Fatalf("read index template: %v", err)
	}
	templateText := string(templateBytes)
	for _, forbidden := range []string{"1950", "1960", "2024"} {
		if strings.Contains(templateText, forbidden) {
			t.Fatalf("fallback year %q must not be duplicated in index template", forbidden)
		}
	}
}
