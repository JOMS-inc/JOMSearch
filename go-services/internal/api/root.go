package api

import (
	"database/sql"
	"net/http"

	"github.com/JOMS-inc/JOMSearch/go-services/internal/templates"
)

var searchTmpl = templates.Page("search.html")

type SearchResult struct {
	URL         string `json:"url"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

type SearchPageData struct {
	templates.BaseData
	Query         string
	SearchResults []SearchResult
}

// Root håndterer GET / og servér HTML-søgesiden.
func Root(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		language := r.URL.Query().Get("language")
		if language == "" {
			language = "en"
		}

		results, err := searchPages(db, q, language, "html")
		if err != nil {
			http.Error(w, "database error", http.StatusInternalServerError)
			return
		}

		data := SearchPageData{
			BaseData:      templates.BaseData{User: currentUser(r)},
			Query:         q,
			SearchResults: results,
		}
		if err := searchTmpl.ExecuteTemplate(w, "layout.html", data); err != nil {
			http.Error(w, "template error", http.StatusInternalServerError)
		}
	}
}
