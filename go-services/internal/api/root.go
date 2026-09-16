package api

import (
	"database/sql"
	"html/template"
	"net/http"
)

var searchTmpl = template.Must(template.ParseFiles("templates/layout.html", "templates/search.html"))

type SearchResult struct {
	URL         string `json:"url"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

type SearchPageData struct {
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

		data := SearchPageData{Query: q, SearchResults: results}
		if err := searchTmpl.ExecuteTemplate(w, "layout.html", data); err != nil {
			http.Error(w, "template error", http.StatusInternalServerError)
		}
	}
}