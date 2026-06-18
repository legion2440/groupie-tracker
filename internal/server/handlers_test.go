package server

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"groupie-tracker/internal/catalog"
	"groupie-tracker/internal/model"
)

// убеждаемся, что корень ("/") отвечает HTTP 200 и содержит карточки артистов.
func TestRootHandler(t *testing.T) {
	loadCalls := 0
	mux := initRoutes(testDependenciesWithCatalog(testCatalog(), &loadCalls))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}
	if ct := res.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("unexpected Content-Type: %s", ct)
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	assertRenderedArtists(t, string(body), []string{"Echo Lane", "Ava Stone", "Northern Lights"})
	if loadCalls != 1 {
		t.Fatalf("expected loader to be called once, got %d", loadCalls)
	}
}

func TestSearchFormMarkup(t *testing.T) {
	body := renderRootBody(t, "/")

	for _, want := range []string{
		`method="get"`,
		`action="/"`,
		`data-search-form`,
		`name="q"`,
		`id="artist-search-type"`,
		`id="artist-search-suggestions"`,
		`role="listbox"`,
		`id="artist-search-status"`,
		`/static/results.js`,
		`/static/search.js`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected %q in rendered HTML", want)
		}
	}

	searchInput := inputTagByID(t, body, "artist-search")
	if strings.Contains(searchInput, "readonly") {
		t.Fatalf("search input must not be readonly: %s", searchInput)
	}
	for _, want := range []string{
		`role="combobox"`,
		`aria-autocomplete="list"`,
		`aria-controls="artist-search-suggestions"`,
		`aria-expanded="false"`,
		`maxlength="200"`,
	} {
		if !strings.Contains(searchInput, want) {
			t.Fatalf("expected %q in search input: %s", want, searchInput)
		}
	}

	typeInput := inputTagByID(t, body, "artist-search-type")
	if strings.Contains(typeInput, `name="search_type"`) {
		t.Fatalf("empty search type should not submit by default: %s", typeInput)
	}
}

func TestSearchStatePersistence(t *testing.T) {
	body := renderRootBody(t, "/?q=Echo+Lane&search_type=member")

	searchInput := inputTagByID(t, body, "artist-search")
	if !strings.Contains(searchInput, `value="Echo Lane"`) {
		t.Fatalf("expected visible search value to persist: %s", searchInput)
	}

	typeInput := inputTagByID(t, body, "artist-search-type")
	if !strings.Contains(typeInput, `value="member"`) {
		t.Fatalf("expected typed search value to persist: %s", typeInput)
	}
	if !strings.Contains(typeInput, `name="search_type"`) {
		t.Fatalf("expected meaningful typed search to keep search_type name: %s", typeInput)
	}
}

func TestBroadSearchStatePersistence(t *testing.T) {
	body := renderRootBody(t, "/?q=echo")

	searchInput := inputTagByID(t, body, "artist-search")
	if !strings.Contains(searchInput, `value="echo"`) {
		t.Fatalf("expected broad search value to persist: %s", searchInput)
	}

	typeInput := inputTagByID(t, body, "artist-search-type")
	if !strings.Contains(typeInput, `value=""`) {
		t.Fatalf("expected empty typed search value: %s", typeInput)
	}
	if strings.Contains(typeInput, `name="search_type"`) {
		t.Fatalf("broad search should not submit empty search_type: %s", typeInput)
	}
}

func TestEmptySearchQueryClearsTypedStateAndPreservesFilters(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		wantHidden string
		want       []string
	}{
		{
			name:       "empty query",
			path:       "/?q=&search_type=member&members=3",
			wantHidden: `name="members" value="3"`,
			want:       []string{"Northern Lights"},
		},
		{
			name:       "decoded whitespace query",
			path:       "/?q=+++&search_type=location&locations=London%2C+UK",
			wantHidden: `name="locations" value="London, UK"`,
			want:       []string{"Northern Lights"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := renderRootBody(t, tt.path)
			assertRenderedArtists(t, body, tt.want)

			typeInput := inputTagByID(t, body, "artist-search-type")
			if !strings.Contains(typeInput, `value=""`) {
				t.Fatalf("expected empty typed search value: %s", typeInput)
			}
			if !strings.Contains(body, tt.wantHidden) {
				t.Fatalf("expected active filter to be preserved as %q", tt.wantHidden)
			}
		})
	}
}

func TestSearchFormPreservesFilterState(t *testing.T) {
	body := renderRootBody(t, "/?q=echo&creation_from=1980&creation_to=2001&album_from=1984-05-05&album_to=2003-07-20&members=1&members=3&locations=Washington%2C+USA&locations=London%2C+UK")

	for _, want := range []string{
		`name="creation_from" value="1980"`,
		`name="creation_to" value="2001"`,
		`name="album_from" value="1984-05-05"`,
		`name="album_to" value="2003-07-20"`,
		`name="members" value="1"`,
		`name="members" value="3"`,
		`name="locations" value="Washington, USA"`,
		`name="locations" value="London, UK"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected preserved hidden input %q in body", want)
		}
	}
}

func TestArtistDetailSearchFormMarkup(t *testing.T) {
	body := renderArtistDetailTemplate(t)

	for _, want := range []string{
		`method="get"`,
		`action="/"`,
		`data-search-form`,
		`name="q"`,
		`id="artist-search-type"`,
		`role="combobox"`,
		`role="listbox"`,
		`/static/search.js`,
		`/static/previews.js`,
		`class="artist-vinyl" data-preview-artist="Echo Lane"`,
		`placeholder="Search artists, members, albums, locations"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected %q in rendered artist HTML", want)
		}
	}

	searchInput := inputTagByID(t, body, "artist-search")
	if strings.Contains(searchInput, "readonly") {
		t.Fatalf("artist search input must not be readonly: %s", searchInput)
	}
	for _, want := range []string{
		`maxlength="200"`,
		`aria-autocomplete="list"`,
		`aria-controls="artist-search-suggestions"`,
		`aria-expanded="false"`,
	} {
		if !strings.Contains(searchInput, want) {
			t.Fatalf("expected %q in artist search input: %s", want, searchInput)
		}
	}

	typeInput := inputTagByID(t, body, "artist-search-type")
	if strings.Contains(typeInput, `name="search_type"`) {
		t.Fatalf("artist page empty search type should not submit by default: %s", typeInput)
	}
}

func TestArtistSlugRoute(t *testing.T) {
	mux := initRoutes(testDependenciesWithCatalog(testCatalog(), nil))
	req := httptest.NewRequest(http.MethodGet, "/echo-lane", nil)
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
	if !strings.Contains(string(body), "Echo Lane") {
		t.Fatalf("expected artist page content, got: %s", string(body))
	}
}

func TestUnknownArtistSlugReturns404(t *testing.T) {
	mux := initRoutes(testDependenciesWithCatalog(testCatalog(), nil))
	req := httptest.NewRequest(http.MethodGet, "/unknown-artist", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	res := rec.Result()
	defer res.Body.Close()
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", res.StatusCode)
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	bodyText := string(body)
	for _, want := range []string{
		`class="page-home page-error-404"`,
		`class="topbar"`,
		`class="search-shell"`,
		`id="theme-switch"`,
		`src="/static/groupie_tracker_404_neon.gif"`,
		`class="footer"`,
		`TO MAIN PAGE`,
		`href="/"`,
	} {
		if !strings.Contains(bodyText, want) {
			t.Fatalf("expected custom 404 page to contain %q, got: %s", want, bodyText)
		}
	}
	for _, old := range []string{
		"Ошибка 404",
		"Артист не найден",
		"← На главную",
	} {
		if strings.Contains(bodyText, old) {
			t.Fatalf("expected custom 404 page to omit old content %q, got: %s", old, bodyText)
		}
	}
}

func TestLegacyArtistURLRedirectsToCanonicalSlug(t *testing.T) {
	mux := initRoutes(testDependenciesWithCatalog(testCatalog(), nil))
	req := httptest.NewRequest(http.MethodGet, "/artist/1", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	res := rec.Result()
	defer res.Body.Close()
	if res.StatusCode != http.StatusMovedPermanently {
		t.Fatalf("expected 301, got %d", res.StatusCode)
	}
	if location := res.Header.Get("Location"); location != "/echo-lane" {
		t.Fatalf("redirect Location = %q, want /echo-lane", location)
	}
}

func TestRootSlugRouteDoesNotConflictWithSystemPaths(t *testing.T) {
	mux := initRoutes(testDependenciesWithCatalog(testCatalog(), nil))

	tests := []struct {
		path string
		want int
	}{
		{path: "/", want: http.StatusOK},
		{path: "/api/filter-options", want: http.StatusOK},
		{path: "/static/style.css", want: http.StatusOK},
		{path: "/static/groupie_tracker_404_neon.gif", want: http.StatusOK},
		{path: "/api/not-found", want: http.StatusNotFound},
		{path: "/static/not-found.css", want: http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()

			mux.ServeHTTP(rec, req)

			res := rec.Result()
			defer res.Body.Close()
			if res.StatusCode != tt.want {
				t.Fatalf("expected %d, got %d", tt.want, res.StatusCode)
			}
		})
	}
}

func TestArtistCardsUseSlugURLs(t *testing.T) {
	body := renderRootBody(t, "/")

	for _, want := range []string{
		`href="/echo-lane"`,
		`href="/ava-stone"`,
		`href="/northern-lights"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected artist card link %q in body", want)
		}
	}
	if strings.Contains(body, `href="/artist/`) {
		t.Fatalf("artist cards must not link to legacy numeric URLs: %s", body)
	}
}

func TestPageActiveSearchFromCriteria(t *testing.T) {
	tests := []struct {
		name       string
		criteria   catalog.Criteria
		wantSearch bool
	}{
		{name: "empty"},
		{
			name:       "search only",
			criteria:   catalog.Criteria{SearchText: "echo"},
			wantSearch: true,
		},
		{
			name: "search and filters",
			criteria: catalog.Criteria{
				SearchText:   "echo",
				SearchField:  catalog.SearchArtist,
				MemberCounts: []int{2},
				Locations:    []string{"Washington, USA"},
			},
			wantSearch: true,
		},
		{
			name:       "stale typed search with empty query",
			criteria:   catalog.Criteria{SearchText: "   ", SearchField: catalog.SearchArtist},
			wantSearch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasActiveSearch(tt.criteria); got != tt.wantSearch {
				t.Fatalf("hasActiveSearch() = %v, want %v", got, tt.wantSearch)
			}
		})
	}
}

func TestRootHandlerEmptyResultsState(t *testing.T) {
	body := renderRootBody(t, "/?q=missing")

	for _, want := range []string{
		`class="artist-empty"`,
		`role="status"`,
		`No artists found`,
		`Try changing the search or filters.`,
		`href="/"`,
		`Reset`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected empty-state content %q in body", want)
		}
	}
	for _, forbidden := range []string{
		`class="card artist-card"`,
		`0 artists`,
		`0 of`,
		`ResultCount`,
		`TotalCount`,
		`Clear filters`,
		`Reset all`,
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("unexpected count/card marker %q in empty state", forbidden)
		}
	}
}

func TestRootHandlerEmptyStateActions(t *testing.T) {
	for _, path := range []string{
		"/?q=missing",
		"/?members=1&locations=London%2C+UK",
		"/?q=Echo&search_type=artist&locations=London%2C+UK",
	} {
		body := renderRootBody(t, path)
		empty := emptyStateSection(t, body)

		if got := strings.Count(empty, `href="/"`); got != 1 {
			t.Fatalf("empty state should render exactly one reset href, got %d in %s", got, empty)
		}
		if got := strings.Count(empty, `>Reset<`); got != 1 {
			t.Fatalf("empty state should render exactly one Reset action, got %d in %s", got, empty)
		}
		for _, forbidden := range []string{`Clear filters`, `Reset all`} {
			if strings.Contains(empty, forbidden) {
				t.Fatalf("empty state must not render %q: %s", forbidden, empty)
			}
		}
	}
}

func TestRenderedAppTextDoesNotUseLongDashSeparators(t *testing.T) {
	pages := []string{
		renderRootBody(t, "/"),
		renderRootBody(t, "/?q=missing"),
		renderArtistDetailTemplate(t),
		renderErrorTemplate(t),
	}
	for _, body := range pages {
		assertNoLongDashForServerTest(t, body)
	}

	mux := initRoutes(testDependenciesWithCatalog(testCatalog(), nil))
	res, body := performSuggestionsRequest(t, mux, http.MethodGet, "/api/search/suggestions?q=london")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from suggestions, got %d", res.StatusCode)
	}
	assertNoLongDashForServerTest(t, string(body))
	if !strings.Contains(string(body), " - ") {
		t.Fatalf("expected ASCII suggestion separator in JSON: %s", string(body))
	}
}

func TestSearchStateEscapesHTML(t *testing.T) {
	rawQuery := `<script>alert("x")</script>&`
	body := renderRootBody(t, "/?q="+url.QueryEscape(rawQuery))

	if strings.Contains(body, rawQuery) {
		t.Fatalf("unescaped query was injected into HTML: %s", body)
	}
	searchInput := inputTagByID(t, body, "artist-search")
	for _, want := range []string{"&lt;script&gt;", "&#34;x&#34;", "&amp;"} {
		if !strings.Contains(searchInput, want) {
			t.Fatalf("expected escaped %q in search input: %s", want, searchInput)
		}
	}
}

func TestRootHandlerFilters(t *testing.T) {
	tests := []struct {
		name string
		path string
		want []string
	}{
		{
			name: "broad search",
			path: "/?q=northern",
			want: []string{"Northern Lights"},
		},
		{
			name: "typed artist search",
			path: "/?q=echo+lane&search_type=artist",
			want: []string{"Echo Lane"},
		},
		{
			name: "typed member search",
			path: "/?q=echo+lane&search_type=member",
			want: []string{"Ava Stone"},
		},
		{
			name: "creation range",
			path: "/?creation_from=1990&creation_to=1990",
			want: []string{"Echo Lane"},
		},
		{
			name: "first album range",
			path: "/?album_from=1984-05-05&album_to=1984-05-05",
			want: []string{"Ava Stone"},
		},
		{
			name: "member count OR",
			path: "/?members=1&members=3",
			want: []string{"Ava Stone", "Northern Lights"},
		},
		{
			name: "location hierarchy",
			path: "/?locations=Washington%2C+USA",
			want: []string{"Echo Lane"},
		},
		{
			name: "combined criteria",
			path: "/?q=echo&search_type=artist&creation_from=1985&creation_to=1995&album_from=1995-01-10&album_to=1995-01-10&members=2&locations=Washington%2C+USA",
			want: []string{"Echo Lane"},
		},
		{
			name: "no matches",
			path: "/?q=missing",
			want: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := initRoutes(testDependenciesWithCatalog(testCatalog(), nil))
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
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
			assertRenderedArtists(t, string(body), tt.want)
		})
	}
}

func TestRootHandlerMemberEightPlusFilter(t *testing.T) {
	cat := catalog.Catalog{
		Artists: []catalog.ArtistEntry{
			testArtist(10, "Quartet", "quartet.jpeg", []string{"A", "B", "C", "D"}, 1990, "10-01-1995", nil),
			testArtist(11, "Octet", "octet.jpeg", []string{"A", "B", "C", "D", "E", "F", "G", "H"}, 1991, "11-01-1995", nil),
			testArtist(12, "Nonet", "nonet.jpeg", []string{"A", "B", "C", "D", "E", "F", "G", "H", "I"}, 1992, "12-01-1995", nil),
		},
	}

	body := renderRootBodyWithCatalog(t, "/?members=8%2B", cat)
	for _, want := range []string{`aria-label="Octet"`, `aria-label="Nonet"`, `name="members" value="8&#43;"`, `value="8&#43;"`, `<span>8&#43;</span>`} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected %q in body: %s", want, body)
		}
	}
	if strings.Contains(body, `aria-label="Quartet"`) || strings.Contains(body, `value="8"`) {
		t.Fatalf("8+ filter must mean >=8 and must not render plain value 8: %s", body)
	}

	body = renderRootBodyWithCatalog(t, "/?members=4&members=8%2B", cat)
	for _, want := range []string{`aria-label="Quartet"`, `aria-label="Octet"`, `aria-label="Nonet"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected OR member filter result %q in body: %s", want, body)
		}
	}
}

func TestRootPageFormIntegration(t *testing.T) {
	body := renderRootBody(t, "/?q=Echo+Lane&search_type=artist&members=1&members=3&locations=Washington%2C+USA&locations=London%2C+UK")

	if strings.Count(body, "<form") != 2 || strings.Count(body, "</form>") != 2 {
		t.Fatalf("expected exactly two sibling forms, got body: %s", body)
	}
	if strings.Contains(searchFormSection(t, body), `data-filter-form`) {
		t.Fatalf("filter form nested in search form")
	}
	if strings.Contains(filterFormSection(t, body), `data-search-form`) {
		t.Fatalf("search form nested in filter form")
	}

	searchForm := searchFormSection(t, body)
	if countInputsByName(searchForm, "q") != 1 {
		t.Fatalf("expected one q control in search form, got %d in %s", countInputsByName(searchForm, "q"), searchForm)
	}
	if countInputsByName(searchForm, "search_type") != 1 {
		t.Fatalf("expected one search_type control in typed search form, got %d in %s", countInputsByName(searchForm, "search_type"), searchForm)
	}

	filterForm := filterFormSection(t, body)
	if countInputsByName(filterForm, "q") != 1 {
		t.Fatalf("expected one q control in filter form, got %d in %s", countInputsByName(filterForm, "q"), filterForm)
	}
	if countInputsByName(filterForm, "search_type") != 1 {
		t.Fatalf("expected one search_type control in filter form, got %d in %s", countInputsByName(filterForm, "search_type"), filterForm)
	}
	for _, value := range []string{"1", "2", "3"} {
		if got := countInputsByNameValue(filterForm, "members", value); got > 1 {
			t.Fatalf("duplicate generated member value %q in filter form: %s", value, filterForm)
		}
	}
	for _, value := range []string{"Washington, USA", "London, UK"} {
		if got := countInputsByNameValue(filterForm, "locations", value); got > 1 {
			t.Fatalf("duplicate generated location value %q in filter form: %s", value, filterForm)
		}
	}
	if locationSearch := inputTagByID(t, filterForm, "location-search"); strings.Contains(locationSearch, `name=`) {
		t.Fatalf("local location search must not submit: %s", locationSearch)
	}
	for _, forbidden := range []string{
		`name="creation_from" value=`,
		`name="creation_to" value=`,
		`name="album_from" value=`,
		`name="album_to" value=`,
	} {
		if strings.Contains(filterForm, forbidden) {
			t.Fatalf("default range hidden field should not carry a value in server HTML: %q", forbidden)
		}
	}
}

func TestRootPagePlaceholderMarkupRemoved(t *testing.T) {
	body := renderRootBody(t, "/")

	for _, forbidden := range []string{
		"Filter placeholders",
		"Location examples",
		"ResultCount",
		"TotalCount",
		"/force500",
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("unexpected placeholder/count marker %q in body", forbidden)
		}
	}
	if !strings.Contains(body, `value="8&#43;"`) || !strings.Contains(body, `<span>8&#43;</span>`) {
		t.Fatalf("expected fixed 8+ member option in body")
	}
}

func renderRootBody(t *testing.T, path string) string {
	t.Helper()
	mux := initRoutes(testDependenciesWithCatalog(testCatalog(), nil))
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

func inputTagByID(t *testing.T, body string, id string) string {
	t.Helper()
	pattern := regexp.MustCompile(`<input\b[^>]*\bid="` + regexp.QuoteMeta(id) + `"[^>]*>`)
	tag := pattern.FindString(body)
	if tag == "" {
		t.Fatalf("input with id %q not found", id)
	}
	return tag
}

func searchFormSection(t *testing.T, body string) string {
	t.Helper()
	return formSectionByMarker(t, body, `data-search-form`)
}

func formSectionByMarker(t *testing.T, body string, marker string) string {
	t.Helper()
	markerIndex := strings.Index(body, marker)
	if markerIndex < 0 {
		t.Fatalf("form marker %q not found", marker)
	}
	start := strings.LastIndex(body[:markerIndex], "<form")
	if start < 0 {
		t.Fatalf("form marker %q opening tag not found", marker)
	}
	end := strings.Index(body[start:], "</form>")
	if end < 0 {
		t.Fatalf("form marker %q closing tag not found", marker)
	}
	return body[start : start+end]
}

func emptyStateSection(t *testing.T, body string) string {
	t.Helper()
	marker := `class="artist-empty"`
	start := strings.Index(body, marker)
	if start < 0 {
		t.Fatal("artist empty state not found")
	}
	open := strings.LastIndex(body[:start], "<div")
	if open < 0 {
		t.Fatal("artist empty state opening div not found")
	}
	end := strings.Index(body[open:], `</section>`)
	if end < 0 {
		t.Fatal("artist section not closed")
	}
	return body[open : open+end]
}

func countInputsByName(text string, name string) int {
	count := 0
	for _, input := range inputTagsInText(text) {
		if strings.Contains(input, `name="`+name+`"`) {
			count++
		}
	}
	return count
}

func countInputsByNameValue(text string, name string, value string) int {
	count := 0
	for _, input := range inputTagsInText(text) {
		if strings.Contains(input, `name="`+name+`"`) && strings.Contains(input, `value="`+value+`"`) {
			count++
		}
	}
	return count
}

func renderArtistDetailTemplate(t *testing.T) string {
	t.Helper()
	var buf bytes.Buffer
	err := tmplAll.ExecuteTemplate(&buf, "artist.html", ArtistDetailPage{
		Artist: model.Artist{
			ID:           1,
			Name:         "Echo Lane",
			Image:        "echo.jpeg",
			Members:      []string{"Ava Stone", "Chris Pike"},
			CreationDate: 1990,
			FirstAlbum:   "10-01-1995",
		},
		Concerts: []ConcertsByLocation{
			{Location: "Seattle, Washington, USA", Dates: []string{"10-01-1995"}},
		},
	})
	if err != nil {
		t.Fatalf("render artist detail template: %v", err)
	}
	return buf.String()
}

func renderErrorTemplate(t *testing.T) string {
	t.Helper()
	var buf bytes.Buffer
	err := tmplAll.ExecuteTemplate(&buf, "error.html", struct {
		Code    int
		Message string
		Query   QueryState
	}{Code: http.StatusInternalServerError, Message: "Internal Server Error", Query: QueryState{}})
	if err != nil {
		t.Fatalf("render error template: %v", err)
	}
	return buf.String()
}

func assertNoLongDashForServerTest(t *testing.T, text string) {
	t.Helper()
	if strings.ContainsAny(text, "—–") {
		t.Fatalf("unexpected long dash in rendered output: %s", text)
	}
}

func TestRootHandlerInvalidQuery(t *testing.T) {
	tests := []string{
		"/?creation_from=bad",
		"/?creation_from=2000&creation_to=1990",
		"/?album_from=bad-date",
		"/?album_from=2000-01-01&album_to=1990-01-01",
		"/?search_type=band",
		"/?members=0",
	}

	for _, path := range tests {
		t.Run(path, func(t *testing.T) {
			mux := initRoutes(testDependenciesWithCatalog(testCatalog(), nil))
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()

			mux.ServeHTTP(rec, req)

			res := rec.Result()
			defer res.Body.Close()

			if res.StatusCode != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d", res.StatusCode)
			}
			body, err := io.ReadAll(res.Body)
			if err != nil {
				t.Fatalf("read response body: %v", err)
			}
			bodyText := string(body)
			if !strings.Contains(bodyText, "ERROR 400") || !strings.Contains(bodyText, "Некорректные параметры поиска или фильтров") {
				t.Fatalf("expected custom 400 page, got: %s", bodyText)
			}
			for _, want := range []string{
				`class="error-page error-page--standard"`,
				`class="topbar"`,
				`class="search-shell"`,
				`id="theme-switch"`,
				`class="footer"`,
				`TO MAIN PAGE`,
				`href="/"`,
			} {
				if !strings.Contains(bodyText, want) {
					t.Fatalf("expected custom 400 page to contain %q, got: %s", want, bodyText)
				}
			}
			if strings.Contains(bodyText, "groupie_tracker_404_neon.gif") {
				t.Fatalf("expected custom 400 page without 404 GIF, got: %s", bodyText)
			}
			if strings.Contains(bodyText, "Ошибка 400") {
				t.Fatalf("expected custom 400 page without duplicate Russian title, got: %s", bodyText)
			}
		})
	}
}

func TestRootHandlerLoaderFailure(t *testing.T) {
	mux := initRoutes(dependencies{
		updateNow: func() error {
			return nil
		},
		loadCatalog: func() (catalog.Catalog, error) {
			return catalog.Catalog{}, errors.New("fixture loader failure")
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", res.StatusCode)
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if !strings.Contains(string(body), "ERROR 500") || !strings.Contains(string(body), "Ошибка загрузки артистов") {
		t.Fatalf("expected custom 500 page, got: %s", string(body))
	}
}

// Проверяет, что ошибка injected updater возвращает статус 500 и фирменную страницу ошибки.
func Test500Template(t *testing.T) {
	forcedErr := errors.New("forced refresh failure")
	mux := initRoutes(dependencies{
		updateNow: func() error {
			return forcedErr
		},
		loadCatalog: func() (catalog.Catalog, error) {
			return testCatalog(), nil
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/refresh", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", res.StatusCode)
	}
	if ct := res.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("expected text/html Content-Type, got %q", ct)
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	bodyText := string(body)
	if !strings.Contains(bodyText, "ERROR 500") {
		t.Fatalf("expected custom error page, got: %s", bodyText)
	}
	if !strings.Contains(bodyText, "Internal Server Error") {
		t.Fatalf("expected internal server error message, got: %s", bodyText)
	}
}

func TestRefreshSuccess(t *testing.T) {
	callCount := 0
	mux := initRoutes(dependencies{
		updateNow: func() error {
			callCount++
			return nil
		},
		loadCatalog: func() (catalog.Catalog, error) {
			return testCatalog(), nil
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/refresh", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}
	if callCount != 1 {
		t.Fatalf("expected updater to be called once, got %d", callCount)
	}
}

// Проверяет, что при некорректном HTTP-методе (GET вместо POST) на /api/refresh
// сервер возвращает статус 400 и фирменную страницу ошибки.
func Test400Template(t *testing.T) {
	callCount := 0
	mux := initRoutes(dependencies{
		updateNow: func() error {
			callCount++
			return nil
		},
		loadCatalog: func() (catalog.Catalog, error) {
			return testCatalog(), nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/refresh", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", res.StatusCode)
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	bodyText := string(body)
	if !strings.Contains(bodyText, "Неверный HTTP-метод") {
		t.Fatalf("expected custom error page, got: %s", bodyText)
	}
	if callCount != 0 {
		t.Fatalf("expected updater not to be called, got %d calls", callCount)
	}
}
