// Package db connects to PostgreSQL and keeps its schema up to date.
package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"log/slog"

	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"
)

// The migration files are compiled into the binary, so the image needs no
// extra files and the schema always matches the code that uses it.
//
//go:embed migrations/*.sql
var migrations embed.FS

// Open connects to the PostgreSQL database at url, e.g.
// postgres://user:pass@host:5432/jomsearch?sslmode=require
func Open(url string) (*sql.DB, error) {
	conn, err := sql.Open("pgx", url)
	if err != nil {
		return nil, err
	}
	if err := conn.Ping(); err != nil {
		conn.Close()
		return nil, err
	}
	return conn, nil
}

// Migrate applies the migrations in migrations/ that have not run yet.
//
// The web app and the crawler both call it on startup. A Postgres advisory
// lock makes sure only one of them migrates at a time, so it is also safe
// with several app replicas starting together.
func Migrate(conn *sql.DB) error {
	sub, err := fs.Sub(migrations, "migrations")
	if err != nil {
		return err
	}
	// Retry the lock every second (default: every 5s) for up to 5 minutes.
	locker, err := lock.NewPostgresSessionLocker(lock.WithLockTimeout(1, 300))
	if err != nil {
		return err
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, conn, sub, goose.WithSessionLocker(locker))
	if err != nil {
		return err
	}
	results, err := provider.Up(context.Background())
	if err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	for _, r := range results {
		slog.Info("applied migration", "version", r.Source.Version, "file", r.Source.Path, "took", r.Duration)
	}
	return nil
}
