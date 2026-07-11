package collection

import (
	"bytes"
	"image"
	"io"
	"path/filepath"
	"testing"

	"github.com/jphastings/dotpostcard/formats"
	"github.com/jphastings/dotpostcard/formats/web"
	"github.com/jphastings/dotpostcard/internal/testhelpers"
	"github.com/jphastings/dotpostcard/types"
	"github.com/stretchr/testify/assert"
)

// encodeSample encodes testhelpers.SamplePostcard via the web codec, then
// independently decodes the result again. Encoding resamples pixel
// dimensions and round-trips metadata through XMP (which, for example, marks
// secret regions as "prehidden"), so wantMeta/wantFront — not
// testhelpers.SamplePostcard.Meta directly — are what AddWebPostcard should
// also arrive at when decoding the exact same bytes.
func encodeSample(t *testing.T) (data []byte, filename string, wantMeta types.Metadata, wantFront image.Image) {
	t.Helper()

	// testhelpers.SamplePostcard.Front/Back are nil: fixtures.go builds that
	// struct literal from testhelpers.TestImages before TestImages is
	// populated (that happens in an init() func, which Go runs after
	// package-level var initializers). Substitute the real images here
	// rather than editing the shared fixture file.
	pc := testhelpers.SamplePostcard
	pc.Front = testhelpers.TestImages["sample-front.png"]
	pc.Back = testhelpers.TestImages["sample-back.png"]
	assert.NotNil(t, pc.Front)
	assert.NotNil(t, pc.Back)

	fws, err := web.DefaultCodec.Encode(pc, nil)
	assert.NoError(t, err)
	assert.NotEmpty(t, fws)

	data, err = fws[0].Bytes()
	assert.NoError(t, err)
	filename = fws[0].Filename

	decoded, err := web.BundleFromReader(io.NopCloser(bytes.NewReader(data)), filename).Decode(formats.DecodeOptions{})
	assert.NoError(t, err)

	return data, filename, decoded.Meta, decoded.Front
}

// encodeNamed encodes a copy of testhelpers.SamplePostcard with its Name and
// Meta.SentOn overridden, so tests can craft several distinct cards.
func encodeNamed(t *testing.T, name string, sentOn *types.Date) (data []byte, filename string) {
	t.Helper()

	pc := testhelpers.SamplePostcard
	pc.Name = name
	pc.Meta.SentOn = sentOn
	pc.Front = testhelpers.TestImages["sample-front.png"]
	pc.Back = testhelpers.TestImages["sample-back.png"]
	assert.NotNil(t, pc.Front)
	assert.NotNil(t, pc.Back)

	fws, err := web.DefaultCodec.Encode(pc, nil)
	assert.NoError(t, err)
	assert.NotEmpty(t, fws)

	data, err = fws[0].Bytes()
	assert.NoError(t, err)

	return data, fws[0].Filename
}

// encodeTransparentSample encodes a single-sided postcard from
// testhelpers' transparency sample, flagged as having transparency, so
// tests can exercise the thumbnail's alpha-preserving path.
func encodeTransparentSample(t *testing.T) (data []byte, filename string) {
	t.Helper()

	pc := testhelpers.SamplePostcard
	pc.Name = "transparent-postcard"
	pc.Front = testhelpers.TestImages["sample-transparency-front.png"]
	pc.Back = nil
	pc.Meta.Flip = types.FlipNone
	pc.Meta.HasTransparency = true
	assert.NotNil(t, pc.Front)

	// A non-nil, zero-value EncodeOptions is used (rather than nil, as the
	// other encode* helpers pass) because formats/web.codec.Encode
	// dereferences opts.Archival directly for the webp/png cases without the
	// nil-safe handling its other opts accesses use; that's a latent,
	// pre-existing bug only reachable when encoding a transparent postcard
	// with nil options, which no production call site does.
	fws, err := web.DefaultCodec.Encode(pc, &formats.EncodeOptions{})
	assert.NoError(t, err)
	assert.NotEmpty(t, fws)

	data, err = fws[0].Bytes()
	assert.NoError(t, err)

	return data, fws[0].Filename
}

// encodeCard encodes a copy of testhelpers.SamplePostcard named name, after
// applying mutate to its metadata, so search-filter tests can craft cards
// with distinct sender/recipient/location/context-author/sent-on fields.
func encodeCard(t *testing.T, name string, mutate func(*types.Metadata)) (data []byte, filename string) {
	t.Helper()

	pc := testhelpers.SamplePostcard
	pc.Name = name
	pc.Front = testhelpers.TestImages["sample-front.png"]
	pc.Back = testhelpers.TestImages["sample-back.png"]
	assert.NotNil(t, pc.Front)
	assert.NotNil(t, pc.Back)
	mutate(&pc.Meta)

	fws, err := web.DefaultCodec.Encode(pc, nil)
	assert.NoError(t, err)
	assert.NotEmpty(t, fws)

	data, err = fws[0].Bytes()
	assert.NoError(t, err)

	return data, fws[0].Filename
}

func mustCreate(t *testing.T) *Collection {
	t.Helper()

	col, err := Create(filepath.Join(t.TempDir(), "test.postcards"))
	assert.NoError(t, err)
	t.Cleanup(func() { col.Close() })

	return col
}
