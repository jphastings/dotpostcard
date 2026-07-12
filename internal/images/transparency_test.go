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

	// A thin fully-transparent sliver, as scanner deskew sometimes leaves at
	// a frame edge, is junk: it has no solid transparent core
	sliver := image.NewNRGBA(image.Rect(0, 0, 200, 150))
	for i := 3; i < len(sliver.Pix); i += 4 {
		sliver.Pix[i] = 255
	}
	for x := 40; x < 140; x++ {
		for t := 0; t < 3; t++ { // 3px-thick diagonal streak: ~300px total
			sliver.Pix[((x/2+t)*200+x)*4+3] = 0
		}
	}
	assert.False(t, images.HasTransparency(sliver), "thin transparent sliver is junk")

	// A small solid blob — like one die-cut rounded corner — is genuine
	corner := image.NewNRGBA(image.Rect(0, 0, 200, 150))
	for i := 3; i < len(corner.Pix); i += 4 {
		corner.Pix[i] = 255
	}
	for y := 0; y < 20; y++ {
		for x := 0; x < 20-y; x++ {
			corner.Pix[(y*200+x)*4+3] = 0
		}
	}
	assert.True(t, images.HasTransparency(corner), "solid corner blob is genuine")

	assert.True(t, images.HasTransparency(testhelpers.TestImages["sample-transparency-front.png"]),
		"genuinely transparent image")
}
