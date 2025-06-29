package server

import (
	"io/fs"
	"mime"
	"net/http"
	"strings"
)

func InitRoutes() *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("/artist/", ArtistDetailHandler)
	mux.HandleFunc("/api/refresh", RefreshHandler)

	// разкомментить для проверки теста на 500
	// mux.HandleFunc("/force500", func(w http.ResponseWriter, r *http.Request) {
	// 	renderError(w, http.StatusInternalServerError, "Ошибка 500 (тест)")
	// })

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
	mux.HandleFunc("/", ArtistsHandler) // последний, catch-all

	return mux
}

func init() {
	mime.AddExtensionType(".css", "text/css")
}
