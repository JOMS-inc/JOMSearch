package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
)

type SearchResponse struct {
	SearchResults []SearchResult `json:"search_results"`
}

// SearchHandler håndterer GET /api/search og returnerer JSON-søgeresultater.
func SearchHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		language := r.URL.Query().Get("language")
		if language == "" {
			language = "en"
		}

		var results []SearchResult
		if q != "" {
			rows, err := db.Query(
				"SELECT url, title, description FROM pages WHERE language = ? AND content LIKE ?",
				language, "%"+q+"%",
			)
			if err != nil {
				http.Error(w, "database error", http.StatusInternalServerError)
				return
			}
			defer rows.Close()

			for rows.Next() {
				var res SearchResult
				if err := rows.Scan(&res.URL, &res.Title, &res.Description); err != nil {
					http.Error(w, "database error", http.StatusInternalServerError)
					return
				}
				results = append(results, res)
			}
			if err := rows.Err(); err != nil {
				http.Error(w, "database error", http.StatusInternalServerError)
				return
			}
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(SearchResponse{SearchResults: results}); err != nil {
			http.Error(w, "encoding error", http.StatusInternalServerError)
		}
	}
}

// RegisterSearchRoutes wirer /api/search op.
func RegisterSearchRoutes(mux *http.ServeMux, db *sql.DB) {
	mux.HandleFunc("GET /api/search", SearchHandler(db))
}