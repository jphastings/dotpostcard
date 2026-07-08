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
