package server

import (
	"html/template"
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

// минимальный набор полей для списка на главной.
type ArtistCard struct {
	ID    int
	Name  string
	Image string
}

// данные для шаблона index.html.
type PageData struct {
	Artists []ArtistCard
}

// Для детальной страницы
type ArtistDetailPage struct {
	Artist   model.Artist
	Concerts []ConcertsByLocation
}

type ConcertsByLocation struct {
	Location string
	Dates    []string
}

type dateWrap struct {
	raw string
	t   time.Time
}

// Главная страница — список артистов
func ArtistsHandler(w http.ResponseWriter, r *http.Request) {
	artists, err := core.GetArtists()
	if err != nil {
		renderError(w, 500, "Ошибка загрузки артистов")
		log.Printf("fetch artists: %v", err)
		return
	}

	var cards []ArtistCard
	for _, a := range artists {
		cards = append(cards, ArtistCard{
			ID:    a.ID,
			Name:  a.Name,
			Image: a.Image,
		})
	}

	tmpl, err := template.ParseFS(tmplFS, "templates/index.html")
	if err != nil {
		renderError(w, 500, "Ошибка шаблона")
		log.Printf("parse template: %v", err)
		return
	}

	data := PageData{Artists: cards}
	tmpl.Execute(w, data)
}

// Детальная страница артиста
func ArtistDetailHandler(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/artist/")
	idStr = path.Clean(idStr)
	id, err := strconv.Atoi(idStr)
	if err != nil || id < 1 {
		renderError(w, 404, "Такой страницы не существует.")
		return
	}

	artists, err := core.GetArtists()
	if err != nil {
		renderError(w, 500, "Ошибка загрузки артистов")
		log.Printf("fetch artists: %v", err)
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

	// Получаем локации и даты концертов
	relationsAll, err := core.GetRelations()
	if err != nil {
		renderError(w, 500, "Ошибка загрузки связей (relations)")
		return
	}
	var concerts []ConcertsByLocation
	for _, rel := range relationsAll {
		if rel.ID == id {
			for rawCity, rawDates := range rel.DatesLocations {
				city := strings.ReplaceAll(rawCity, "_", " ") // sao_paulo → sao paulo
				city = strings.ReplaceAll(city, "-", ", ")    // santiago-chile → santiago, chile
				city = capitalizeCity(city)                   // santiago, chile → Santiago, Chile

				// даты без звёзд и в алфавитном порядке
				// собираем и сортируем
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

			break
		}
	}

	page := ArtistDetailPage{
		Artist:   *artist,
		Concerts: concerts,
	}

	tmpl, err := template.ParseFS(tmplFS, "templates/artist.html")
	if err != nil {
		renderError(w, 500, "Ошибка шаблона")
		log.Printf("parse template: %v", err)
		return
	}

	tmpl.Execute(w, page)
}

// renderError отрисовывает кастомную страницу ошибки
func renderError(w http.ResponseWriter, code int, message string) {
	tmpl, err := template.ParseFS(tmplFS, "templates/error.html")
	if err != nil {
		http.Error(w, "Ошибка шаблона", http.StatusInternalServerError)
		return
	}
	data := struct {
		Code    int
		Message string
	}{
		Code:    code,
		Message: message,
	}
	tmpl.Execute(w, data)
}

// RefreshHandler запускает принудительное обновление кеша.
func RefreshHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	go core.ForceUpdate()
	w.WriteHeader(http.StatusAccepted)
}

// capitalizeASCII делает "santiago, chile" → "Santiago, Chile".
// Работает только с буквами a-z; для кириллицы и полного Unicode
// остаётся как есть, но в нашем API всё ASCII.
func capitalizeCity(s string) string {
	// ручной whitelist для типичных аббревиатур
	abbr := map[string]struct{}{
		"uk": {}, "usa": {}, "u.s.a": {}, "uae": {},
	}
	words := strings.Fields(s) // split by space
	for i, w := range words {
		lw := strings.ToLower(w)
		if _, ok := abbr[lw]; ok {
			words[i] = strings.ToUpper(w) // UK, USA
			continue
		}
		if len(w) > 0 && 'a' <= w[0] && w[0] <= 'z' {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}
