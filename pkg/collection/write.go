package collection

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/jphastings/dotpostcard/formats"
	"github.com/jphastings/dotpostcard/formats/web"
	"github.com/jphastings/dotpostcard/types"
)

// cardRow holds every value that gets written to the cards table for one card.
type cardRow struct {
	name       string
	filename   string
	mimetype   string
	data       []byte
	fileSHA256 string
	thumb      []byte

	metadataJSON string

	flip               string
	sentOn             any // string or nil
	locale             string
	senderName         string
	recipientName      string
	locationName       string
	countryCode        string
	latitude           *float64
	longitude          *float64
	frontPxW           int
	frontPxH           int
	frontDescription   string
	backDescription    string
	frontTranscript    string
	backTranscript     string
	contextDescription string
	contextAuthorName  string
	addedAt            string
}

func (r cardRow) insertArgs() []any {
	return []any{
		r.name, r.filename, r.mimetype, r.data, r.fileSHA256, r.thumb, r.metadataJSON,
		r.flip, r.sentOn, r.locale, r.senderName, r.recipientName, r.locationName, r.countryCode,
		r.latitude, r.longitude, r.frontPxW, r.frontPxH,
		r.frontDescription, r.backDescription, r.frontTranscript, r.backTranscript,
		r.contextDescription, r.contextAuthorName, r.addedAt,
	}
}

func (r cardRow) updateArgs(id int64) []any {
	return []any{
		r.filename, r.mimetype, r.data, r.fileSHA256, r.thumb, r.metadataJSON,
		r.flip, r.sentOn, r.locale, r.senderName, r.recipientName, r.locationName, r.countryCode,
		r.latitude, r.longitude, r.frontPxW, r.frontPxH,
		r.frontDescription, r.backDescription, r.frontTranscript, r.backTranscript,
		r.contextDescription, r.contextAuthorName, r.addedAt, id,
	}
}

const insertCardSQL = `INSERT INTO cards (
	name, filename, mimetype, data, file_sha256, thumb, metadata_json,
	flip, sent_on, locale, sender_name, recipient_name, location_name, country_code,
	latitude, longitude, front_px_w, front_px_h,
	front_description, back_description, front_transcript, back_transcript,
	context_description, context_author_name, added_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

const updateCardSQL = `UPDATE cards SET
	filename = ?, mimetype = ?, data = ?, file_sha256 = ?, thumb = ?, metadata_json = ?,
	flip = ?, sent_on = ?, locale = ?, sender_name = ?, recipient_name = ?, location_name = ?, country_code = ?,
	latitude = ?, longitude = ?, front_px_w = ?, front_px_h = ?,
	front_description = ?, back_description = ?, front_transcript = ?, back_transcript = ?,
	context_description = ?, context_author_name = ?, added_at = ?
WHERE id = ?`

// AddWebPostcard decodes a web-format postcard file (a *.postcard.{webp,jpg,jpeg,png}
// image with embedded XMP metadata) and stores it in the collection. Adding a
// file whose name already exists in the collection replaces it unless the
// bytes are byte-for-byte identical, in which case it's a no-op.
func (c *Collection) AddWebPostcard(filename string, data []byte) (CardSummary, error) {
	if c.readOnly {
		return CardSummary{}, errReadOnly
	}

	mimetype, err := Mimetype(filename)
	if err != nil {
		mimetype, err = MimetypeFromData(data)
		if err != nil {
			return CardSummary{}, err
		}
	}

	pc, err := web.BundleFromReader(io.NopCloser(bytes.NewReader(data)), filename).Decode(formats.DecodeOptions{})
	if err != nil {
		return CardSummary{}, fmt.Errorf("decoding %s: %w", filename, err)
	}

	metadataJSON, err := json.Marshal(toStoredMetadata(pc.Meta))
	if err != nil {
		return CardSummary{}, fmt.Errorf("encoding metadata for %s: %w", filename, err)
	}

	thumb, err := makeThumbnail(pc.Front, pc.Meta.HasTransparency)
	if err != nil {
		return CardSummary{}, fmt.Errorf("generating thumbnail for %s: %w", filename, err)
	}

	sum := sha256.Sum256(data)
	frontBounds := pc.Front.Bounds()

	row := cardRow{
		name:               pc.Name,
		filename:           filename,
		mimetype:           mimetype,
		data:               data,
		fileSHA256:         hex.EncodeToString(sum[:]),
		thumb:              thumb,
		metadataJSON:       string(metadataJSON),
		flip:               string(pc.Meta.Flip),
		sentOn:             dateArg(pc.Meta.SentOn),
		locale:             pc.Meta.Locale,
		senderName:         pc.Meta.Sender.Name,
		recipientName:      pc.Meta.Recipient.Name,
		locationName:       pc.Meta.Location.Name,
		countryCode:        pc.Meta.Location.CountryCode,
		latitude:           pc.Meta.Location.Latitude,
		longitude:          pc.Meta.Location.Longitude,
		frontPxW:           frontBounds.Dx(),
		frontPxH:           frontBounds.Dy(),
		frontDescription:   pc.Meta.Front.Description,
		backDescription:    pc.Meta.Back.Description,
		frontTranscript:    pc.Meta.Front.Transcription.Text,
		backTranscript:     pc.Meta.Back.Transcription.Text,
		contextDescription: pc.Meta.Context.Description,
		contextAuthorName:  pc.Meta.Context.Author.Name,
		addedAt:            time.Now().UTC().Format(time.RFC3339),
	}

	return c.upsertCard(row)
}

func (c *Collection) upsertCard(row cardRow) (CardSummary, error) {
	tx, err := c.db.Begin()
	if err != nil {
		return CardSummary{}, fmt.Errorf("adding %s: %w", row.name, err)
	}
	defer tx.Rollback()

	var id int64
	var existingSHA string
	err = tx.QueryRow(`SELECT id, file_sha256 FROM cards WHERE name = ?`, row.name).Scan(&id, &existingSHA)

	switch {
	case errors.Is(err, sql.ErrNoRows):
		res, err := tx.Exec(insertCardSQL, row.insertArgs()...)
		if err != nil {
			return CardSummary{}, fmt.Errorf("adding %s: %w", row.name, err)
		}
		if id, err = res.LastInsertId(); err != nil {
			return CardSummary{}, fmt.Errorf("adding %s: %w", row.name, err)
		}

	case err != nil:
		return CardSummary{}, fmt.Errorf("adding %s: %w", row.name, err)

	case existingSHA == row.fileSHA256:
		summary, err := summaryByID(tx, id)
		if err != nil {
			return CardSummary{}, fmt.Errorf("adding %s: %w", row.name, err)
		}
		return summary, tx.Commit()

	default:
		if _, err := tx.Exec(updateCardSQL, row.updateArgs(id)...); err != nil {
			return CardSummary{}, fmt.Errorf("adding %s: %w", row.name, err)
		}
	}

	summary, err := summaryByID(tx, id)
	if err != nil {
		return CardSummary{}, fmt.Errorf("adding %s: %w", row.name, err)
	}

	return summary, tx.Commit()
}

// Remove deletes the named card (and its FTS entry) from the collection.
func (c *Collection) Remove(name string) error {
	if c.readOnly {
		return errReadOnly
	}

	res, err := c.db.Exec(`DELETE FROM cards WHERE name = ?`, name)
	if err != nil {
		return fmt.Errorf("removing %s: %w", name, err)
	}

	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("removing %s: %w", name, err)
	}
	if n == 0 {
		return fmt.Errorf("removing %s: %w", name, ErrNotFound)
	}

	return nil
}

func dateArg(d *types.Date) any {
	if d == nil || d.Time.IsZero() {
		return nil
	}
	return d.Time.Format("2006-01-02")
}

// Mimetype returns the image mimetype implied by a web-format postcard
// filename's extension (.webp, .jpg/.jpeg or .png).
func Mimetype(filename string) (string, error) {
	switch {
	case strings.HasSuffix(filename, ".webp"):
		return "image/webp", nil
	case strings.HasSuffix(filename, ".jpg"), strings.HasSuffix(filename, ".jpeg"):
		return "image/jpeg", nil
	case strings.HasSuffix(filename, ".png"):
		return "image/png", nil
	default:
		return "", fmt.Errorf("unsupported postcard file extension: %s", filename)
	}
}

// MimetypeFromData returns the image mimetype implied by a web-format
// postcard's leading magic bytes, for files (like `*.postcard`) whose
// name doesn't reveal the codec.
func MimetypeFromData(data []byte) (string, error) {
	switch {
	case len(data) >= 2 && data[0] == 0xFF && data[1] == 0xD8:
		return "image/jpeg", nil
	case len(data) >= 8 && bytes.Equal(data[:8], []byte("\x89PNG\r\n\x1a\n")):
		return "image/png", nil
	case len(data) >= 12 && bytes.Equal(data[0:4], []byte("RIFF")) && bytes.Equal(data[8:12], []byte("WEBP")):
		return "image/webp", nil
	default:
		return "", fmt.Errorf("unrecognized image data: unknown magic bytes")
	}
}
