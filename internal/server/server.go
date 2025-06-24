package server

import (
	"io/fs"
	"mime"
	"net/http"
)

func InitRoutes() *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("/artist/", ArtistDetailHandler)
	mux.HandleFunc("/api/refresh", RefreshHandler)

	// statics из embed-FS
	sub, _ := fs.Sub(staticFS, "static")
	mux.Handle("/static/", http.StripPrefix("/static/",
		http.FileServer(http.FS(sub))))

	mux.HandleFunc("/", ArtistsHandler) // последний, catch-all
	return mux
}

func init() {
	mime.AddExtensionType(".css", "text/css")
}
