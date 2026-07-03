package matting

import (
	"image"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFillInteriorHoles(t *testing.T) {
	const w, h = 20, 16
	src := image.NewNRGBA(image.Rect(0, 0, w, h))
	out := image.NewNRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < w*h; i++ {
		src.Pix[i*4], src.Pix[i*4+1], src.Pix[i*4+2], src.Pix[i*4+3] = 200, 180, 160, 255

		x, y := i%w, i/w
		switch {
		case x < 3: // transparent margin, connected to the border
			out.Pix[i*4+3] = 0
		case x == 3: // soft fibre edge, connected to the margin
			out.Pix[i*4+3] = 120
		case x == 10 && y == 8: // enclosed semi-transparent speck
			out.Pix[i*4+3] = 90
		case x >= 14 && x <= 15 && y >= 4 && y <= 5: // enclosed transparent blob
			out.Pix[i*4+3] = 0
		default:
			out.Pix[i*4], out.Pix[i*4+1], out.Pix[i*4+2], out.Pix[i*4+3] = 200, 180, 160, 255
		}
	}

	fillInteriorHoles(out, src)

	alphaAt := func(x, y int) uint8 { return out.Pix[(y*w+x)*4+3] }
	assert.EqualValues(t, 0, alphaAt(1, 8), "border-connected margin stays transparent")
	assert.EqualValues(t, 120, alphaAt(3, 8), "soft edge connected to the outside stays soft")
	assert.EqualValues(t, 255, alphaAt(10, 8), "enclosed speck becomes opaque")
	assert.EqualValues(t, 255, alphaAt(14, 4), "enclosed blob becomes opaque")
	assert.EqualValues(t, 200, out.Pix[(8*w+10)*4], "filled pixels get original colour back")
}
