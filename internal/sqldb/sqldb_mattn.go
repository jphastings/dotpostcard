//go:build mobile_sqlite

// This build wires up github.com/mattn/go-sqlite3 (cgo), the only sqlite
// driver that works under gomobile on iOS; it needs the `sqlite_fts5` build
// tag alongside `mobile_sqlite` for FTS5 support to be compiled in.
package sqldb

import (
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

const driverName = "sqlite3"

// Open opens (creating the file if needed) a SQLite database at path.
// journal_mode is deliberately left at its default (DELETE) rather than WAL,
// since WAL's sidecar files don't sync atomically over iCloud Drive.
func Open(path string, readOnly bool) (*sql.DB, error) {
	dsn := fmt.Sprintf("file:%s?_busy_timeout=%d", escapePath(path), busyTimeoutMS)
	if readOnly {
		dsn += "&mode=ro"
	}

	db, err := sql.Open(driverName, dsn)
	if err != nil {
		return nil, fmt.Errorf("opening sqlite database %q: %w", path, err)
	}

	// Non-WAL SQLite serializes writers; a single pooled connection avoids
	// spurious SQLITE_BUSY errors between concurrent goroutines.
	db.SetMaxOpenConns(1)

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("opening sqlite database %q: %w", path, err)
	}

	return db, nil
}
