package api

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"

	"github.com/JOMS-inc/JOMSearch/go-services/internal/metrics"
)

// ClickHandler handles GET /go?url=...&rank=N&language=xx.
//
// Search result links point here instead of straight at Wikipedia, so we can
// count which results people actually open (click-through rate). It only
// redirects to URLs that are in the pages table; anything else is refused,
// so the endpoint can't be used as an open redirect to a phishing site.
func ClickHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Redirect to the URL as stored in our database, never to the raw
		// query parameter: only pages we indexed can be redirect targets.
		var target, language string
		err := db.QueryRow(`SELECT url, language FROM pages WHERE url = $1`, r.URL.Query().Get("url")).
			Scan(&target, &language)
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "unknown url", http.StatusBadRequest)
			return
		} else if err != nil {
			http.Error(w, "database error", http.StatusInternalServerError)
			return
		}

		rank, _ := strconv.Atoi(r.URL.Query().Get("rank")) // bad rank -> "unknown" bucket
		metrics.ResultClicks.WithLabelValues(language, metrics.RankBucket(rank)).Inc()
		http.Redirect(w, r, target, http.StatusFound)
	}
}
