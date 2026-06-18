package server

import (
	"encoding/json"
	"log"
	"net/http"

	"groupie-tracker/internal/catalog"
)

type suggestionResponse struct {
	Value string `json:"value"`
	Label string `json:"label"`
	Type  string `json:"type"`
}

type apiErrorResponse struct {
	Error string `json:"error"`
}

func serveSuggestions(w http.ResponseWriter, r *http.Request, loadCatalog catalogLoaderFunc) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		writeJSON(w, http.StatusMethodNotAllowed, apiErrorResponse{Error: "method not allowed"})
		return
	}

	query := r.URL.Query().Get("q")
	if searchTextTooLong(query) {
		writeJSON(w, http.StatusBadRequest, apiErrorResponse{Error: "query too long"})
		return
	}

	catalogData, err := loadCatalog()
	if err != nil {
		log.Printf("load catalog for suggestions: %v", err)
		writeJSON(w, http.StatusInternalServerError, apiErrorResponse{Error: "internal server error"})
		return
	}

	suggestions := catalog.Suggest(catalogData, query)
	response := make([]suggestionResponse, 0, len(suggestions))
	for _, suggestion := range suggestions {
		response = append(response, suggestionResponse{
			Value: suggestion.Value,
			Label: suggestion.Label,
			Type:  string(suggestion.Field),
		})
	}
	writeJSON(w, http.StatusOK, response)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("encode json response: %v", err)
	}
}
