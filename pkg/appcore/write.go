package appcore

import (
	"fmt"

	"github.com/jphastings/dotpostcard/pkg/collection"
)

// This file holds the write surface of the facade: transient, package-level
// operations rather than methods on Collection. Each one opens the
// collection file read-write, performs a single change, and closes it again,
// so a write never coincides with one of this package's own long-lived
// read-only Collection handles on the same file (avoiding SQLITE_BUSY
// contention; see internal/sqldb's busy_timeout) and stays easy to wrap in a
// single NSFileCoordinator write on the Swift side.
//
// Callers must invalidate (and, if needed, reopen) any Collection already
// open on path after calling one of these — a completed write is not
// visible through a handle opened before it ran.

// SetCollectionTitle opens the collection at path, sets its title, and closes it.
func SetCollectionTitle(path, title string) error {
	col, err := collection.Open(path)
	if err != nil {
		return fmt.Errorf("opening collection: %w", err)
	}
	defer col.Close()

	if err := col.SetTitle(title); err != nil {
		return fmt.Errorf("setting collection title: %w", err)
	}

	return nil
}

// AddCardToCollection opens the collection at path, decodes and stores the
// web-format postcard file (filename, data), and closes it. It returns the
// added card's CardSummary as JSON. An invalid or undecodable file errors
// without changing the collection.
func AddCardToCollection(path, filename string, data []byte) (string, error) {
	col, err := collection.Open(path)
	if err != nil {
		return "", fmt.Errorf("opening collection: %w", err)
	}
	defer col.Close()

	summary, err := col.AddWebPostcard(filename, data)
	if err != nil {
		return "", fmt.Errorf("adding card to collection: %w", err)
	}

	return marshalJSON(summary)
}

// RemoveCardFromCollection opens the collection at path, removes the named
// card, and closes it. It errors if no card with that name exists.
func RemoveCardFromCollection(path, name string) error {
	col, err := collection.Open(path)
	if err != nil {
		return fmt.Errorf("opening collection: %w", err)
	}
	defer col.Close()

	if err := col.Remove(name); err != nil {
		return fmt.Errorf("removing card from collection: %w", err)
	}

	return nil
}

// CreateCollection creates a new, empty collection file at path (erroring if
// it already exists), optionally setting its title, then closes it.
func CreateCollection(path, title string) error {
	col, err := collection.Create(path)
	if err != nil {
		return fmt.Errorf("creating collection: %w", err)
	}
	defer col.Close()

	if title != "" {
		if err := col.SetTitle(title); err != nil {
			return fmt.Errorf("setting collection title: %w", err)
		}
	}

	return nil
}
