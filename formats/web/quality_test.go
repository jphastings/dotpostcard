package web

import (
	"bytes"
	"image"
	"math/rand"
	"testing"

	"github.com/jphastings/dotpostcard/formats"
	"github.com/jphastings/dotpostcard/internal/testhelpers"
	"github.com/stretchr/testify/assert"
)

func TestQualityLevelsOrderBySizeAndArchiveIsLossless(t *testing.T) {
	noise := image.NewRGBA(image.Rect(0, 0, 400, 300))
	rand.New(rand.NewSource(1)).Read(noise.Pix)
	for i := 3; i < len(noise.Pix); i += 4 {
		noise.Pix[i] = 0xff
	}
	pc := testhelpers.SamplePostcard
	pc.Front, pc.Back = noise, nil

	codec, err := Codec("webp")
	assert.NoError(t, err)

	size := map[formats.Quality]int{}
	var archive []byte
	for _, q := range []formats.Quality{formats.Efficient, formats.Detail, formats.Archive} {
		fws, err := codec.Encode(pc, &formats.EncodeOptions{Quality: q})
		assert.NoError(t, err)
		data, err := fws[0].Bytes()
		assert.NoError(t, err)
		size[q] = len(data)
		archive = data
	}

	assert.Less(t, size[formats.Efficient], size[formats.Detail])
	assert.Less(t, size[formats.Detail], size[formats.Archive])
	assert.True(t, bytes.Contains(archive, []byte("VP8L")))
}

func TestParseQualityRejectsUnknownNames(t *testing.T) {
	_, err := formats.ParseQuality("best")
	assert.ErrorContains(t, err, "efficient, detail, archive")
}
