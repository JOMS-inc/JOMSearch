package crawler

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/JOMS-inc/JOMSearch/go-services/internal/db"
	"github.com/JOMS-inc/JOMSearch/go-services/internal/searchlog"
	"github.com/JOMS-inc/JOMSearch/go-services/internal/testdb"
)

// fakeWikipedia answers like the real API. Pages come back out of order,
// as the real API does, to check that the crawler sorts by index.
const fakeResponse = `{"batchcomplete":true,"query":{"pages":[
	{"pageid":2,"title":"Kubernetes Operator","index":2,"fullurl":"https://en.wikipedia.org/wiki/Kubernetes_Operator","extract":"An operator extends Kubernetes."},
	{"pageid":1,"title":"Kubernetes","index":1,"fullurl":"https://en.wikipedia.org/wiki/Kubernetes","extract":"Kubernetes is a container orchestration system."},
	{"pageid":3,"title":"Empty","index":3,"fullurl":"https://en.wikipedia.org/wiki/Empty","extract":"  "}
]}}`

func newCrawler(t *testing.T, handler http.HandlerFunc) (*Crawler, *atomic.Int32) {
	t.Helper()
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		handler(w, r)
	}))
	t.Cleanup(srv.Close)

	conn := testdb.New(t)
	if err := db.Migrate(conn); err != nil {
		t.Fatal(err)
	}
	return &Crawler{
		DB:           conn,
		HTTP:         srv.Client(),
		APIURL:       func(string) string { return srv.URL },
		UserAgent:    "test",
		Log:          slog.New(slog.NewTextHandler(io.Discard, nil)),
		PagesPerTerm: 5,
		Refresh:      time.Hour,
	}, &calls
}

func TestRunStoresSearchablePages(t *testing.T) {
	c, calls := newCrawler(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("gsrsearch"); got != "kubernetes" {
			t.Errorf("searched for %q", got)
		}
		if r.Header.Get("User-Agent") != "test" {
			t.Error("User-Agent not sent")
		}
		_, _ = io.WriteString(w, fakeResponse)
	})

	terms := []searchlog.Term{{Query: "kubernetes", Language: "en", Searches: 3}}
	st, err := c.Run(context.Background(), terms, 10)
	if err != nil {
		t.Fatal(err)
	}
	if st.Terms != 1 || st.Pages != 2 {
		t.Errorf("stats = %+v, want 1 term, 2 pages (the empty extract is skipped)", st)
	}

	// The new pages are in the full-text index, so search finds them.
	var n int
	if err := c.DB.QueryRow(`SELECT count(*) FROM pages WHERE search @@ to_tsquery('english', 'kubernetes')`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("FTS finds %d kubernetes pages, want 2", n)
	}

	// Running again skips the term: it was crawled less than Refresh ago.
	st, err = c.Run(context.Background(), terms, 10)
	if err != nil {
		t.Fatal(err)
	}
	if st.Skipped != 1 || calls.Load() != 1 {
		t.Errorf("second run: stats = %+v, requests = %d; want skipped, no new request", st, calls.Load())
	}
}

func TestRunSkipsInvalidTerms(t *testing.T) {
	c, calls := newCrawler(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, fakeResponse)
	})
	terms := []searchlog.Term{
		{Query: "kubernetes", Language: "evil.example"}, // would end up in the hostname
		{Query: "x", Language: "en"},                    // too short
	}
	st, err := c.Run(context.Background(), terms, 10)
	if err != nil {
		t.Fatal(err)
	}
	if st.Skipped != 2 || calls.Load() != 0 {
		t.Errorf("stats = %+v, requests = %d; want both skipped, no requests", st, calls.Load())
	}
}

func TestRunRetriesFailedTermsNextTime(t *testing.T) {
	fail := true
	c, _ := newCrawler(t, func(w http.ResponseWriter, r *http.Request) {
		if fail {
			http.Error(w, "down", http.StatusServiceUnavailable)
			return
		}
		_, _ = io.WriteString(w, fakeResponse)
	})
	terms := []searchlog.Term{{Query: "kubernetes", Language: "en"}}

	st, err := c.Run(context.Background(), terms, 10)
	if err != nil {
		t.Fatal(err)
	}
	if st.Failed != 1 {
		t.Fatalf("stats = %+v, want 1 failed", st)
	}

	fail = false
	st, err = c.Run(context.Background(), terms, 10)
	if err != nil {
		t.Fatal(err)
	}
	if st.Terms != 1 || st.Pages != 2 {
		t.Errorf("retry: stats = %+v, want the term crawled", st)
	}
}

func TestRunStopsAtMaxTerms(t *testing.T) {
	c, calls := newCrawler(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"query":{"pages":[]}}`)
	})
	terms := []searchlog.Term{
		{Query: "aa", Language: "en"}, {Query: "bb", Language: "en"}, {Query: "cc", Language: "en"},
	}
	if _, err := c.Run(context.Background(), terms, 2); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 2 {
		t.Errorf("made %d requests, want 2", calls.Load())
	}
}
