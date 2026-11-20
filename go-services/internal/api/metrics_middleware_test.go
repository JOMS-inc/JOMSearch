package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/JOMS-inc/JOMSearch/go-services/internal/metrics"
)

func TestInstrumentLabelsByRoutePattern(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /about", func(w http.ResponseWriter, r *http.Request) {})
	h := Instrument(mux)

	about := metrics.HTTPRequests.WithLabelValues("GET /about", "GET", "200")
	unmatched := metrics.HTTPRequests.WithLabelValues("unmatched", "GET", "404")
	a0, u0 := testutil.ToFloat64(about), testutil.ToFloat64(unmatched)

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/about", nil))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/wp-admin/x.php", nil))

	if got := testutil.ToFloat64(about) - a0; got != 1 {
		t.Errorf(`"GET /about" count increased by %v, want 1`, got)
	}
	// Unknown paths share one label, so bots can't create new time series.
	if got := testutil.ToFloat64(unmatched) - u0; got != 1 {
		t.Errorf(`"unmatched" count increased by %v, want 1`, got)
	}
}
