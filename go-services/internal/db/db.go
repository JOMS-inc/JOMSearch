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
// Every step checks first, so running it again is a no-op.
func Migrate(conn *sql.DB) error {
	if err := migrateMustChangePassword(conn); err != nil {
		return err
	}
	return migrateFullTextIndex(conn)
}

func migrateMustChangePassword(conn *sql.DB) error {
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

// ftsSchema is the full-text index on pages, the same as in schema/schema.sql.
// pages_fts is an "external content" table: it only stores the index, the
// text itself stays in pages. The triggers keep the index in sync whenever
// a page is inserted, updated or deleted.
const ftsSchema = `
CREATE VIRTUAL TABLE IF NOT EXISTS pages_fts USING fts5(
    title,
    content,
    content='pages',
    content_rowid='rowid',
    tokenize='unicode61 remove_diacritics 2'
);

CREATE TRIGGER IF NOT EXISTS pages_ai AFTER INSERT ON pages BEGIN
    INSERT INTO pages_fts(rowid, title, content)
    VALUES (new.rowid, new.title, new.content);
END;

CREATE TRIGGER IF NOT EXISTS pages_ad AFTER DELETE ON pages BEGIN
    INSERT INTO pages_fts(pages_fts, rowid, title, content)
    VALUES ('delete', old.rowid, old.title, old.content);
END;

CREATE TRIGGER IF NOT EXISTS pages_au AFTER UPDATE ON pages BEGIN
    INSERT INTO pages_fts(pages_fts, rowid, title, content)
    VALUES ('delete', old.rowid, old.title, old.content);
    INSERT INTO pages_fts(rowid, title, content)
    VALUES (new.rowid, new.title, new.content);
END;
`

// migrateFullTextIndex adds the pages_fts index to databases created before
// it existed, and fills it with the pages that are already there.
func migrateFullTextIndex(conn *sql.DB) error {
	var n int
	err := conn.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = 'pages_fts'`).Scan(&n)
	if err != nil || n > 0 {
		return err
	}

	tx, err := conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // no-op after Commit

	if _, err := tx.Exec(ftsSchema); err != nil {
		return err
	}
	// 'rebuild' indexes every existing row in pages.
	if _, err := tx.Exec(`INSERT INTO pages_fts(pages_fts) VALUES ('rebuild')`); err != nil {
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
