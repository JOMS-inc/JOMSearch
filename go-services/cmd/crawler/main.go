// Command crawler reads the search log, picks the searches worth adding
// content for (those that found nothing first, then the most popular), and
// stores matching Wikipedia articles in the pages table.
//
// It is a one-shot job: run it on a schedule (cron) rather than keeping it
// running. It can run while the web app is serving, see db.Open.
//
//	go run ./cmd/crawler -max-terms 10
package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/JOMS-inc/JOMSearch/go-services/internal/crawler"
	"github.com/JOMS-inc/JOMSearch/go-services/internal/db"
	"github.com/JOMS-inc/JOMSearch/go-services/internal/searchlog"
)

func main() {
	dbPath := flag.String("db", getEnv("DB_PATH", "../db/whoknows.db"), "SQLite database")
	logPath := flag.String("log", getEnv("SEARCH_LOG", "../logs/searches.log"), "search log written by the web app")
	maxTerms := flag.Int("max-terms", 20, "most search terms to fetch in this run")
	perTerm := flag.Int("per-term", 5, "articles to fetch per search term")
	delay := flag.Duration("delay", time.Second, "pause between requests to Wikipedia")
	refresh := flag.Duration("refresh", 7*24*time.Hour, "crawl a term again after this long")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	if err := run(logger, *dbPath, *logPath, *maxTerms, *perTerm, *delay, *refresh); err != nil {
		logger.Error("crawl failed", "err", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger, dbPath, logPath string, maxTerms, perTerm int, delay, refresh time.Duration) error {
	f, err := os.Open(logPath)
	if err != nil {
		return err
	}
	terms, err := searchlog.Rank(f)
	_ = f.Close() // only read from
	if err != nil {
		return err
	}

	conn, err := db.Open(dbPath)
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := db.Migrate(conn); err != nil {
		return err
	}

	c := &crawler.Crawler{
		DB:           conn,
		HTTP:         &http.Client{Timeout: 15 * time.Second},
		APIURL:       crawler.WikipediaAPI,
		UserAgent:    userAgent(),
		Log:          logger,
		PagesPerTerm: perTerm,
		Delay:        delay,
		Refresh:      refresh,
	}

	// Stop cleanly on Ctrl+C or `docker stop`; finished terms are already saved.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	logger.Info("starting", "distinct_searches", len(terms), "max_terms", maxTerms)
	st, err := c.Run(ctx, terms, maxTerms)
	logger.Info("done", "terms", st.Terms, "skipped", st.Skipped, "failed", st.Failed, "new_pages", st.Pages)
	return err
}

// userAgent identifies the crawler, as Wikipedia's policy requires.
// Set CRAWLER_CONTACT (a URL or email) so they can reach you about it.
func userAgent() string {
	ua := "JOMSearchBot/1.0"
	if contact := os.Getenv("CRAWLER_CONTACT"); contact != "" {
		ua += " (" + contact + ")"
	}
	return ua
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
