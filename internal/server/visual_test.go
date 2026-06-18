package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// CSS раздаётся как статика
func TestCSSServed(t *testing.T) {
	mux := initRoutes(testDependenciesWithCatalog(testCatalog(), nil))

	req := httptest.NewRequest(http.MethodGet, "/static/style.css", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	if ct := res.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/css") {
		t.Fatalf("unexpected Content-Type %s", ct)
	}
}

func TestSearchJSServed(t *testing.T) {
	mux := initRoutes(testDependenciesWithCatalog(testCatalog(), nil))

	req := httptest.NewRequest(http.MethodGet, "/static/search.js", nil)
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

func TestResultsJSServed(t *testing.T) {
	mux := initRoutes(testDependenciesWithCatalog(testCatalog(), nil))

	req := httptest.NewRequest(http.MethodGet, "/static/results.js", nil)
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

func TestFaviconStaticFilesServed(t *testing.T) {
	mux := initRoutes(testDependenciesWithCatalog(testCatalog(), nil))

	tests := []struct {
		path        string
		contentType string
		want        []string
		forbidden   []string
	}{
		{
			path:        "/static/favicon.svg",
			contentType: "image/svg+xml",
			want:        []string{`id="vinyl"`},
			forbidden:   []string{"animateTransform"},
		},
		{
			path:        "/static/favicon.js",
			contentType: "javascript",
			want: []string{
				"data:image/svg+xml,",
				"prefers-reduced-motion: reduce",
				"window.setInterval(tick, frameMs)",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()

			mux.ServeHTTP(rec, req)

			res := rec.Result()
			defer res.Body.Close()

			if res.StatusCode != http.StatusOK {
				t.Fatalf("expected 200, got %d", res.StatusCode)
			}
			if ct := res.Header.Get("Content-Type"); !strings.Contains(ct, tt.contentType) {
				t.Fatalf("unexpected Content-Type %s", ct)
			}
			body, err := io.ReadAll(res.Body)
			if err != nil {
				t.Fatalf("read response body: %v", err)
			}
			for _, want := range tt.want {
				if !strings.Contains(string(body), want) {
					t.Fatalf("expected static file to contain %q", want)
				}
			}
			for _, forbidden := range tt.forbidden {
				if strings.Contains(string(body), forbidden) {
					t.Fatalf("static file must not contain %q", forbidden)
				}
			}
		})
	}
}

func TestFaviconMarkup(t *testing.T) {
	pages := []struct {
		name string
		body string
	}{
		{name: "root", body: renderRootBody(t, "/")},
		{name: "artist", body: renderArtistDetailTemplate(t)},
		{name: "error", body: renderErrorTemplate(t)},
	}

	for _, page := range pages {
		t.Run(page.name, func(t *testing.T) {
			for _, want := range []string{
				`<link rel="icon" type="image/svg+xml" href="/static/favicon.svg">`,
				`<script src="/static/favicon.js" defer></script>`,
			} {
				if !strings.Contains(page.body, want) {
					t.Fatalf("expected favicon markup %q", want)
				}
			}
		})
	}
}

func TestCSSFinalFilterAndEmptyStateContracts(t *testing.T) {
	content, err := staticFS.ReadFile("static/style.css")
	if err != nil {
		t.Fatalf("read style.css: %v", err)
	}
	css := string(content)

	for _, want := range []string{
		".artist-empty",
		".artist-empty__title",
		".artist-empty__text",
		".artist-empty__actions",
		".member-button:has(.member-button__input:checked)",
		".location-option[hidden]",
		".location-option:has(input:focus-visible)",
		".filter-action:focus-visible",
		"grid-template-columns: 288px minmax(0, 1fr);",
		"width: 288px;",
		"@media (min-width: 720px)",
		"@media (min-width: 1024px)",
		"@media (max-width: 1023.98px)",
		"overflow-y: auto;",
	} {
		if !strings.Contains(css, want) {
			t.Fatalf("expected CSS to contain %q", want)
		}
	}
	for _, forbidden := range []string{
		".artist-count",
		"member-button[aria-pressed",
		".filter-actions",
		".filter-action--secondary",
		".filter-action--quiet",
		".location-list::-webkit-scrollbar",
		"max-height: 320px",
	} {
		if strings.Contains(css, forbidden) {
			t.Fatalf("CSS must not contain dead selector %q", forbidden)
		}
	}
	locationList := cssBlockForTest(t, css, ".location-list")
	for _, forbidden := range []string{"overflow-y: auto", "overflow: auto", "max-height: 320px"} {
		if strings.Contains(locationList, forbidden) {
			t.Fatalf("location list must not own vertical scrolling via %q: %s", forbidden, locationList)
		}
	}
	tablet := cssMediaBlockContainingForTest(t, css, "@media (min-width: 720px)", "grid-template-columns: 288px minmax(0, 1fr);")
	for _, want := range []string{
		"grid-template-columns: 288px minmax(0, 1fr);",
		"width: min(calc(100% - 40px), var(--container-max));",
		"align-self: start;",
	} {
		if !strings.Contains(tablet, want) {
			t.Fatalf("expected tablet layout contract %q in %s", want, tablet)
		}
	}
	if strings.Contains(tablet, "repeat(2, minmax(0, 1fr))") {
		t.Fatalf("tablet layout should keep results as one card column beside sidebar: %s", tablet)
	}
	desktop := cssMediaBlockContainingForTest(t, css, "@media (min-width: 1024px)", "grid-template-columns: 288px minmax(0, 1fr);")
	for _, want := range []string{
		"grid-template-columns: 288px minmax(0, 1fr);",
		"width: min(calc(100% - 128px), var(--container-max));",
		"position: sticky;",
		"overflow-y: auto;",
		"grid-template-columns: repeat(2, minmax(0, 1fr));",
	} {
		if !strings.Contains(desktop, want) {
			t.Fatalf("expected desktop layout contract %q in %s", want, desktop)
		}
	}
	artistDetailTablet := cssMediaBlockContainingForTest(t, css, "@media (max-width: 1023px)", "width: min(380px, 100%);")
	for _, want := range []string{
		"grid-template-columns: 1fr;",
		"justify-self: start;",
		"width: min(380px, 100%);",
	} {
		if !strings.Contains(artistDetailTablet, want) {
			t.Fatalf("expected artist detail tablet layout contract %q in %s", want, artistDetailTablet)
		}
	}
}

func cssBlockForTest(t *testing.T, css string, selector string) string {
	t.Helper()
	start := strings.Index(css, selector+" {")
	if start < 0 {
		t.Fatalf("selector %q not found", selector)
	}
	end := strings.Index(css[start:], "\n}")
	if end < 0 {
		t.Fatalf("selector %q block not closed", selector)
	}
	return css[start : start+end+2]
}

func cssMediaBlockContainingForTest(t *testing.T, css string, marker string, required string) string {
	t.Helper()
	offset := 0
	for {
		start := strings.Index(css[offset:], marker)
		if start < 0 {
			t.Fatalf("media marker %q containing %q not found", marker, required)
		}
		start += offset
		next := strings.Index(css[start+len(marker):], "\n@media ")
		if next < 0 {
			block := css[start:]
			if strings.Contains(block, required) {
				return block
			}
			t.Fatalf("media marker %q containing %q not found", marker, required)
		}
		block := css[start : start+len(marker)+next]
		if strings.Contains(block, required) {
			return block
		}
		offset = start + len(marker)
	}
}

// Проверяет, что для несуществующего маршрута сервер возвращает статус 404 и фирменную страницу ошибки.
func Test404Template(t *testing.T) {
	mux := initRoutes(testDependenciesWithCatalog(testCatalog(), nil))

	req := httptest.NewRequest(http.MethodGet, "/no/such/page", nil)
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
		`src="/static/groupie_tracker_404_neon.gif"`,
		`error-404-home`,
		`error-home-link`,
		`TO MAIN PAGE`,
		`class="footer"`,
	} {
		if !strings.Contains(bodyText, want) {
			t.Fatalf("expected custom 404 page to contain %q, got: %s", want, bodyText)
		}
	}
	for _, old := range []string{
		"Ошибка 404",
		"Страница не найдена",
		"← На главную",
	} {
		if strings.Contains(bodyText, old) {
			t.Fatalf("expected custom 404 page to omit old content %q, got: %s", old, bodyText)
		}
	}
}
