package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFilterOptionsJSServed(t *testing.T) {
	mux := initRoutes(testDependenciesWithCatalog(testCatalog(), nil))

	req := httptest.NewRequest(http.MethodGet, "/static/filter-options.js", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}
	if ct := res.Header.Get("Content-Type"); !strings.Contains(ct, "javascript") {
		t.Fatalf("unexpected Content-Type %s", ct)
	}
}

func TestFiltersJSServed(t *testing.T) {
	mux := initRoutes(testDependenciesWithCatalog(testCatalog(), nil))

	req := httptest.NewRequest(http.MethodGet, "/static/filters.js", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}
	if ct := res.Header.Get("Content-Type"); !strings.Contains(ct, "javascript") {
		t.Fatalf("unexpected Content-Type %s", ct)
	}
}

func TestFilterOptionsScriptContracts(t *testing.T) {
	content, err := staticFS.ReadFile("static/filter-options.js")
	if err != nil {
		t.Fatalf("read filter options script: %v", err)
	}
	script := string(content)

	for _, want := range []string{
		"/api/filter-options",
		"groupie:catalog-refreshed",
		"filter-range:update",
		"AbortController",
		"cache: 'no-store'",
		"optionsController.abort();",
		"latestOptionsRequest",
		"requestID !== latestOptionsRequest",
		"Number.isInteger(value.min)",
		"Number.isInteger(value.max)",
		"Number.isInteger(count) && count >= 1 && count <= 8",
		"typeof location.value === 'string'",
		"typeof location.label === 'string'",
		"renderMemberOptions(options.memberCounts);",
		"renderLocationOptions(options.locations);",
		"checkedValues",
		"document.createElement('label')",
		"document.createElement('input')",
		"document.createElement('span')",
		"const value = count === 8 ? '8+' : String(count);",
		"input.value = value;",
		"text.textContent = value;",
		"text.textContent = option.label;",
		"input.checked = selected.has",
		"window.groupieFilters?.syncFilterParams();",
		"window.groupieFilters?.applyLocationSearch();",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("expected filter-options.js to contain %q", want)
		}
	}
	for _, forbidden := range []string{
		"innerHTML",
		"insertAdjacentHTML",
		".artist-card",
		"artist-grid",
		"requestSubmit",
		".submit()",
		"dispatchEvent(new Event('change'",
		"history.pushState",
		"history.replaceState",
		"pageshow",
	} {
		if strings.Contains(script, forbidden) {
			t.Fatalf("filter-options.js must not contain %q", forbidden)
		}
	}
}

func TestFiltersScriptContracts(t *testing.T) {
	content, err := staticFS.ReadFile("static/filters.js")
	if err != nil {
		t.Fatalf("read filters script: %v", err)
	}
	script := string(content)

	for _, want := range []string{
		"[data-filter-form]",
		"[data-location-search]",
		"[data-location-empty]",
		"normalizeLocationQuery",
		"[data-filter-param=\"creation-from\"]",
		"[data-filter-param=\"creation-to\"]",
		"[data-filter-param=\"album-from\"]",
		"[data-filter-param=\"album-to\"]",
		"yearStart",
		"yearEnd",
		"-01-01",
		"-12-31",
		"field.disabled = true;",
		"field.disabled = false;",
		"[data-location-option]",
		"option.hidden = !matched;",
		"locationEmpty.hidden = query === '' || visibleCount > 0;",
		"syncControlsFromURL",
		"syncCheckboxesFromURL(params, 'members');",
		"syncCheckboxesFromURL(params, 'locations');",
		"syncRangeFromURL('creation', 'creation_from', 'creation_to');",
		"syncRangeFromURL('first-album', 'album_from', 'album_to');",
		"document.addEventListener('groupie:results-url-applied', syncControlsFromURL);",
		"window.addEventListener('pageshow', syncControlsFromURL);",
		"minInput.dispatchEvent(new Event('input'",
		"maxInput.dispatchEvent(new Event('input'",
		"form.requestSubmit()",
		"form.submit()",
		"window.groupieResults?.submitFilterForm?.(form)",
		"event.isTrusted",
		"isFilterCheckbox(event.target)",
		"isRangeInput(event.target)",
		"submitFilterForm();",
		"form.addEventListener('submit', (event) =>",
		"form.addEventListener('change'",
		"filter-range:update",
		"window.groupieFilters =",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("expected filters.js to contain %q", want)
		}
	}
	for _, forbidden := range []string{
		"fetch(",
		"innerHTML",
		"insertAdjacentHTML",
		".artist-card",
		"artist-grid",
		"history.pushState",
		"history.replaceState",
	} {
		if strings.Contains(script, forbidden) {
			t.Fatalf("filters.js must not contain %q", forbidden)
		}
	}
}

func TestRefreshScriptRangeUpdateContracts(t *testing.T) {
	content, err := staticFS.ReadFile("static/refresh.js")
	if err != nil {
		t.Fatalf("read refresh script: %v", err)
	}
	script := string(content)

	for _, want := range []string{
		"groupie:catalog-refreshed",
		"filter-range:update",
		"readBounds()",
		"minInput.min = String(nextMin);",
		"maxInput.max = String(nextMax);",
		"wasFullRange",
		"currentMin === oldBounds.minLimit",
		"currentMax === oldBounds.maxLimit",
		"clamp(currentMin, nextMin, nextMax)",
		"clamp(currentMax, nextMin, nextMax)",
		"preservedMin > preservedMax",
		"render(activeInput);",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("expected refresh.js to contain %q", want)
		}
	}
	for _, forbidden := range []string{
		"aria-pressed",
		"member-button",
		".artist-card",
		"artist-grid",
		"console.log",
		"toast(",
		"insertAdjacentHTML",
		"history.pushState",
		"history.replaceState",
	} {
		if strings.Contains(script, forbidden) {
			t.Fatalf("refresh.js must not contain %q", forbidden)
		}
	}
	if got := strings.Count(script, "groupie:catalog-refreshed"); got != 1 {
		t.Fatalf("refresh.js should dispatch groupie:catalog-refreshed once, got %d", got)
	}
}
