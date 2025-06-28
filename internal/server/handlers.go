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

	"groupie-tracker/internal/core"
	"groupie-tracker/internal/model"
)

/*──────────────────── типы для шаблонов ────────────────────*/

type ArtistCard struct {
	ID    int
	Name  string
	Image string
}

type PageData struct {
	Artists []ArtistCard
}

type ConcertsByLocation struct {
	Location string
	Dates    []string
}

type ArtistDetailPage struct {
	Artist   model.Artist
	Concerts []ConcertsByLocation
}

type dateWrap struct {
	raw string
	t   time.Time
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
	if r.URL.Path != "/" {
		renderError(w, http.StatusNotFound, "Страница не найдена")
		return
	}

	artists, err := core.GetArtists()
	if err != nil {
		renderError(w, 500, "Ошибка загрузки артистов")
		log.Printf("fetch artists: %v", err)
		return
	}

	cards := make([]ArtistCard, 0, len(artists))
	for _, a := range artists {
		cards = append(cards, ArtistCard{ID: a.ID, Name: a.Name, Image: a.Image})
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmplAll.ExecuteTemplate(w, "index.html", PageData{Artists: cards})
}

// детальная страница артиста
func ArtistDetailHandler(w http.ResponseWriter, r *http.Request) {
	idStr := path.Clean(strings.TrimPrefix(r.URL.Path, "/artist/"))
	id, err := strconv.Atoi(idStr)
	if err != nil || id < 1 {
		renderError(w, 404, "Такой страницы не существует.")
		return
	}

	artists, err := core.GetArtists()
	if err != nil {
		renderError(w, 500, "Ошибка загрузки артистов")
		return
	}
	var artist *model.Artist
	for _, a := range artists {
		if a.ID == id {
			artist = &a
			break
		}
	}
	if artist == nil {
		renderError(w, 404, "Артист не найден")
		return
	}

	relations, _ := core.GetRelations()
	var concerts []ConcertsByLocation
	for _, rel := range relations {
		if rel.ID != id {
			continue
		}
		for rawCity, rawDates := range rel.DatesLocations {
			city := capitalizeCity(strings.ReplaceAll(strings.ReplaceAll(rawCity, "_", " "), "-", ", "))
			dw := make([]dateWrap, 0, len(rawDates))
			for _, d := range rawDates {
				d = strings.TrimPrefix(d, "*")
				if t, err := time.Parse("02-01-2006", d); err == nil {
					dw = append(dw, dateWrap{raw: d, t: t})
				}
			}
			sort.Slice(dw, func(i, j int) bool { return dw[i].t.Before(dw[j].t) })
			dates := make([]string, len(dw))
			for i, v := range dw {
				dates[i] = v.raw
			}
			concerts = append(concerts, ConcertsByLocation{city, dates})
		}
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmplAll.ExecuteTemplate(w, "artist.html", ArtistDetailPage{*artist, concerts})
}

// принудительное обновление кеша
func RefreshHandler(w http.ResponseWriter, r *http.Request) {
	if err := core.UpdateNow(); err != nil {
		log.Printf("update failed: %v", err)
		renderError(w, 500, "Internal Server Error")
		return
	}
	w.WriteHeader(http.StatusOK)
}

// страница ошибки
func renderError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(code)
	_ = tmplAll.ExecuteTemplate(w, "error.html",
		struct {
			Code    int
			Message string
		}{code, msg})
}

/*───────────────────── image proxy ─────────────────────────*/

const imgCacheMaxAge = 60 * 60 * 24 * 7 // 7 дней

func ImgProxy(w http.ResponseWriter, r *http.Request) {
	filename := strings.TrimPrefix(r.URL.Path, "/img/")
	orig := "https://groupietrackers.herokuapp.com/api/images/" + filename

	resp, err := http.Get(orig)
	if err != nil || resp.StatusCode != http.StatusOK {
		http.NotFound(w, r)
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
