// Package sqldb opens the *.postcards collection files described in
// pkg/collection. It hides the choice of SQLite driver behind a build tag:
// modernc.org/sqlite (pure Go) for regular builds, and github.com/mattn/go-sqlite3
// (cgo) for gomobile builds, which need a driver that works on iOS.
package sqldb

import "strings"

// busyTimeoutMS controls how long a connection waits on a lock held by
// another connection/process before giving up with SQLITE_BUSY.
const busyTimeoutMS = 5000

// escapePath percent-encodes the characters SQLite's URI filename parser
// treats as syntactically significant, so paths containing '%', '?' or '#'
// round-trip correctly through the "file:" DSNs both drivers expect.
func escapePath(path string) string {
	return strings.NewReplacer("%", "%25", "?", "%3f", "#", "%23").Replace(path)
}
