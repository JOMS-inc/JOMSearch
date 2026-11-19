// Package crawler fills the pages table with Wikipedia articles for the
// things users search for (see package searchlog).
//
// It uses Wikipedia's API instead of scraping HTML: one request per search
// term returns the best matching articles with their intro as plain text.
// That is far less load on Wikipedia than downloading whole pages, it is what
// their bot policy asks for, and it does not break when the page layout
// changes. New pages are picked up by the full-text index automatically,
// because the pages_fts triggers run on every INSERT.
package crawler

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/JOMS-inc/JOMSearch/go-services/internal/searchlog"
)

// Languages the crawler accepts. The language ends up in the API hostname
// (en.wikipedia.org), so only known values may get there; a search for
// language=evil.example must not make us send requests to another host.
// These are also the only values the pages table allows.
var Languages = map[string]bool{"en": true, "da": true}

// WikipediaAPI returns the API endpoint for a Wikipedia language edition.
func WikipediaAPI(lang string) string {
	return "https://" + lang + ".wikipedia.org/w/api.php"
}

// Crawler fetches articles for search terms and stores them.
type Crawler struct {
	DB        *sql.DB
	HTTP      *http.Client
	APIURL    func(lang string) string // WikipediaAPI, or a test server
	UserAgent string
	Log       *slog.Logger

	PagesPerTerm int           // articles to fetch per search term
	Delay        time.Duration // pause between requests (rate limit)
	Refresh      time.Duration // crawl a term again once it is this old
}

// Stats summarises one run.
type Stats struct {
	Terms   int // terms fetched
	Skipped int // terms crawled recently, or not valid
	Failed  int // terms whose request failed (retried next run)
	Pages   int // new pages stored
}

// Run crawls up to maxTerms of terms, in the given order.
//
// A failing term is logged and skipped so one bad request does not stop the
// run; it is not marked as crawled, so the next run tries it again. Only a
// cancelled context (Ctrl+C, docker stop) ends the run early.
func (c *Crawler) Run(ctx context.Context, terms []searchlog.Term, maxTerms int) (Stats, error) {
	var st Stats
	for _, t := range terms {
		if st.Terms+st.Failed >= maxTerms {
			break
		}
		if !validTerm(t) {
			st.Skipped++
			continue
		}
		recent, err := c.crawledRecently(t)
		if err != nil {
			return st, err
		}
		if recent {
			st.Skipped++
			continue
		}

		if st.Terms+st.Failed > 0 {
			if err := sleep(ctx, c.Delay); err != nil {
				return st, err
			}
		}

		articles, err := c.fetch(ctx, t)
		if err != nil {
			if ctx.Err() != nil {
				return st, ctx.Err()
			}
			c.Log.Warn("fetch failed", "query", t.Query, "language", t.Language, "err", err)
			st.Failed++
			continue
		}
		added, err := c.store(t, articles)
		if err != nil {
			return st, err
		}
		st.Terms++
		st.Pages += added
		c.Log.Info("crawled", "query", t.Query, "language", t.Language,
			"searches", t.Searches, "found", len(articles), "new", added)
	}
	return st, nil
}

func validTerm(t searchlog.Term) bool {
	n := len([]rune(t.Query))
	return Languages[t.Language] && n >= 2 && n <= 100
}

func (c *Crawler) crawledRecently(t searchlog.Term) (bool, error) {
	var at time.Time
	err := c.DB.QueryRow(
		`SELECT crawled_at FROM crawled_terms WHERE query = ? AND language = ?`,
		t.Query, t.Language,
	).Scan(&at)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return time.Since(at) < c.Refresh, nil
}

// Article is one Wikipedia page as returned by the API.
type Article struct {
	Title   string `json:"title"`
	URL     string `json:"fullurl"`
	Extract string `json:"extract"`
	Index   int    `json:"index"` // position in the search results
}

type apiResponse struct {
	Query struct {
		Pages []Article `json:"pages"`
	} `json:"query"`
	Error *struct {
		Code string `json:"code"`
		Info string `json:"info"`
	} `json:"error"`
}

// fetch searches Wikipedia for t and returns the matching articles with
// their intro text, best match first.
func (c *Crawler) fetch(ctx context.Context, t searchlog.Term) ([]Article, error) {
	params := url.Values{
		"action":        {"query"},
		"format":        {"json"},
		"formatversion": {"2"},
		// generator=search runs a search and feeds the hits to prop=...,
		// so search + content is a single request.
		"generator":    {"search"},
		"gsrsearch":    {t.Query},
		"gsrlimit":     {fmt.Sprint(c.PagesPerTerm)},
		"gsrnamespace": {"0"}, // articles only, no talk/user pages
		"prop":         {"extracts|info"},
		"exintro":      {"1"}, // the intro; full text is limited to 1 page per request
		"explaintext":  {"1"}, // plain text instead of HTML
		"exlimit":      {"max"},
		"inprop":       {"url"},
		"redirects":    {"1"},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.APIURL(t.Language)+"?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.UserAgent)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %s", resp.Status)
	}

	var body apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if body.Error != nil {
		return nil, fmt.Errorf("api error %s: %s", body.Error.Code, body.Error.Info)
	}

	var articles []Article
	for _, a := range body.Query.Pages {
		a.Extract = strings.TrimSpace(a.Extract)
		if a.Title != "" && a.URL != "" && a.Extract != "" {
			articles = append(articles, a)
		}
	}
	// The API returns pages in no particular order; index is the search rank.
	sort.Slice(articles, func(i, j int) bool { return articles[i].Index < articles[j].Index })
	return articles, nil
}

// store inserts the articles and marks t as crawled, in one transaction.
// Pages that already exist (same title or URL) are left alone.
// It returns how many pages were new.
func (c *Crawler) store(t searchlog.Term, articles []Article) (int, error) {
	tx, err := c.DB.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback() //nolint:errcheck // no-op after Commit

	now := time.Now().UTC()
	added := 0
	for _, a := range articles {
		res, err := tx.Exec(
			`INSERT INTO pages (title, url, language, last_updated, content)
			 VALUES (?, ?, ?, ?, ?) ON CONFLICT DO NOTHING`,
			a.Title, a.URL, t.Language, now, a.Extract,
		)
		if err != nil {
			return 0, fmt.Errorf("insert %q: %w", a.Title, err)
		}
		n, _ := res.RowsAffected()
		added += int(n)
	}

	if _, err := tx.Exec(
		`INSERT INTO crawled_terms (query, language, crawled_at, pages_added) VALUES (?, ?, ?, ?)
		 ON CONFLICT (query, language) DO UPDATE SET crawled_at = excluded.crawled_at, pages_added = excluded.pages_added`,
		t.Query, t.Language, now, added,
	); err != nil {
		return 0, err
	}
	return added, tx.Commit()
}

func sleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
