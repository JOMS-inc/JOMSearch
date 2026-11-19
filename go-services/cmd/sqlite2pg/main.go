// Command sqlite2pg copies the data from the old SQLite database into
// PostgreSQL. It is a one-off tool for the move to Postgres.
//
//	go run ./cmd/sqlite2pg -sqlite ../db/whoknows.db -pg "$DATABASE_URL"
//
// It is safe to run more than once: rows that already exist are updated
// instead of duplicated. That is what makes a short cutover possible: copy
// everything while the old app keeps serving, then run it once more right
// before switching, which only has to catch up on the few rows (new users,
// changed passwords) written in between. See database.md.
package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"time"

	_ "modernc.org/sqlite" // reads the old database

	"github.com/JOMS-inc/JOMSearch/go-services/internal/db"
)

func main() {
	sqlitePath := flag.String("sqlite", "../db/whoknows.db", "old SQLite database (read only)")
	pgURL := flag.String("pg", "", "PostgreSQL connection URL (required)")
	flag.Parse()
	if *pgURL == "" {
		log.Fatal("-pg is required")
	}

	// mode=ro: the copy never writes to the old database.
	src, err := sql.Open("sqlite", "file:"+*sqlitePath+"?mode=ro")
	if err != nil {
		log.Fatal(err)
	}
	defer src.Close()
	if err := src.Ping(); err != nil {
		log.Fatalf("open sqlite: %v", err)
	}

	dst, err := db.Open(*pgURL)
	if err != nil {
		log.Fatalf("open postgres: %v", err)
	}
	defer dst.Close()
	if err := db.Migrate(dst); err != nil {
		log.Fatal(err)
	}

	start := time.Now()
	counts, err := copyAll(src, dst)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("copied %d users, %d pages, %d crawled terms in %s",
		counts[0], counts[1], counts[2], time.Since(start).Round(time.Millisecond))
}

// copyAll copies every table in one Postgres transaction: either all of it
// lands or none of it does, so a failed run leaves Postgres as it was.
func copyAll(src, dst *sql.DB) ([3]int, error) {
	var counts [3]int
	tx, err := dst.Begin()
	if err != nil {
		return counts, err
	}
	defer tx.Rollback() //nolint:errcheck // no-op after Commit

	if counts[0], err = copyUsers(src, tx); err != nil {
		return counts, fmt.Errorf("users: %w", err)
	}
	if counts[1], err = copyPages(src, tx); err != nil {
		return counts, fmt.Errorf("pages: %w", err)
	}
	if counts[2], err = copyCrawledTerms(src, tx); err != nil {
		return counts, fmt.Errorf("crawled_terms: %w", err)
	}
	return counts, tx.Commit()
}

func copyUsers(src *sql.DB, tx *sql.Tx) (int, error) {
	// Databases from before the breach fix lack must_change_password; the old
	// app's migration flagged every existing user, so do the same here.
	mustChange := "1"
	if has, err := hasColumn(src, "users", "must_change_password"); err != nil {
		return 0, err
	} else if has {
		mustChange = "must_change_password"
	}

	rows, err := src.Query(`SELECT id, username, email, password, ` + mustChange + ` FROM users ORDER BY id`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	n := 0
	for rows.Next() {
		var id int64
		var username, email, password string
		var flag int
		if err := rows.Scan(&id, &username, &email, &password, &flag); err != nil {
			return n, err
		}
		// Same id as in SQLite, so existing session cookies keep working.
		if _, err := tx.Exec(
			`INSERT INTO users (id, username, email, password, must_change_password)
			 VALUES ($1, $2, $3, $4, $5)
			 ON CONFLICT (id) DO UPDATE SET username = excluded.username, email = excluded.email,
			     password = excluded.password, must_change_password = excluded.must_change_password`,
			id, username, email, password, flag != 0,
		); err != nil {
			return n, err
		}
		n++
	}
	if err := rows.Err(); err != nil {
		return n, err
	}

	// Ids were inserted by hand, so move the id counter past them; otherwise
	// the next registration would get id 1 again and fail.
	_, err = tx.Exec(`SELECT setval(pg_get_serial_sequence('users', 'id'), GREATEST((SELECT max(id) FROM users), 1))`)
	return n, err
}

func copyPages(src *sql.DB, tx *sql.Tx) (int, error) {
	rows, err := src.Query(`SELECT title, url, language, last_updated, content FROM pages`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	n := 0
	for rows.Next() {
		var title, url, language, content string
		var updated sql.NullString // SQLite stores it as text, if at all
		if err := rows.Scan(&title, &url, &language, &updated, &content); err != nil {
			return n, err
		}
		var updatedAt any // stays NULL if missing or unreadable
		if t, ok := parseTime(updated.String); ok {
			updatedAt = t
		}
		// pages.search is generated, so Postgres indexes each page on insert.
		if _, err := tx.Exec(
			`INSERT INTO pages (title, url, language, last_updated, content)
			 VALUES ($1, $2, $3, $4, $5)
			 ON CONFLICT (title) DO UPDATE SET url = excluded.url, language = excluded.language,
			     last_updated = excluded.last_updated, content = excluded.content`,
			title, url, language, updatedAt, content,
		); err != nil {
			return n, fmt.Errorf("%q: %w", title, err)
		}
		n++
	}
	return n, rows.Err()
}

func copyCrawledTerms(src *sql.DB, tx *sql.Tx) (int, error) {
	if has, err := hasTable(src, "crawled_terms"); err != nil || !has {
		return 0, err // older databases have no crawler data
	}
	rows, err := src.Query(`SELECT query, language, crawled_at, pages_added FROM crawled_terms`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	n := 0
	for rows.Next() {
		var query, language, crawledAt string
		var added int
		if err := rows.Scan(&query, &language, &crawledAt, &added); err != nil {
			return n, err
		}
		at, ok := parseTime(crawledAt)
		if !ok {
			at = time.Time{} // unknown: counts as long ago, so it is crawled again
		}
		if _, err := tx.Exec(
			`INSERT INTO crawled_terms (query, language, crawled_at, pages_added)
			 VALUES ($1, $2, $3, $4)
			 ON CONFLICT (query, language) DO UPDATE SET crawled_at = excluded.crawled_at, pages_added = excluded.pages_added`,
			query, language, at, added,
		); err != nil {
			return n, err
		}
		n++
	}
	return n, rows.Err()
}

// timeLayouts are the ways SQLite rows store time: the Go SQLite driver
// writes time.Time.String(), and older rows may be plain SQL timestamps.
var timeLayouts = []string{
	"2006-01-02 15:04:05.999999999 -0700 MST",
	"2006-01-02 15:04:05.999999999-07:00",
	time.RFC3339Nano,
	"2006-01-02 15:04:05",
	"2006-01-02",
}

func parseTime(s string) (time.Time, bool) {
	for _, layout := range timeLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func hasTable(conn *sql.DB, table string) (bool, error) {
	var n int
	err := conn.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&n)
	return n > 0, err
}

func hasColumn(conn *sql.DB, table, column string) (bool, error) {
	var n int
	err := conn.QueryRow(`SELECT count(*) FROM pragma_table_info(?) WHERE name = ?`, table, column).Scan(&n)
	return n > 0, err
}
