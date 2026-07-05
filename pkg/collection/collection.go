// Package collection reads and writes *.postcards files: SQLite databases
// that hold web-format postcard files as blobs, alongside extracted &
// FTS5-searchable metadata, for the postcard viewer app.
package collection

import (
	"database/sql"
	"errors"
	"fmt"
	"os"

	"github.com/jphastings/dotpostcard/internal/sqldb"
	"github.com/jphastings/dotpostcard/internal/version"
	"github.com/jphastings/dotpostcard/types"
)

// ErrNotFound is returned (wrapped) when a card name doesn't exist in the collection.
var ErrNotFound = errors.New("card not found")

// errReadOnly is returned (wrapped) by mutating methods on a collection opened with OpenReadOnly.
var errReadOnly = errors.New("collection is read-only")

// Collection is an open *.postcards file.
type Collection struct {
	db       *sql.DB
	readOnly bool
}

// CardSummary is the list/search-friendly view of a card: everything needed
// to render a grid cell or list row without decoding the card's image data.
type CardSummary struct {
	Name          string      `json:"name"`
	Filename      string      `json:"filename"`
	Mimetype      string      `json:"mimetype"`
	Flip          types.Flip  `json:"flip"`
	SentOn        *types.Date `json:"sent_on,omitempty"`
	SenderName    string      `json:"sender_name,omitempty"`
	RecipientName string      `json:"recipient_name,omitempty"`
	LocationName  string      `json:"location_name,omitempty"`
	CountryCode   string      `json:"country_code,omitempty"`
	Latitude      *float64    `json:"latitude,omitempty"`
	Longitude     *float64    `json:"longitude,omitempty"`
	FrontPxW      int         `json:"front_px_w"`
	FrontPxH      int         `json:"front_px_h"`
	HasBack       bool        `json:"has_back"`
}

// Create makes a new, empty collection file. It errors if path already exists.
func Create(path string) (*Collection, error) {
	if _, err := os.Stat(path); err == nil {
		return nil, fmt.Errorf("creating collection at %s: file already exists", path)
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("creating collection at %s: %w", path, err)
	}

	db, err := sqldb.Open(path, false)
	if err != nil {
		return nil, fmt.Errorf("creating collection at %s: %w", path, err)
	}

	if err := initSchema(db, fmt.Sprintf("postcards/v%s", version.Version)); err != nil {
		db.Close()
		os.Remove(path)
		return nil, fmt.Errorf("creating collection at %s: %w", path, err)
	}

	return &Collection{db: db}, nil
}

// Open opens an existing collection for reading and writing, first migrating
// it to the current schema version if it was written by an older postcards.
func Open(path string) (*Collection, error) {
	db, err := sqldb.Open(path, false)
	if err != nil {
		return nil, fmt.Errorf("opening collection at %s: %w", path, err)
	}
	if err := ensureSchema(db, false); err != nil {
		db.Close()
		return nil, fmt.Errorf("opening collection at %s: %w", path, err)
	}

	return &Collection{db: db}, nil
}

// OpenReadOnly opens an existing collection for reading only; mutating
// methods (AddWebPostcard, Remove) will fail, and collections with an
// outdated schema are never migrated (they error instead).
func OpenReadOnly(path string) (*Collection, error) {
	db, err := sqldb.Open(path, true)
	if err != nil {
		return nil, fmt.Errorf("opening collection at %s: %w", path, err)
	}
	if err := ensureSchema(db, true); err != nil {
		db.Close()
		return nil, fmt.Errorf("opening collection at %s: %w", path, err)
	}

	return &Collection{db: db, readOnly: true}, nil
}

// Close releases the underlying database handle.
func (c *Collection) Close() error {
	return c.db.Close()
}

// Title returns the collection's user-set title, or "" if none has been set.
// There's no filename-derived fallback here; presenting one is the caller's job.
func (c *Collection) Title() (string, error) {
	var title sql.NullString
	if err := c.db.QueryRow(`SELECT title FROM meta`).Scan(&title); err != nil {
		return "", fmt.Errorf("reading collection title: %w", err)
	}
	return title.String, nil
}

// SetTitle sets the collection's title; an empty string clears it.
func (c *Collection) SetTitle(title string) error {
	if c.readOnly {
		return errReadOnly
	}

	var arg any
	if title != "" {
		arg = title
	}

	if _, err := c.db.Exec(`UPDATE meta SET title = ?`, arg); err != nil {
		return fmt.Errorf("setting collection title: %w", err)
	}
	return nil
}
