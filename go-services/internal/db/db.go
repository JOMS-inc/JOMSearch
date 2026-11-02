package db

import (
	"database/sql"

	_ "modernc.org/sqlite"
)

func Open(path string) (*sql.DB, error) {
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	if err := conn.Ping(); err != nil {
		return nil, err
	}

	return conn, nil
}

// Migrate applies schema changes the production database may be missing.
// The prod DB only lives on the server, so the app upgrades it on startup.
func Migrate(conn *sql.DB) error {
	has, err := hasColumn(conn, "users", "must_change_password")
	if err != nil || has {
		return err
	}

	// The column is added once, right after the breach: every user that
	// exists at that point is flagged and must pick a new password.
	// Users registered later get the default 0.
	tx, err := conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // no-op after Commit

	if _, err := tx.Exec(`ALTER TABLE users ADD COLUMN must_change_password INTEGER NOT NULL DEFAULT 0`); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE users SET must_change_password = 1`); err != nil {
		return err
	}
	return tx.Commit()
}

func hasColumn(conn *sql.DB, table, column string) (bool, error) {
	rows, err := conn.Query(`SELECT name FROM pragma_table_info(?)`, table)
	if err != nil {
		return false, err
	}
	defer rows.Close()

	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return false, err
		}
		if name == column {
			return true, nil
		}
	}
	return false, rows.Err()
}
