package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/JOMS-inc/JOMSearch/go-services/internal/metrics"
)

// statusRecorder remembers the status code a handler wrote.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// Instrument counts and times every request.
//
// The route label is the ServeMux pattern (e.g. "GET /search"), not the raw
// path: ServeMux sets r.Pattern on the request it was given, so it can be
// read after next has run. Requests that matched no route (404s, bots
// probing /wp-admin) all get "unmatched", so random paths can't create new
// time series.
func Instrument(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)

		route := r.Pattern
		if route == "" {
			route = "unmatched"
		}
		metrics.HTTPRequests.WithLabelValues(route, r.Method, strconv.Itoa(rec.status)).Inc()
		metrics.HTTPDuration.WithLabelValues(route).Observe(time.Since(start).Seconds())
	})
}
