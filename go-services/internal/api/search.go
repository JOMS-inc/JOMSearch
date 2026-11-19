package api

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/JOMS-inc/JOMSearch/go-services/internal/searchlog"
)

// searchLogger records every search. It discards by default (tests), and
// main sets a real one with SetSearchLogger.
var searchLogger = slog.New(slog.DiscardHandler)

// SetSearchLogger sets where searches are logged.
func SetSearchLogger(l *slog.Logger) { searchLogger = l }

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

		results, err := searchPages(db, q, language)
		if err != nil {
			http.Error(w, "database error", http.StatusInternalServerError)
			return
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

// searchPages finds pages in the given language that match q.
//
// It uses the pages_fts full-text index instead of `content LIKE '%q%'`.
// A LIKE with a leading % cannot use any index, so SQLite has to read every
// page. FTS5 keeps an inverted index (word -> pages containing it), so it
// only looks at pages that actually contain the words, and ranks them by
// relevance (bm25) instead of returning them in table order.
func searchPages(db *sql.DB, q, language string) ([]SearchResult, error) {
	match := ftsQuery(q)
	if match == "" {
		return nil, nil
	}

	rows, err := db.Query(`
		SELECT p.url, p.title, substr(p.content, 1, 200) AS description
		FROM pages_fts
		JOIN pages p ON p.rowid = pages_fts.rowid
		WHERE pages_fts MATCH ? AND p.language = ?
		ORDER BY pages_fts.rank`,
		match, language,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var res SearchResult
		if err := rows.Scan(&res.URL, &res.Title, &res.Description); err != nil {
			return nil, err
		}
		results = append(results, res)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Log after the query so the line includes how many results it found.
	// Searches with 0 results show what content is missing.
	searchlog.Log(searchLogger, q, language, len(results))
	return results, nil
}

// ftsQuery turns user input into a safe FTS5 query.
//
// FTS5 has its own query syntax (AND, OR, NOT, *, quotes, ...), so raw input
// like `foo"` or `NOT` would cause syntax errors. Wrapping the input in
// double quotes makes it a plain phrase, and inner quotes are escaped by
// doubling them. The trailing * makes the last word a prefix match, so
// "prog" still finds "programming", like the old LIKE search did.
func ftsQuery(q string) string {
	q = strings.TrimSpace(q)
	if q == "" {
		return ""
	}
	return `"` + strings.ReplaceAll(q, `"`, `""`) + `"*`
}
