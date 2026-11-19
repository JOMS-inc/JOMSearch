package db_test

import (
	"testing"

	"github.com/JOMS-inc/JOMSearch/go-services/internal/db"
	"github.com/JOMS-inc/JOMSearch/go-services/internal/testdb"
)

// TestMigrateIsIdempotent: the app and the crawler both migrate on every
// start, so running Migrate on an up-to-date database must do nothing.
func TestMigrateIsIdempotent(t *testing.T) {
	conn := testdb.New(t) // already migrated once
	if err := db.Migrate(conn); err != nil {
		t.Fatalf("second Migrate: %v", err)
	}
}

// TestSearchColumnFollowsLanguage checks the generated search column: it is
// filled on INSERT and UPDATE without any trigger, and stems each page in
// its own language.
func TestSearchColumnFollowsLanguage(t *testing.T) {
	conn := testdb.New(t)

	matches := func(cfg, word string) int {
		t.Helper()
		var n int
		err := conn.QueryRow(
			`SELECT count(*) FROM pages WHERE search @@ to_tsquery($1::regconfig, $2)`, cfg, word,
		).Scan(&n)
		if err != nil {
			t.Fatal(err)
		}
		return n
	}

	// English stemming: "languages" and "language" share the stem "languag".
	if got := matches("english", "languages"); got != 2 {
		t.Errorf("english 'languages': %d pages, want 2", got)
	}
	// Danish stemming: "hovedstaden" (the capital) stems like "hovedstad".
	if got := matches("danish", "hovedstaden"); got != 1 {
		t.Errorf("danish 'hovedstaden': %d pages, want 1", got)
	}

	// UPDATE recomputes the column.
	if _, err := conn.Exec(`UPDATE pages SET content = 'Nothing here.' WHERE title = 'København'`); err != nil {
		t.Fatal(err)
	}
	if got := matches("danish", "hovedstad"); got != 0 {
		t.Errorf("after update: %d pages, want 0", got)
	}
}
