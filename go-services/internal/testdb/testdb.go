// Package testdb builds a fresh, seeded PostgreSQL database for tests.
//
// It applies the real migrations (internal/db/migrations) and adds a few
// known pages and the legacy admin user, so Go tests and the Playwright E2E
// tests (via cmd/testdb) all run against the same, predictable data.
//
// Tests need a Postgres server. Point TEST_DATABASE_URL at one, e.g. the
// dev one from docker-compose.dev.yml:
//
//	TEST_DATABASE_URL=postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable go test ./...
//
// Without it, database tests are skipped locally and fail in CI.
package testdb

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"testing"

	"github.com/JOMS-inc/JOMSearch/go-services/internal/db"
)

// Page is a row inserted into the pages table by Seed.
type Page struct {
	Title    string
	URL      string
	Language string
	Content  string
}

// Pages are the known search results the tests can rely on.
var Pages = []Page{
	{
		"Go (programming language)", "https://en.wikipedia.org/wiki/Go_(programming_language)", "en",
		"Go is a statically typed, compiled programming language designed at Google.",
	},
	{
		"SQLite", "https://en.wikipedia.org/wiki/SQLite", "en",
		"SQLite is a database engine written in the C programming language.",
	},
	{
		"København", "https://da.wikipedia.org/wiki/K%C3%B8benhavn", "da",
		"København er Danmarks hovedstad og største by.",
	},
}

// Seed applies the migrations and inserts Pages and the admin user.
func Seed(conn *sql.DB) error {
	if err := db.Migrate(conn); err != nil {
		return err
	}
	// The legacy admin user: its password is "password" as an MD5 hash, the
	// way the old app stored it, and it is flagged because the hash leaked.
	if _, err := conn.Exec(
		`INSERT INTO users (username, email, password, must_change_password)
		 VALUES ('admin', 'keamonk1@stud.kea.dk', '5f4dcc3b5aa765d61d8327deb882cf99', TRUE)`,
	); err != nil {
		return fmt.Errorf("insert admin: %w", err)
	}
	for _, p := range Pages {
		if _, err := conn.Exec(
			`INSERT INTO pages (title, url, language, content) VALUES ($1, $2, $3, $4)`,
			p.Title, p.URL, p.Language, p.Content,
		); err != nil {
			return fmt.Errorf("insert page %q: %w", p.Title, err)
		}
	}
	return nil
}

// New returns a seeded database for one test, in its own Postgres schema
// (namespace) that is dropped after the test. Separate schemas let test
// packages run in parallel against the same server without seeing each
// other's rows.
func New(t testing.TB) *sql.DB {
	t.Helper()
	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		if os.Getenv("CI") != "" {
			t.Fatal("TEST_DATABASE_URL is not set")
		}
		t.Skip("TEST_DATABASE_URL not set, skipping database test")
	}

	admin, err := db.Open(base)
	if err != nil {
		t.Fatalf("testdb: %v", err)
	}
	t.Cleanup(func() { admin.Close() })

	schema := "test_" + randomHex(8)
	if _, err := admin.Exec(`CREATE SCHEMA ` + schema); err != nil {
		t.Fatalf("testdb: create schema: %v", err)
	}
	t.Cleanup(func() {
		if _, err := admin.Exec(`DROP SCHEMA ` + schema + ` CASCADE`); err != nil {
			t.Errorf("testdb: drop schema: %v", err)
		}
	})

	conn, err := db.Open(withSearchPath(t, base, schema))
	if err != nil {
		t.Fatalf("testdb: %v", err)
	}
	t.Cleanup(func() { conn.Close() }) // runs before the DROP above (LIFO)

	if err := Seed(conn); err != nil {
		t.Fatalf("testdb: %v", err)
	}
	return conn
}

// withSearchPath makes every connection from the URL use schema, so the
// unqualified table names in the app and migrations resolve to it.
func withSearchPath(t testing.TB, base, schema string) string {
	t.Helper()
	u, err := url.Parse(base)
	if err != nil {
		t.Fatalf("testdb: parse TEST_DATABASE_URL: %v", err)
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	return u.String()
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b) // never returns an error (crypto/rand docs)
	return hex.EncodeToString(b)
}
