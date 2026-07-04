package collection

import (
	"bytes"
	"image/jpeg"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestThumbnail(t *testing.T) {
	data, filename, _, _ := encodeSample(t)
	col := mustCreate(t)

	summary, err := col.AddWebPostcard(filename, data)
	assert.NoError(t, err)

	thumb, err := col.Thumbnail(summary.Name)
	assert.NoError(t, err)

	img, err := jpeg.Decode(bytes.NewReader(thumb))
	assert.NoError(t, err)

	bounds := img.Bounds()
	assert.LessOrEqual(t, bounds.Dx(), thumbnailMaxDimension)
	assert.LessOrEqual(t, bounds.Dy(), thumbnailMaxDimension)
	assert.True(t, bounds.Dx() == thumbnailMaxDimension || bounds.Dy() == thumbnailMaxDimension,
		"expected thumbnail to be scaled up to the max dimension on at least one axis")

	_, err = col.Thumbnail("no-such-card")
	assert.ErrorIs(t, err, ErrNotFound)
}
