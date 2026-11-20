package api

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"
	"unicode"

	"github.com/JOMS-inc/JOMSearch/go-services/internal/metrics"
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

		results, err := searchPages(db, q, language, "api")
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

// searchPages finds pages in the given language that match q, best first.
// source ("html" or "api") is only used to label the search metrics.
//
// It uses the GIN index on pages.search (see db/migrations) instead of
// `content LIKE '%q%'`. A LIKE with a leading % cannot use an index, so the
// database has to read every page. The GIN index maps each word to the
// pages containing it, so only matching pages are read, and ts_rank orders
// them by relevance (title matches weigh more than content matches).
func searchPages(db *sql.DB, q, language, source string) ([]SearchResult, error) {
	match := tsQuery(q)
	if match == "" {
		return nil, nil
	}

	start := time.Now()
	// The query is stemmed with the same language config as the pages
	// (danish or english), so "programming" and "programs" both find "program".
	rows, err := db.Query(`
		SELECT url, title, substr(content, 1, 200) AS description
		FROM pages,
		     to_tsquery(CASE $2 WHEN 'da' THEN 'danish'::regconfig ELSE 'english'::regconfig END, $1) AS q
		WHERE language = $2 AND search @@ q
		ORDER BY ts_rank(search, q) DESC, title`,
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
	metrics.SearchDuration.WithLabelValues(language).Observe(time.Since(start).Seconds())
	metrics.SearchResults.Observe(float64(len(results)))
	metrics.Searches.WithLabelValues(language, source, metrics.Outcome(len(results))).Inc()
	return results, nil
}

// maxQueryWords caps how many words of a search are used.
const maxQueryWords = 10

// tsQuery turns user input into a safe to_tsquery string.
//
// to_tsquery has its own syntax (& | ! <-> : * parentheses), so raw input
// like `foo(` would be a syntax error. Keeping only runs of letters and
// digits removes all of it. Each word gets :* (prefix match), so "prog"
// still finds "programming", and words are joined with & (all must match).
// Example: `Go (prog` -> `go:* & prog:*`.
func tsQuery(q string) string {
	words := strings.FieldsFunc(strings.ToLower(q), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	if len(words) > maxQueryWords {
		words = words[:maxQueryWords]
	}
	for i, w := range words {
		words[i] = w + ":*"
	}
	return strings.Join(words, " & ")
}
