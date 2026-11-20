// Package metrics defines the Prometheus metrics the app exposes on /metrics.
//
// They are chosen to answer questions about how people use JOMSearch and
// whether it helps them (see monitoring.md), not only whether it is up.
//
// Labels only ever take a small, fixed set of values. The search text itself
// is never a label: every distinct query would create a new time series and
// Prometheus would run out of memory. Top queries are in the search log instead.
package metrics

import (
	"net/http"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// Searches counts searches. outcome is "hit" or "zero" (no results),
	// source is "html" (the search page) or "api" (/api/search).
	Searches = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "jomsearch_searches_total",
		Help: "Searches, by language, source (html/api) and outcome (hit/zero).",
	}, []string{"language", "source", "outcome"})

	// SearchResults is how many results each search returned.
	SearchResults = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "jomsearch_search_results",
		Help:    "Number of results per search.",
		Buckets: []float64{0, 1, 3, 5, 10, 25, 50},
	})

	// SearchDuration is how long the database search took.
	SearchDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "jomsearch_search_duration_seconds",
		Help:    "Time spent running the search query against the database.",
		Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5},
	}, []string{"language"})

	// ResultClicks counts clicks on search results, by result position.
	ResultClicks = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "jomsearch_result_clicks_total",
		Help: "Clicks on search results, by language and rank bucket.",
	}, []string{"language", "rank"})

	// Logins counts login attempts. result is "success" or "failure".
	Logins = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "jomsearch_logins_total",
		Help: "Login attempts, by result (success/failure).",
	}, []string{"result"})

	// Registrations counts sign-up attempts. result is "success",
	// "invalid" (missing fields, passwords differ) or "taken".
	Registrations = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "jomsearch_registrations_total",
		Help: "Registration attempts, by result (success/invalid/taken).",
	}, []string{"result"})

	// HTTPRequests counts every request by route pattern, method and status.
	HTTPRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "jomsearch_http_requests_total",
		Help: "HTTP requests, by route pattern, method and status code.",
	}, []string{"route", "method", "status"})

	// HTTPDuration is how long requests took, by route pattern.
	HTTPDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "jomsearch_http_request_duration_seconds",
		Help:    "HTTP request duration, by route pattern.",
		Buckets: prometheus.DefBuckets,
	}, []string{"route"})
)

// Outcome returns the outcome label for a search with n results.
func Outcome(n int) string {
	if n == 0 {
		return "zero"
	}
	return "hit"
}

// RankBucket groups a 1-based result position into a few label values, so a
// long result list can't create one time series per position.
func RankBucket(rank int) string {
	switch {
	case rank >= 1 && rank <= 3:
		return strconv.Itoa(rank)
	case rank >= 4 && rank <= 10:
		return "4-10"
	case rank > 10:
		return "11+"
	}
	return "unknown"
}

// Handler serves the metrics in Prometheus' text format.
func Handler() http.Handler { return promhttp.Handler() }
