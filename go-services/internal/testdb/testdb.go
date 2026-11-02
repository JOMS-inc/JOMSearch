// Package testdb builds a fresh, seeded SQLite database for tests.
//
// It applies the real schema (schema/schema.sql at the repo root) and adds a
// few known pages, so Go tests and the Playwright E2E tests (via cmd/testdb)
// all run against the same, predictable data.
package testdb

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
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

// schemaPath finds schema/schema.sql relative to this source file, so it works
// no matter which directory `go test` or `go run` is started from.
func schemaPath() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "schema", "schema.sql")
}

// Seed applies the schema and inserts Pages into conn.
func Seed(conn *sql.DB) error {
	schema, err := os.ReadFile(schemaPath())
	if err != nil {
		return fmt.Errorf("read schema: %w", err)
	}
	if _, err := conn.Exec(string(schema)); err != nil {
		return fmt.Errorf("apply schema: %w", err)
	}
	for _, p := range Pages {
		if _, err := conn.Exec(
			`INSERT INTO pages (title, url, language, content) VALUES (?, ?, ?, ?)`,
			p.Title, p.URL, p.Language, p.Content,
		); err != nil {
			return fmt.Errorf("insert page %q: %w", p.Title, err)
		}
	}
	return nil
}

// Create opens (or creates) the SQLite file at path and seeds it.
func Create(path string) (*sql.DB, error) {
	conn, err := db.Open(path)
	if err != nil {
		return nil, err
	}
	if err := Seed(conn); err != nil {
		conn.Close()
		return nil, err
	}
	return conn, nil
}

// New returns a seeded database in a temp dir that is removed after the test.
func New(t testing.TB) *sql.DB {
	t.Helper()
	conn, err := Create(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("testdb: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}
