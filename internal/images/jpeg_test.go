package images

import (
	"image"
	"image/color"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJPEGQualityForSizeIsTheHighestThatFits(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	img := image.NewNRGBA(image.Rect(0, 0, 64, 64))
	for i := range img.Pix {
		img.Pix[i] = uint8(rng.Intn(256))
		if i%4 == 3 {
			img.Pix[i] = 255
		}
	}
	bg := color.RGBA{255, 255, 255, 255}
	target, err := encodeJPEG(img, bg, 60)
	require.NoError(t, err)
	budget := len(target)

	q := JPEGQualityForSize(img, bg, budget)

	fits, err := encodeJPEG(img, bg, q)
	require.NoError(t, err)
	assert.LessOrEqual(t, len(fits), budget)
	tooBig, err := encodeJPEG(img, bg, q+1)
	require.NoError(t, err)
	assert.Greater(t, len(tooBig), budget)
}
