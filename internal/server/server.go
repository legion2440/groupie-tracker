package server

import (
	"io/fs"
	"mime"
	"net/http"
	"strings"

	"groupie-tracker/internal/core"
	"groupie-tracker/internal/model"
)

type dependencies struct {
	updateNow     updateNowFunc
	loadCatalog   catalogLoaderFunc
	loadRelations relationLoaderFunc
	previewLookup previewLookupFunc
}

func InitRoutes() *http.ServeMux {
	return initRoutes(dependencies{
		updateNow:     core.UpdateNow,
		loadCatalog:   loadCatalog,
		loadRelations: core.GetRelations,
		previewLookup: defaultDeezerPreviewService.Preview,
	})
}

func initRoutes(deps dependencies) *http.ServeMux {
	if deps.loadRelations == nil {
		deps.loadRelations = func() ([]model.Relation, error) {
			return nil, nil
		}
	}
	if deps.previewLookup == nil {
		deps.previewLookup = defaultDeezerPreviewService.Preview
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/artist/", func(w http.ResponseWriter, r *http.Request) {
		serveLegacyArtistRedirect(w, r, deps.loadCatalog)
	})
	mux.HandleFunc("/api/refresh", func(w http.ResponseWriter, r *http.Request) {
		serveRefresh(w, r, deps.updateNow)
	})
	mux.HandleFunc("/api/search/suggestions", func(w http.ResponseWriter, r *http.Request) {
		serveSuggestions(w, r, deps.loadCatalog)
	})
	mux.HandleFunc("/api/filter-options", func(w http.ResponseWriter, r *http.Request) {
		serveFilterOptions(w, r, deps.loadCatalog)
	})
	mux.HandleFunc("/api/deezer-preview", func(w http.ResponseWriter, r *http.Request) {
		serveDeezerPreview(w, r, deps.previewLookup)
	})

	// statics из embed-FS
	sub, _ := fs.Sub(staticFS, "static")
	fileSrv := http.FileServer(http.FS(sub))
	mux.Handle("/static/", http.StripPrefix("/static/",
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// если файла нет во встроенном FS → наш шаблон 404
			if _, err := sub.Open(strings.TrimPrefix(r.URL.Path, "/static/")); err != nil {
				renderError(w, http.StatusNotFound, "Файл не найден")
				return
			}
			fileSrv.ServeHTTP(w, r) // обычная раздача
		})))

	mux.HandleFunc("/img/", ImgProxy)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			serveArtists(w, r, deps.loadCatalog)
			return
		}
		if isReservedRootPath(r.URL.Path) {
			renderError(w, http.StatusNotFound, "Страница не найдена")
			return
		}
		serveArtistSlug(w, r, deps.loadCatalog, deps.loadRelations)
	}) // последний, catch-all

	return mux
}

func isReservedRootPath(path string) bool {
	return path == "/api" ||
		path == "/static" ||
		path == "/img" ||
		path == "/artist" ||
		strings.HasPrefix(path, "/api/") ||
		strings.HasPrefix(path, "/static/") ||
		strings.HasPrefix(path, "/img/") ||
		strings.HasPrefix(path, "/artist/")
}

func init() {
	mime.AddExtensionType(".css", "text/css")
	mime.AddExtensionType(".js", "text/javascript")
	mime.AddExtensionType(".svg", "image/svg+xml")
}
