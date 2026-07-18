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

	meta, mimetype, err := decodeCardMeta(path, data)
	if err != nil {
		return nil, err
	}

	return &CardFile{
		path:     path,
		name:     nameFromFilename(path),
		mimetype: mimetype,
		data:     data,
		meta:     meta,
	}, nil
}

// decodeCardMeta extracts and decodes a compiled web-format postcard's
// embedded XMP metadata straight from its bytes, without ever decoding
// pixel data. filename drives mimetype sniffing's error context (mimetype
// itself is sniffed from data, not the name) and is only used in wrapped
// error messages, so it need not be a real filesystem path — MetaJSONFromCardBytes
// reuses this to work from raw bytes handed across the gomobile bridge.
func decodeCardMeta(filename string, data []byte) (types.Metadata, string, error) {
	mimetype, extractXMP, err := xmpExtractorFor(filename, data)
	if err != nil {
		return types.Metadata{}, "", err
	}

	xmpData, err := extractXMP(data)
	if err != nil {
		return types.Metadata{}, "", fmt.Errorf("reading XMP from %s: %w", filename, err)
	}

	pc, err := xmp.BundleFromBytes(xmpData, filename).Decode(formats.DecodeOptions{})
	if err != nil {
		return types.Metadata{}, "", fmt.Errorf("decoding metadata from %s: %w", filename, err)
	}

	return pc.Meta, mimetype, nil
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
	return metadataJSON(f.meta)
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

func xmpExtractorFor(filename string, data []byte) (mimetype string, extract func([]byte) ([]byte, error), err error) {
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
		return "", nil, fmt.Errorf("unsupported postcard file extension: %s", filename)
	}
}

// jsonSafeMetadata mirrors types.Metadata, but swaps each side's secrets for
// types.SecretPolygon. types.Polygon only implements json.Unmarshaler, and
// its UnmarshalJSON requires a "type": "polygon"/"box" discriminator that a
// bare reflection Marshal of types.Metadata never writes (see
// pkg/collection/metadata.go's storedMetadata doc comment, which works
// around the same quirk for on-disk collection storage) — so a bare
// json.Marshal of types.Metadata can't be fed back into CompilePostcard once
// secrets are present. Embedding types.Metadata and re-declaring Front/Back
// here overrides just those two fields in the marshalled output, leaving
// every other field's encoding untouched.
type jsonSafeMetadata struct {
	types.Metadata
	Front jsonSafeSide `json:"front,omitempty"`
	Back  jsonSafeSide `json:"back,omitempty"`
}

type jsonSafeSide struct {
	Description   string                `json:"description,omitempty"`
	Transcription types.AnnotatedText   `json:"transcription,omitempty"`
	Secrets       []types.SecretPolygon `json:"secrets,omitempty"`
}

func toJSONSafeSide(side types.Side) jsonSafeSide {
	var secrets []types.SecretPolygon
	if len(side.Secrets) > 0 {
		secrets = make([]types.SecretPolygon, len(side.Secrets))
		for i, p := range side.Secrets {
			secrets[i] = types.SecretPolygon{Type: "polygon", Prehidden: p.Prehidden, Points: p.Points}
		}
	}

	return jsonSafeSide{
		Description:   side.Description,
		Transcription: side.Transcription,
		Secrets:       secrets,
	}
}

// metadataJSON marshals meta to the canonical metadata JSON shape every
// appcore entry point returning types.Metadata uses — CardFile.MetaJSON,
// MetaJSONFromCardBytes and MetaJSONFromComponentYAML — so its secrets
// round-trip straight back into CompilePostcard.
func metadataJSON(meta types.Metadata) (string, error) {
	return marshalJSON(jsonSafeMetadata{
		Metadata: meta,
		Front:    toJSONSafeSide(meta.Front),
		Back:     toJSONSafeSide(meta.Back),
	})
}

// nameFromFilename mirrors formats/web.BundleFromReader's derivation of a
// card's name from its filename.
func nameFromFilename(path string) string {
	base := filepath.Base(path)
	return strings.TrimSuffix(strings.TrimSuffix(base, filepath.Ext(base)), ".postcard")
}
