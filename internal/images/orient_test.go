package images

import (
	"image"
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOrientMovesEachCornerToItsDisplayedPosition(t *testing.T) {
	const w, h = 4, 3
	corners := []struct {
		x, y int
		c    color.NRGBA
	}{
		{0, 0, color.NRGBA{255, 0, 0, 255}},
		{w - 1, 0, color.NRGBA{0, 255, 0, 255}},
		{0, h - 1, color.NRGBA{0, 0, 255, 255}},
		{w - 1, h - 1, color.NRGBA{255, 255, 0, 255}},
	}
	src := image.NewNRGBA(image.Rect(0, 0, w, h))
	for _, c := range corners {
		src.SetNRGBA(c.x, c.y, c.c)
	}

	displayed := map[int]func(x, y int) (int, int){
		1: func(x, y int) (int, int) { return x, y },
		2: func(x, y int) (int, int) { return w - 1 - x, y },
		3: func(x, y int) (int, int) { return w - 1 - x, h - 1 - y },
		4: func(x, y int) (int, int) { return x, h - 1 - y },
		5: func(x, y int) (int, int) { return y, x },
		6: func(x, y int) (int, int) { return h - 1 - y, x },
		7: func(x, y int) (int, int) { return h - 1 - y, w - 1 - x },
		8: func(x, y int) (int, int) { return y, w - 1 - x },
	}

	for orientation, toDisplayed := range displayed {
		out := Orient(src, orientation)

		wantW, wantH := w, h
		if orientation >= 5 {
			wantW, wantH = h, w
		}
		assert.Equal(t, image.Rect(0, 0, wantW, wantH), out.Bounds(), "orientation %d", orientation)
		for _, c := range corners {
			dx, dy := toDisplayed(c.x, c.y)
			assert.Equal(t, c.c, color.NRGBAModel.Convert(out.At(dx, dy)), "orientation %d corner (%d,%d)", orientation, c.x, c.y)
		}
	}
}

func TestTurnPlacesEveryPixelClockwiseAndKeepsLowAlphaColour(t *testing.T) {
	const w, h = 3, 2
	src := image.NewNRGBA(image.Rect(5, 7, 5+w, 7+h))
	for x := 0; x < w; x++ {
		for y := 0; y < h; y++ {
			src.SetNRGBA(5+x, 7+y, color.NRGBA{uint8(10 + x), uint8(10 + y), 100, 1})
		}
	}

	to := map[int]func(x, y int) (int, int){
		1:  func(x, y int) (int, int) { return h - 1 - y, x },
		2:  func(x, y int) (int, int) { return w - 1 - x, h - 1 - y },
		-1: func(x, y int) (int, int) { return y, w - 1 - x },
	}
	for quarters, place := range to {
		out := Turn(src, quarters)
		wantW, wantH := w, h
		if quarters%2 != 0 {
			wantW, wantH = h, w
		}
		assert.Equal(t, image.Rect(0, 0, wantW, wantH), out.Bounds())
		for x := 0; x < w; x++ {
			for y := 0; y < h; y++ {
				dx, dy := place(x, y)
				assert.Equal(t, src.NRGBAAt(5+x, 7+y), out.NRGBAAt(dx, dy), "quarters %d source (%d,%d)", quarters, x, y)
			}
		}
	}
}
