package images_test

import (
	"image"
	"testing"

	"github.com/jphastings/dotpostcard/internal/images"
	"github.com/jphastings/dotpostcard/internal/testhelpers"
	"github.com/stretchr/testify/assert"
)

func TestHasTransparency(t *testing.T) {
	opaque := image.NewNRGBA(image.Rect(0, 0, 200, 150))
	for i := 3; i < len(opaque.Pix); i += 4 {
		opaque.Pix[i] = 255
	}
	assert.False(t, images.HasTransparency(opaque), "fully opaque image")

	// A junk alpha channel: a few stray pixels slightly below opaque, as
	// scanners and exports sometimes produce, must not count as transparency
	noisy := image.NewNRGBA(image.Rect(0, 0, 200, 150))
	for i := 3; i < len(noisy.Pix); i += 4 {
		noisy.Pix[i] = 255
	}
	noisy.Pix[3] = 228 // pixel (0,0) at ~89% alpha
	noisy.Pix[403] = 180
	assert.False(t, images.HasTransparency(noisy), "stray near-opaque noise pixels")

	assert.True(t, images.HasTransparency(testhelpers.TestImages["sample-transparency-front.png"]),
		"genuinely transparent image")
}
