//go:build !mobile_sqlite && !wasm

// This build wires up modernc.org/sqlite, a pure-Go (cgo-free) driver, so
// `CGO_ENABLED=0` builds and tests keep working. It includes FTS5 support.
package sqldb

import (
	"database/sql"
	"fmt"
	"net/url"

	_ "modernc.org/sqlite"
)

const driverName = "sqlite"

// Open opens (creating the file if needed) a SQLite database at path.
// journal_mode is deliberately left at its default (DELETE) rather than WAL,
// since WAL's sidecar files don't sync atomically over iCloud Drive.
func Open(path string, readOnly bool) (*sql.DB, error) {
	q := url.Values{}
	q.Set("_pragma", fmt.Sprintf("busy_timeout(%d)", busyTimeoutMS))
	if readOnly {
		q.Set("mode", "ro")
	}

	db, err := sql.Open(driverName, "file:"+escapePath(path)+"?"+q.Encode())
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
