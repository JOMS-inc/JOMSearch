// Package searchlog records what users search for and summarises it.
//
// Every search becomes one JSON line in a log file, written with Go's
// built-in structured logger (log/slog):
//
//	{"time":"...","level":"INFO","msg":"search","query":"go","language":"en","results":3}
//
// A plain file is enough here: it needs no extra service, it is easy to read
// with tail/grep/jq, and the crawler can read it to decide what to scrape.
package searchlog

import (
	"bufio"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Msg is the slog message used for search lines. Rank skips other lines.
const Msg = "search"

// Open returns a logger that appends JSON lines to the file at path,
// creating the file and its directory if needed. Close the file on shutdown.
func Open(path string) (*slog.Logger, *os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, nil, err
	}
	return slog.New(slog.NewJSONHandler(f, nil)), f, nil
}

// Log writes one search to logger.
func Log(logger *slog.Logger, query, language string, results int) {
	logger.Info(Msg,
		slog.String("query", query),
		slog.String("language", language),
		slog.Int("results", results),
	)
}

// Term is one distinct search, summed over the whole log.
type Term struct {
	Query    string
	Language string
	Searches int // how many times it was searched
	Results  int // result count the last time it was searched
}

type line struct {
	Msg      string `json:"msg"`
	Query    string `json:"query"`
	Language string `json:"language"`
	Results  int    `json:"results"`
}

// Rank reads a search log and returns the distinct searches, most useful to
// scrape first: searches that last found nothing come before those that
// did, and within each group the most searched come first.
//
// Queries are lowercased and trimmed so "Go" and " go" count as one.
// Lines that are not search lines or are not valid JSON are skipped, so a
// half-written last line or a stray message never breaks the crawler.
func Rank(r io.Reader) ([]Term, error) {
	byKey := map[[2]string]*Term{}
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		var l line
		if json.Unmarshal(sc.Bytes(), &l) != nil || l.Msg != Msg {
			continue
		}
		q := strings.ToLower(strings.TrimSpace(l.Query))
		if q == "" {
			continue
		}
		key := [2]string{q, l.Language}
		t := byKey[key]
		if t == nil {
			t = &Term{Query: q, Language: l.Language}
			byKey[key] = t
		}
		t.Searches++
		t.Results = l.Results // the log is in time order, so this ends as the latest
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}

	terms := make([]Term, 0, len(byKey))
	for _, t := range byKey {
		terms = append(terms, *t)
	}
	sort.Slice(terms, func(i, j int) bool {
		a, b := terms[i], terms[j]
		if (a.Results == 0) != (b.Results == 0) {
			return a.Results == 0
		}
		if a.Searches != b.Searches {
			return a.Searches > b.Searches
		}
		return a.Query < b.Query // stable order for equal counts
	})
	return terms, nil
}
