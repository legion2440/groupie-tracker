package server

import (
	"net/http"
)

func InitRoutes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/artist/", ArtistDetailHandler)
	mux.HandleFunc("/", ArtistsHandler)
	mux.HandleFunc("/api/refresh", RefreshHandler)
	mux.Handle("/static/", http.StripPrefix("/static/",
		http.FileServer(http.Dir("static"))))
	return mux
}
