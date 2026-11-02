// Command testdb writes a fresh, seeded SQLite database for the E2E tests.
//
// Usage: go run ./cmd/testdb /tmp/e2e.db
package main

import (
	"fmt"
	"os"

	"github.com/JOMS-inc/JOMSearch/go-services/internal/testdb"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: testdb <path-to-new.db>")
		os.Exit(2)
	}
	path := os.Args[1]

	// Start from an empty file so reruns don't hit duplicate pages.
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	conn, err := testdb.Create(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	conn.Close()
	fmt.Println("seeded test database at", path)
}
