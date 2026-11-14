package web

import (
	"bytes"
	"image"
	"io"
	"testing"

	"github.com/jphastings/dotpostcard/formats"
	"github.com/jphastings/dotpostcard/internal/images"
	"github.com/jphastings/dotpostcard/internal/testhelpers"
	"github.com/jphastings/dotpostcard/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func decodeCard(t *testing.T, data []byte) types.Postcard {
	t.Helper()
	pc, err := BundleFromReader(io.NopCloser(bytes.NewReader(data)), "card.postcard").Decode(formats.DecodeOptions{})
	require.NoError(t, err)
	return pc
}

func assertSamePixels(t *testing.T, want, got image.Image) {
	t.Helper()
	assert.Equal(t, images.Turn(want, 0), images.Turn(got, 0))
}

func TestRotateTurnsTheFrontLosslesslyAndRestores(t *testing.T) {
	pc := testhelpers.SamplePostcard
	pc.Front = testhelpers.TestImages["sample-front.png"]
	pc.Back = testhelpers.TestImages["sample-back.png"]
	fws, err := PostcardCodec.Encode(pc, &formats.EncodeOptions{Quality: formats.Archive})
	require.NoError(t, err)
	original, err := fws[0].Bytes()
	require.NoError(t, err)
	before := decodeCard(t, original)

	turnedData, flip, err := Rotate(original, 1, 0)
	require.NoError(t, err)
	turned := decodeCard(t, turnedData)

	assert.Equal(t, types.FlipLeftHand, flip)
	assert.Equal(t, types.FlipLeftHand, turned.Meta.Flip)
	assert.Equal(t, before.Meta.Physical.FrontDimensions.CmHeight, turned.Meta.Physical.FrontDimensions.CmWidth)
	assertSamePixels(t, images.Turn(before.Front, 1), turned.Front)
	assertSamePixels(t, before.Back, turned.Back)

	restoredData, flip, err := Rotate(turnedData, -1, 0)
	require.NoError(t, err)
	restored := decodeCard(t, restoredData)

	assert.Equal(t, types.FlipBook, flip)
	assertSamePixels(t, before.Front, restored.Front)
	assertSamePixels(t, before.Back, restored.Back)
}
