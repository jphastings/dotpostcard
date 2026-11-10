package web

import (
	"image"
	"image/color"
	"testing"

	"github.com/jphastings/dotpostcard/types"
	"github.com/stretchr/testify/assert"
)

func TestRotateForWebPlacesEveryPixelWithoutGaps(t *testing.T) {
	const w, h = 3, 2
	src := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := 0; x < w; x++ {
		for y := 0; y < h; y++ {
			src.Set(x, y, color.RGBA{uint8(10 + x), uint8(10 + y), 0, 255})
		}
	}

	cases := map[string]struct {
		flip types.Flip
		to   func(x, y int) (int, int)
	}{
		"left-hand turns anticlockwise": {types.FlipLeftHand, func(x, y int) (int, int) { return y, w - 1 - x }},
		"right-hand turns clockwise":    {types.FlipRightHand, func(x, y int) (int, int) { return h - 1 - y, x }},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			out, bounds := rotateForWeb(src, tc.flip)
			assert.Equal(t, image.Rect(0, 0, h, w), bounds)

			covered := map[image.Point]bool{}
			for x := 0; x < w; x++ {
				for y := 0; y < h; y++ {
					dx, dy := tc.to(x, y)
					covered[image.Pt(dx, dy)] = true
					assert.Equal(t, src.At(x, y), out.At(dx, dy), "source (%d,%d)", x, y)
				}
			}
			assert.Len(t, covered, w*h)
		})
	}
}
