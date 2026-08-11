package xmpinject_test

import (
	"bytes"
	"image/jpeg"
	"os"
	"testing"

	"github.com/jphastings/dotpostcard/internal/testhelpers"
	"github.com/jphastings/dotpostcard/pkg/xmpinject"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These mirror the unexported layout constants in jpeg.go (derived from
// Adobe's XMP Specification Part 3); they're duplicated here, rather than
// exported, so the boundary tests exercise the public API like any caller
// would.
const (
	maxSingleSegmentXMPsize = 65504
	maxExtendedChunkSize    = 65458
)

// extendedXMPSignature is the Adobe-defined marker that begins every
// ExtendedXMP APP1 segment's content.
var extendedXMPSignature = []byte("http://ns.adobe.com/xmp/extension/\x00")

// xmpishPayload deterministically generates n bytes of XMP-shaped filler
// text, for exercising sizes no real fixture happens to have.
func xmpishPayload(n int) []byte {
	const pattern = `<rdf:li>Sample XMP-ish repeated content for boundary testing 0123456789</rdf:li>`
	buf := make([]byte, 0, n)
	for len(buf) < n {
		buf = append(buf, pattern...)
	}
	return buf[:n]
}

func TestXMPintoJPEG(t *testing.T) {
	original, err := os.ReadFile("1px-nometa.jpg")
	assert.NoError(t, err)
	want, err := os.ReadFile("1px-xmp.jpg")
	assert.NoError(t, err)

	var b bytes.Buffer
	assert.NoError(t, xmpinject.XMPintoJPEG(&b, original, testhelpers.SampleXMP))

	// It's not ideal; but if I need to recreate the fixture data I use this
	// f, _ := os.OpenFile("1px-xmp.jpg", os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0644)
	// _ = xmpinject.XMPintoJPEG(f, original, testhelpers.SampleXMP)

	assert.Equal(t, want, b.Bytes())
}

func TestXMPfromJPEG(t *testing.T) {
	original, err := os.ReadFile("1px-xmp.jpg")
	assert.NoError(t, err)

	xmpData, err := xmpinject.XMPfromJPEG(original)
	assert.NoError(t, err)

	assert.Equal(t, testhelpers.SampleXMP, xmpData)
}

func TestXMPRoundTrip_LargePayload(t *testing.T) {
	original, err := os.ReadFile("1px-nometa.jpg")
	require.NoError(t, err)

	xmpData := xmpishPayload(200_000)

	var b bytes.Buffer
	require.NoError(t, xmpinject.XMPintoJPEG(&b, original, xmpData))

	got, err := xmpinject.XMPfromJPEG(b.Bytes())
	assert.NoError(t, err)
	assert.Equal(t, xmpData, got)
}

func TestXMPRoundTrip_BoundarySizes(t *testing.T) {
	original, err := os.ReadFile("1px-nometa.jpg")
	require.NoError(t, err)

	sizes := map[string]int{
		"single-segment ceiling":     maxSingleSegmentXMPsize,
		"one over single-segment":    maxSingleSegmentXMPsize + 1,
		"one over an extended chunk": maxExtendedChunkSize + 1,
	}

	for name, size := range sizes {
		t.Run(name, func(t *testing.T) {
			xmpData := xmpishPayload(size)

			var b bytes.Buffer
			require.NoError(t, xmpinject.XMPintoJPEG(&b, original, xmpData))

			got, err := xmpinject.XMPfromJPEG(b.Bytes())
			assert.NoError(t, err)
			assert.Equal(t, xmpData, got)
		})
	}
}

func TestXMPintoJPEG_CeilingStaysSingleSegment(t *testing.T) {
	original, err := os.ReadFile("1px-nometa.jpg")
	require.NoError(t, err)

	xmpData := xmpishPayload(maxSingleSegmentXMPsize)

	var b bytes.Buffer
	require.NoError(t, xmpinject.XMPintoJPEG(&b, original, xmpData))

	assert.Equal(t, 0, bytes.Count(b.Bytes(), extendedXMPSignature), "a payload at the single-segment ceiling must not use the ExtendedXMP mechanism")
}

func TestXMPintoJPEG_MultiSegmentOutputIsDecodableJPEG(t *testing.T) {
	original, err := os.ReadFile("1px-nometa.jpg")
	require.NoError(t, err)

	xmpData := xmpishPayload(200_000)

	var b bytes.Buffer
	require.NoError(t, xmpinject.XMPintoJPEG(&b, original, xmpData))

	_, err = jpeg.Decode(bytes.NewReader(b.Bytes()))
	assert.NoError(t, err)
}

func TestXMPfromJPEG_DetectsCorruptedExtendedXMP(t *testing.T) {
	original, err := os.ReadFile("1px-nometa.jpg")
	require.NoError(t, err)

	xmpData := xmpishPayload(200_000)

	var b bytes.Buffer
	require.NoError(t, xmpinject.XMPintoJPEG(&b, original, xmpData))
	corrupted := b.Bytes()

	sigStart := bytes.Index(corrupted, extendedXMPSignature)
	require.NotEqual(t, -1, sigStart, "expected an ExtendedXMP segment in the output")

	// Flip a byte inside the first chunk's data, well past the
	// signature/GUID/total/offset header, without touching any length
	// field, so only the MD5 check can catch the corruption.
	dataStart := sigStart + len(extendedXMPSignature) + 32 /* GUID */ + 4 /* total */ + 4 /* offset */
	corrupted[dataStart] ^= 0xFF

	_, err = xmpinject.XMPfromJPEG(corrupted)
	assert.Error(t, err)
}

func TestXMPfromJPEG_NoXMP(t *testing.T) {
	original, err := os.ReadFile("1px-nometa.jpg")
	require.NoError(t, err)

	_, err = xmpinject.XMPfromJPEG(original)
	assert.EqualError(t, err, "this JPEG isn't a web format postcard (it doesn't have XMP data as its first APP1 chunk)")
}
