package db

import (
	"path/filepath"
	"testing"
)

// TestMigrateAddsFullTextIndex starts from the old schema (no pages_fts,
// no must_change_password) like the prod DB, and checks that Migrate adds
// the index, fills it with existing pages, and keeps it in sync afterwards.
func TestMigrateAddsFullTextIndex(t *testing.T) {
	conn, err := Open(filepath.Join(t.TempDir(), "old.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	_, err = conn.Exec(`
		CREATE TABLE users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL UNIQUE,
			email TEXT NOT NULL UNIQUE,
			password TEXT NOT NULL
		);
		CREATE TABLE pages (
			title TEXT PRIMARY KEY UNIQUE,
			url TEXT NOT NULL UNIQUE,
			language TEXT NOT NULL DEFAULT 'en',
			last_updated TIMESTAMP,
			content TEXT NOT NULL
		);
		INSERT INTO pages (title, url, content) VALUES ('Old page', 'https://a', 'existing content');`)
	if err != nil {
		t.Fatal(err)
	}

	// Run twice: the second run must be a no-op.
	for i := 0; i < 2; i++ {
		if err := Migrate(conn); err != nil {
			t.Fatalf("Migrate run %d: %v", i+1, err)
		}
	}

	count := func(match string) int {
		t.Helper()
		var n int
		if err := conn.QueryRow(`SELECT count(*) FROM pages_fts WHERE pages_fts MATCH ?`, match).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}

	if got := count("existing"); got != 1 {
		t.Errorf("page from before the migration: got %d matches, want 1", got)
	}

	// The triggers keep the index in sync with new and deleted pages.
	if _, err := conn.Exec(`INSERT INTO pages (title, url, content) VALUES ('New', 'https://b', 'fresh words')`); err != nil {
		t.Fatal(err)
	}
	if got := count("fresh"); got != 1 {
		t.Errorf("inserted page: got %d matches, want 1", got)
	}
	if _, err := conn.Exec(`DELETE FROM pages WHERE title = 'New'`); err != nil {
		t.Fatal(err)
	}
	if got := count("fresh"); got != 0 {
		t.Errorf("deleted page: got %d matches, want 0", got)
	}
}
