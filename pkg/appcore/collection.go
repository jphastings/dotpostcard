package appcore

import (
	"encoding/json"
	"fmt"

	"github.com/jphastings/dotpostcard/pkg/collection"
)

// Collection is a read-only, gomobile-bindable handle onto an open
// *.postcards collection file.
type Collection struct {
	path string
	col  *collection.Collection
}

// OpenCollection opens an existing collection file read-only.
func OpenCollection(path string) (*Collection, error) {
	col, err := collection.OpenReadOnly(path)
	if err != nil {
		return nil, fmt.Errorf("opening collection: %w", err)
	}

	return &Collection{path: path, col: col}, nil
}

// Close releases the underlying database handle.
func (c *Collection) Close() error {
	return c.col.Close()
}

// Path returns the filesystem path this collection was opened from.
func (c *Collection) Path() string {
	return c.path
}

// Title returns the collection's user-set title, or "" if none has been set.
func (c *Collection) Title() (string, error) {
	title, err := c.col.Title()
	if err != nil {
		return "", fmt.Errorf("reading collection title: %w", err)
	}
	return title, nil
}

// CardCount returns the number of cards in the collection.
func (c *Collection) CardCount() (int64, error) {
	count, err := c.col.Count()
	if err != nil {
		return 0, fmt.Errorf("counting cards: %w", err)
	}
	return count, nil
}

// ListJSON returns every card's summary as a JSON array of collection.CardSummary.
func (c *Collection) ListJSON() (string, error) {
	summaries, err := c.col.List()
	if err != nil {
		return "", fmt.Errorf("listing cards: %w", err)
	}
	return marshalJSONArray(summaries)
}

// SearchJSON runs a full-text search and returns the results as a JSON array
// of collection.SearchResult.
func (c *Collection) SearchJSON(query string) (string, error) {
	results, err := c.col.Search(query)
	if err != nil {
		return "", fmt.Errorf("searching collection: %w", err)
	}
	return marshalJSONArray(results)
}

// SearchFilteredJSON decodes a collection.SearchFilter from filterJSON, runs
// a field-scoped search, and returns the results as a JSON array of
// collection.SearchResult (the same shape SearchJSON returns).
func (c *Collection) SearchFilteredJSON(filterJSON string) (string, error) {
	var filter collection.SearchFilter
	if err := json.Unmarshal([]byte(filterJSON), &filter); err != nil {
		return "", fmt.Errorf("parsing search filter: %w", err)
	}

	results, err := c.col.SearchFiltered(filter)
	if err != nil {
		return "", fmt.Errorf("searching collection: %w", err)
	}
	return marshalJSONArray(results)
}

// CardMetaJSON returns the full postcard metadata (types.Metadata) for a
// card, as JSON.
func (c *Collection) CardMetaJSON(name string) (string, error) {
	meta, err := c.col.Metadata(name)
	if err != nil {
		return "", fmt.Errorf("reading metadata for %q: %w", name, err)
	}
	return marshalJSON(meta)
}

// Thumbnail returns the pre-generated JPEG thumbnail of the card's front image.
func (c *Collection) Thumbnail(name string) ([]byte, error) {
	thumb, err := c.col.Thumbnail(name)
	if err != nil {
		return nil, fmt.Errorf("reading thumbnail for %q: %w", name, err)
	}
	return thumb, nil
}

// CardImage returns the raw, untouched bytes of the card's stored web-format file.
func (c *Collection) CardImage(name string) ([]byte, error) {
	data, _, err := c.col.CardData(name)
	if err != nil {
		return nil, fmt.Errorf("reading image for %q: %w", name, err)
	}
	return data, nil
}

// CardMimetype returns the mimetype of the card's stored web-format file.
func (c *Collection) CardMimetype(name string) (string, error) {
	_, mimetype, err := c.col.CardData(name)
	if err != nil {
		return "", fmt.Errorf("reading mimetype for %q: %w", name, err)
	}
	return mimetype, nil
}
