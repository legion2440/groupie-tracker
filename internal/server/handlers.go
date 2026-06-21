package server

import (
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"groupie-tracker/internal/catalog"
	"groupie-tracker/internal/core"
	"groupie-tracker/internal/geocode"
	"groupie-tracker/internal/model"
)

/*──────────────────── типы для шаблонов ────────────────────*/

type ArtistCard struct {
	ID    int
	Slug  string
	Name  string
	Image string
}

type PageData struct {
	Artists []ArtistCard
	Query   QueryState
	Filters FilterOptionsView
}

type QueryState struct {
	SearchText     string
	SearchType     string
	HasSearch      bool
	HasTypedSearch bool

	CreationFrom string
	CreationTo   string

	AlbumFrom string
	AlbumTo   string

	MemberCounts []string
	Locations    []string
}

type ConcertsByLocation struct {
	Location  string
	Dates     []string
	Latitude  float64
	Longitude float64
}

type ArtistDetailPage struct {
	Artist   model.Artist
	Concerts []ConcertsByLocation
}

type dateWrap struct {
	raw string
	t   time.Time
}

type concertLocationSortItem struct {
	concert            ConcertsByLocation
	normalizedLocation string
	firstDate          time.Time
	hasDate            bool
}

/*──────────────────── шаблоны (init) ───────────────────────*/

var funcMap = template.FuncMap{
	"base": path.Base, // queen.jpeg ← https://…/queen.jpeg
}

var tmplAll *template.Template

func parseTemplates() (*template.Template, error) {
	return template.New("").
		Funcs(funcMap).
		ParseFS(tmplFS, "templates/*.html")
}

func init() {
	var err error
	tmplAll, err = parseTemplates()
	if err != nil {
		log.Fatalf("parse templates: %v", err)
	}
}

/*────────────────────────── хендлеры ───────────────────────*/

// главная страница
func ArtistsHandler(w http.ResponseWriter, r *http.Request) {
	serveArtists(w, r, loadCatalog)
}

func serveArtists(w http.ResponseWriter, r *http.Request, loadCatalog catalogLoaderFunc) {
	if r.URL.Path != "/" {
		renderError(w, http.StatusNotFound, "Страница не найдена")
		return
	}

	catalogData, err := loadCatalog()
	if err != nil {
		log.Printf("load catalog: %v", err)
		renderError(w, http.StatusInternalServerError, "Ошибка загрузки артистов")
		return
	}
	filterOptions := catalog.BuildFilterOptions(catalogData)

	criteria, err := parseCriteria(r.URL.Query())
	if err != nil {
		log.Printf("parse criteria: %v", err)
		renderError(w, http.StatusBadRequest, "Некорректные параметры поиска или фильтров")
		return
	}

	artists, err := catalog.Filter(catalogData, criteria)
	if err != nil {
		log.Printf("filter catalog: %v", err)
		renderError(w, http.StatusBadRequest, "Некорректные параметры поиска или фильтров")
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmplAll.ExecuteTemplate(w, "index.html", PageData{
		Artists: artistCardsFromEntries(artists),
		Query:   queryStateFromCriteria(criteria),
		Filters: filterOptionsView(filterOptions, criteria),
	})
}

func artistCardsFromEntries(artists []catalog.ArtistEntry) []ArtistCard {
	cards := make([]ArtistCard, 0, len(artists))
	for _, artist := range artists {
		cards = append(cards, ArtistCard{
			ID:    artist.ID,
			Slug:  artist.Slug,
			Name:  artist.Name,
			Image: artist.Image,
		})
	}
	return cards
}

func queryStateFromCriteria(criteria catalog.Criteria) QueryState {
	hasSearch := hasActiveSearch(criteria)
	state := QueryState{
		SearchText:     criteria.SearchText,
		SearchType:     string(criteria.SearchField),
		HasSearch:      hasSearch,
		HasTypedSearch: hasSearch && criteria.SearchField != catalog.SearchAny && criteria.SearchField != "",
		MemberCounts:   memberQueryValues(criteria),
		Locations:      append([]string(nil), criteria.Locations...),
	}

	if criteria.CreationFrom != nil {
		state.CreationFrom = strconv.Itoa(*criteria.CreationFrom)
	}
	if criteria.CreationTo != nil {
		state.CreationTo = strconv.Itoa(*criteria.CreationTo)
	}
	if criteria.FirstAlbumFrom != nil {
		state.AlbumFrom = criteria.FirstAlbumFrom.Format(albumQueryLayout)
	}
	if criteria.FirstAlbumTo != nil {
		state.AlbumTo = criteria.FirstAlbumTo.Format(albumQueryLayout)
	}

	return state
}

func memberQueryValues(criteria catalog.Criteria) []string {
	values := make([]string, 0, len(criteria.MemberCounts)+1)
	for _, count := range criteria.MemberCounts {
		values = append(values, strconv.Itoa(count))
	}
	if criteria.MinMemberCount != nil && *criteria.MinMemberCount == 8 {
		values = append(values, "8+")
	}
	return values
}

func hasActiveSearch(criteria catalog.Criteria) bool {
	return catalog.NormalizeSearchText(criteria.SearchText) != ""
}

// детальная страница артиста
func ArtistDetailHandler(w http.ResponseWriter, r *http.Request) {
	serveLegacyArtistRedirect(w, r, loadCatalog)
}

func serveLegacyArtistRedirect(w http.ResponseWriter, r *http.Request, loadCatalog catalogLoaderFunc) {
	idStr := path.Clean(strings.TrimPrefix(r.URL.Path, "/artist/"))
	id, err := strconv.Atoi(idStr)
	if err != nil || id < 1 {
		renderError(w, 404, "Страницы не существует")
		return
	}

	catalogData, err := loadCatalog()
	if err != nil {
		renderError(w, 500, "Ошибка загрузки артистов")
		return
	}
	slug, ok := catalogData.ArtistSlugByID[id]
	if !ok || slug == "" {
		renderError(w, 404, "Артист не найден")
		return
	}

	http.Redirect(w, r, "/"+slug, http.StatusMovedPermanently)
}

func serveArtistSlug(
	w http.ResponseWriter,
	r *http.Request,
	loadCatalog catalogLoaderFunc,
	loadRelations relationLoaderFunc,
	lookupCoordinate coordinateLookupFunc,
) {
	slug := strings.Trim(path.Clean(r.URL.Path), "/")
	if slug == "" || strings.Contains(slug, "/") {
		renderError(w, 404, "Страницы не существует")
		return
	}

	catalogData, err := loadCatalog()
	if err != nil {
		renderError(w, 500, "Ошибка загрузки артистов")
		return
	}
	id, ok := catalogData.ArtistIDBySlug[slug]
	if !ok {
		renderError(w, 404, "Артист не найден")
		return
	}
	entry, ok := catalogData.ArtistByID[id]
	if !ok {
		renderError(w, 404, "Артист не найден")
		return
	}

	renderArtistDetail(w, entry, loadRelations, lookupCoordinate)
}

type relationLoaderFunc func() ([]model.Relation, error)
type coordinateLookupFunc func(rawLocation string) (geocode.Coordinate, bool)

func renderArtistDetail(
	w http.ResponseWriter,
	artist catalog.ArtistEntry,
	loadRelations relationLoaderFunc,
	lookupCoordinate coordinateLookupFunc,
) {
	relations, err := loadRelations()
	if err != nil {
		log.Printf("load artist relations for %d: %v", artist.ID, err)
		renderError(w, http.StatusInternalServerError, "Ошибка загрузки концертов")
		return
	}

	concerts, err := buildConcertsByLocation(artist.ID, relations, lookupCoordinate)
	if err != nil {
		log.Printf("build artist concerts for %d: %v", artist.ID, err)
		renderError(w, http.StatusInternalServerError, "Ошибка загрузки координат концертов")
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmplAll.ExecuteTemplate(w, "artist.html", ArtistDetailPage{artistDetailModel(artist), concerts})
}

func buildConcertsByLocation(
	artistID int,
	relations []model.Relation,
	lookupCoordinate coordinateLookupFunc,
) ([]ConcertsByLocation, error) {
	if lookupCoordinate == nil {
		return nil, fmt.Errorf("coordinate lookup is not configured")
	}

	items := make([]concertLocationSortItem, 0)
	for _, rel := range relations {
		if rel.ID != artistID {
			continue
		}
		for rawCity, rawDates := range rel.DatesLocations {
			city := capitalizeCity(strings.ReplaceAll(strings.ReplaceAll(rawCity, "_", " "), "-", ", "))
			dw := make([]dateWrap, 0, len(rawDates))
			for _, d := range rawDates {
				d = strings.TrimPrefix(d, "*")
				t, err := time.ParseInLocation("02-01-2006", d, time.UTC)
				if err != nil {
					return nil, fmt.Errorf("parse concert date %q for %s: %w", d, rawCity, err)
				}
				dw = append(dw, dateWrap{raw: d, t: t})
			}
			sort.Slice(dw, func(i, j int) bool { return dw[i].t.Before(dw[j].t) })
			dates := make([]string, len(dw))
			for i, v := range dw {
				dates[i] = v.raw
			}

			coordinate, ok := lookupCoordinate(rawCity)
			if !ok {
				return nil, fmt.Errorf("missing coordinates for %s (%s)", city, rawCity)
			}

			item := concertLocationSortItem{
				concert: ConcertsByLocation{
					Location:  city,
					Dates:     dates,
					Latitude:  coordinate.Latitude,
					Longitude: coordinate.Longitude,
				},
				normalizedLocation: geocode.NormalizeLocationKey(rawCity),
			}
			if len(dw) > 0 {
				item.firstDate = dw[0].t
				item.hasDate = true
			}
			items = append(items, item)
		}
	}

	sort.Slice(items, func(i, j int) bool {
		left := items[i]
		right := items[j]
		if left.hasDate != right.hasDate {
			return left.hasDate
		}
		if left.hasDate && !left.firstDate.Equal(right.firstDate) {
			return left.firstDate.Before(right.firstDate)
		}
		return left.normalizedLocation < right.normalizedLocation
	})

	concerts := make([]ConcertsByLocation, len(items))
	for i, item := range items {
		concerts[i] = item.concert
	}
	return concerts, nil
}

func artistDetailModel(artist catalog.ArtistEntry) model.Artist {
	return model.Artist{
		ID:           artist.ID,
		Image:        artist.Image,
		Name:         artist.Name,
		Members:      append([]string(nil), artist.Members...),
		CreationDate: artist.CreationYear,
		FirstAlbum:   artist.FirstAlbumRaw,
	}
}

// принудительное обновление кеша
type updateNowFunc func() error

func serveRefresh(w http.ResponseWriter, r *http.Request, updateNow updateNowFunc) {
	if r.Method != http.MethodPost {
		renderError(w, http.StatusBadRequest, "Неверный HTTP-метод (ожидается POST)")
		return
	}
	if err := updateNow(); err != nil {
		log.Printf("update failed: %v", err)
		renderError(w, http.StatusInternalServerError, "Internal Server Error")
		return
	}
	w.WriteHeader(http.StatusOK)
}

func RefreshHandler(w http.ResponseWriter, r *http.Request) {
	serveRefresh(w, r, core.UpdateNow)
}

// страница ошибки
func renderError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(code)
	if code == http.StatusNotFound {
		_ = tmplAll.ExecuteTemplate(w, "404.html",
			struct {
				Query QueryState
			}{Query: QueryState{}})
		return
	}
	_ = tmplAll.ExecuteTemplate(w, "error.html",
		struct {
			Code    int
			Message string
			Query   QueryState
		}{Code: code, Message: msg, Query: QueryState{}})
}

/*───────────────────── image proxy ─────────────────────────*/

const imgCacheMaxAge = 60 * 60 * 24 * 7 // 7 дней

func ImgProxy(w http.ResponseWriter, r *http.Request) {
	filename := strings.TrimPrefix(r.URL.Path, "/img/")
	orig := "https://groupietrackers.herokuapp.com/api/images/" + filename

	resp, err := http.Get(orig)
	if err != nil || resp.StatusCode != http.StatusOK {
		renderError(w, http.StatusNotFound, "Картинка не найдена")
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Cache-Control",
		fmt.Sprintf("public, max-age=%d", imgCacheMaxAge))
	w.Header().Set("Content-Type", "image/jpeg")
	io.Copy(w, resp.Body)
}

/*───────────────────── helpers ─────────────────────────────*/

// santiago, chile → Santiago, Chile
func capitalizeCity(s string) string {
	abbr := map[string]struct{}{"uk": {}, "usa": {}, "u.s.a": {}, "uae": {}}
	words := strings.Fields(s)
	for i, w := range words {
		lw := strings.ToLower(w)
		if _, ok := abbr[lw]; ok {
			words[i] = strings.ToUpper(w)
			continue
		}
		if len(w) > 0 && 'a' <= w[0] && w[0] <= 'z' {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}
