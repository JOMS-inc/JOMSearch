// Command testdb seeds an empty PostgreSQL database for the E2E tests.
//
// Usage: go run ./cmd/testdb postgres://postgres:postgres@localhost:5432/jomsearch_test?sslmode=disable
//
// The database must be empty (a fresh one, as in CI): it is migrated and
// then filled with the same pages and users the Go tests use.
package main

import (
	"fmt"
	"os"

	"github.com/JOMS-inc/JOMSearch/go-services/internal/db"
	"github.com/JOMS-inc/JOMSearch/go-services/internal/testdb"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: testdb <postgres-url-of-an-empty-database>")
		os.Exit(2)
	}

	conn, err := db.Open(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer conn.Close()

	if err := testdb.Seed(conn); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("seeded test database")
}
