package collection

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jphastings/dotpostcard/types"
)

const summaryColumns = `name, filename, mimetype, flip, sent_on, sender_name, recipient_name, location_name, country_code, latitude, longitude, front_px_w, front_px_h`

// rowScanner is satisfied by both *sql.Row and *sql.Rows.
type rowScanner interface {
	Scan(dest ...any) error
}

// queryRower is satisfied by both *sql.DB and *sql.Tx.
type queryRower interface {
	QueryRow(query string, args ...any) *sql.Row
}

// summaryScan holds the scan destinations for the columns listed in
// summaryColumns, so both a plain "cards" query and the joined FTS query in
// search.go can share the same scan/convert logic.
type summaryScan struct {
	summary CardSummary

	flip                                        string
	sentOn                                      sql.NullString
	senderName, recipientName, locationName, cc sql.NullString
	lat, lng                                    sql.NullFloat64
}

func (s *summaryScan) dest() []any {
	return []any{
		&s.summary.Name, &s.summary.Filename, &s.summary.Mimetype, &s.flip, &s.sentOn,
		&s.senderName, &s.recipientName, &s.locationName, &s.cc, &s.lat, &s.lng,
		&s.summary.FrontPxW, &s.summary.FrontPxH,
	}
}

func (s *summaryScan) result() CardSummary {
	s.summary.Flip = types.Flip(s.flip)
	s.summary.HasBack = s.summary.Flip != types.FlipNone
	s.summary.SentOn = parseDateString(s.sentOn)
	s.summary.SenderName = s.senderName.String
	s.summary.RecipientName = s.recipientName.String
	s.summary.LocationName = s.locationName.String
	s.summary.CountryCode = s.cc.String
	if s.lat.Valid && s.lng.Valid {
		lat, lng := s.lat.Float64, s.lng.Float64
		s.summary.Latitude = &lat
		s.summary.Longitude = &lng
	}
	return s.summary
}

func scanSummary(row rowScanner) (CardSummary, error) {
	var s summaryScan
	if err := row.Scan(s.dest()...); err != nil {
		return CardSummary{}, err
	}
	return s.result(), nil
}

func parseDateString(s sql.NullString) *types.Date {
	if !s.Valid || s.String == "" {
		return nil
	}
	t, err := time.Parse("2006-01-02", s.String)
	if err != nil {
		return nil
	}
	return &types.Date{Time: t}
}

func summaryByID(q queryRower, id int64) (CardSummary, error) {
	return scanSummary(q.QueryRow(`SELECT `+summaryColumns+` FROM cards WHERE id = ?`, id))
}

// List returns every card's summary, ordered by most-recently-sent first
// (undated cards last), then by name.
func (c *Collection) List() ([]CardSummary, error) {
	rows, err := c.db.Query(`SELECT ` + summaryColumns + ` FROM cards ORDER BY sent_on DESC NULLS LAST, name ASC`)
	if err != nil {
		return nil, fmt.Errorf("listing cards: %w", err)
	}
	defer rows.Close()

	var summaries []CardSummary
	for rows.Next() {
		summary, err := scanSummary(rows)
		if err != nil {
			return nil, fmt.Errorf("listing cards: %w", err)
		}
		summaries = append(summaries, summary)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("listing cards: %w", err)
	}

	return summaries, nil
}

// Count returns the number of cards in the collection.
func (c *Collection) Count() (int64, error) {
	var n int64
	if err := c.db.QueryRow(`SELECT COUNT(*) FROM cards`).Scan(&n); err != nil {
		return 0, fmt.Errorf("counting cards: %w", err)
	}
	return n, nil
}

// CardData returns the raw, untouched bytes of the card's web-format file, and its mimetype.
func (c *Collection) CardData(name string) ([]byte, string, error) {
	var data []byte
	var mimetype string

	err := c.db.QueryRow(`SELECT data, mimetype FROM cards WHERE name = ?`, name).Scan(&data, &mimetype)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, "", fmt.Errorf("card %q: %w", name, ErrNotFound)
	}
	if err != nil {
		return nil, "", fmt.Errorf("reading card %q: %w", name, err)
	}

	return data, mimetype, nil
}

// Thumbnail returns the pre-generated JPEG thumbnail of the card's front image.
func (c *Collection) Thumbnail(name string) ([]byte, error) {
	var thumb []byte

	err := c.db.QueryRow(`SELECT thumb FROM cards WHERE name = ?`, name).Scan(&thumb)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("card %q: %w", name, ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("reading thumbnail for %q: %w", name, err)
	}

	return thumb, nil
}

// Metadata returns the full postcard metadata (as decoded from the stored file) for a card.
func (c *Collection) Metadata(name string) (types.Metadata, error) {
	var metadataJSON string

	err := c.db.QueryRow(`SELECT metadata_json FROM cards WHERE name = ?`, name).Scan(&metadataJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return types.Metadata{}, fmt.Errorf("card %q: %w", name, ErrNotFound)
	}
	if err != nil {
		return types.Metadata{}, fmt.Errorf("reading metadata for %q: %w", name, err)
	}

	var sm storedMetadata
	if err := json.Unmarshal([]byte(metadataJSON), &sm); err != nil {
		return types.Metadata{}, fmt.Errorf("parsing metadata for %q: %w", name, err)
	}

	return sm.toMetadata(), nil
}
