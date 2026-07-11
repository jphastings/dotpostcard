package appcore

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jphastings/dotpostcard/formats"
	"github.com/jphastings/dotpostcard/formats/xmp"
	"github.com/jphastings/dotpostcard/pkg/collection"
	"github.com/jphastings/dotpostcard/pkg/xmpinject"
	"github.com/jphastings/dotpostcard/types"
)

// CardFile is a bare *.postcard.{webp,jpg,jpeg,png} file, opened outside of
// any collection. Its metadata is read straight from the embedded XMP block
// via pkg/xmpinject, without ever decoding the image's pixels.
type CardFile struct {
	path     string
	name     string
	mimetype string
	data     []byte
	meta     types.Metadata
}

// OpenCardFile reads a bare postcard file's bytes and decodes its embedded
// XMP metadata. It never decodes pixel data.
func OpenCardFile(path string) (*CardFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	mimetype, extractXMP, err := xmpExtractorFor(path, data)
	if err != nil {
		return nil, err
	}

	xmpData, err := extractXMP(data)
	if err != nil {
		return nil, fmt.Errorf("reading XMP from %s: %w", path, err)
	}

	pc, err := xmp.BundleFromBytes(xmpData, path).Decode(formats.DecodeOptions{})
	if err != nil {
		return nil, fmt.Errorf("decoding metadata from %s: %w", path, err)
	}

	return &CardFile{
		path:     path,
		name:     nameFromFilename(path),
		mimetype: mimetype,
		data:     data,
		meta:     pc.Meta,
	}, nil
}

// Name returns the card's name: the file's basename, with its extension and
// trailing ".postcard" stripped (e.g. "some-card.postcard.webp" -> "some-card").
func (f *CardFile) Name() string {
	return f.name
}

// Path returns the filesystem path this file was opened from.
func (f *CardFile) Path() string {
	return f.path
}

// MetaJSON returns the full postcard metadata (types.Metadata), as JSON.
func (f *CardFile) MetaJSON() (string, error) {
	return marshalJSON(f.meta)
}

// SummaryJSON returns the card's summary, in the same JSON shape as
// collection.CardSummary.
func (f *CardFile) SummaryJSON() (string, error) {
	return marshalJSON(f.summary())
}

// Image returns the file's raw, untouched bytes.
func (f *CardFile) Image() ([]byte, error) {
	return f.data, nil
}

// summary builds a collection.CardSummary from the file's XMP-decoded
// metadata. Front pixel dimensions come straight from meta.Physical.FrontDimensions,
// which formats/xmp already halves against the stored (front+back stacked)
// height when the card has a back; it's 0 when the XMP didn't carry a size.
func (f *CardFile) summary() collection.CardSummary {
	meta := f.meta
	return collection.CardSummary{
		Name:          f.name,
		Filename:      filepath.Base(f.path),
		Mimetype:      f.mimetype,
		Flip:          meta.Flip,
		SentOn:        meta.SentOn,
		SenderName:    meta.Sender.Name,
		RecipientName: meta.Recipient.Name,
		LocationName:  meta.Location.Name,
		CountryCode:   meta.Location.CountryCode,
		Latitude:      meta.Location.Latitude,
		Longitude:     meta.Location.Longitude,
		FrontPxW:      meta.Physical.FrontDimensions.PxWidth,
		FrontPxH:      meta.Physical.FrontDimensions.PxHeight,
		HasBack:       meta.Flip != types.FlipNone,
	}
}

// matchesFilter reports whether the card file satisfies every populated
// field of filter, mirroring collection.SearchFiltered's OR-within-a-field,
// AND-across-fields semantics without a database: name filters
// case-insensitively substring-match the person's name, URI values
// exact-match Person.Uri, country exact-matches the location's country code,
// and text keeps Library.SearchJSON's existing substring behaviour.
func (f *CardFile) matchesFilter(filter collection.SearchFilter) bool {
	if text := strings.ToLower(strings.TrimSpace(filter.Text)); text != "" && !strings.Contains(strings.ToLower(f.searchableText()), text) {
		return false
	}
	if len(filter.From) > 0 && !personMatchesAny(f.meta.Sender, filter.From) {
		return false
	}
	if len(filter.To) > 0 && !personMatchesAny(f.meta.Recipient, filter.To) {
		return false
	}
	if len(filter.With) > 0 && !personMatchesAny(f.meta.Sender, filter.With) && !personMatchesAny(f.meta.Recipient, filter.With) {
		return false
	}
	if len(filter.Collector) > 0 && !personMatchesAny(f.meta.Context.Author, filter.Collector) {
		return false
	}
	if len(filter.Country) > 0 && !containsString(filter.Country, f.meta.Location.CountryCode) {
		return false
	}
	if (filter.SentFrom != "" || filter.SentUntil != "") && !sentOnInRange(f.meta.SentOn, filter.SentFrom, filter.SentUntil) {
		return false
	}
	return true
}

// personMatchesAny reports whether any of values matches p: URI-form values
// (see collection.IsPersonURI) exact-match p.Uri, name-form values
// case-insensitively substring-match p.Name.
func personMatchesAny(p types.Person, values []string) bool {
	for _, v := range values {
		if collection.IsPersonURI(v) {
			if p.Uri == v {
				return true
			}
			continue
		}
		if p.Name != "" && strings.Contains(strings.ToLower(p.Name), strings.ToLower(v)) {
			return true
		}
	}
	return false
}

func containsString(values []string, s string) bool {
	for _, v := range values {
		if v == s {
			return true
		}
	}
	return false
}

// sentOnInRange reports whether sentOn falls within [from, until) using the
// same ISO "yyyy-MM-dd" lexicographic comparison collection.SearchFiltered's
// SQL does; an undated card (sentOn == nil) never matches once either bound
// is given.
func sentOnInRange(sentOn *types.Date, from, until string) bool {
	if sentOn == nil {
		return false
	}
	s := sentOn.Time.Format("2006-01-02")
	if from != "" && s < from {
		return false
	}
	if until != "" && s >= until {
		return false
	}
	return true
}

// searchableText concatenates every field a Library substring search should
// consider for a bare card file.
func (f *CardFile) searchableText() string {
	return strings.Join([]string{
		f.name,
		f.meta.Sender.Name,
		f.meta.Recipient.Name,
		f.meta.Location.Name,
		f.meta.Front.Description,
		f.meta.Back.Description,
		f.meta.Front.Transcription.Text,
		f.meta.Back.Transcription.Text,
	}, " ")
}

func xmpExtractorFor(path string, data []byte) (mimetype string, extract func([]byte) ([]byte, error), err error) {
	mimetype, err = collection.MimetypeFromData(data)
	if err != nil {
		return "", nil, err
	}

	switch mimetype {
	case "image/webp":
		return mimetype, xmpinject.XMPfromWebP, nil
	case "image/jpeg":
		return mimetype, xmpinject.XMPfromJPEG, nil
	case "image/png":
		return mimetype, xmpinject.XMPfromPNG, nil
	default:
		return "", nil, fmt.Errorf("unsupported postcard file extension: %s", path)
	}
}

// nameFromFilename mirrors formats/web.BundleFromReader's derivation of a
// card's name from its filename.
func nameFromFilename(path string) string {
	base := filepath.Base(path)
	return strings.TrimSuffix(strings.TrimSuffix(base, filepath.Ext(base)), ".postcard")
}
