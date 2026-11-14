package images

import (
	"image"

	"golang.org/x/image/draw"
	"golang.org/x/image/math/f64"
)

// Orient returns img as it should be displayed under the given EXIF orientation (1–8).
func Orient(img image.Image, orientation int) image.Image {
	b := img.Bounds()
	w, h := float64(b.Dx()), float64(b.Dy())
	storedToDisplayed := map[int]f64.Aff3{
		2: {-1, 0, w, 0, 1, 0},
		3: {-1, 0, w, 0, -1, h},
		4: {1, 0, 0, 0, -1, h},
		5: {0, 1, 0, 1, 0, 0},
		6: {0, -1, h, 1, 0, 0},
		7: {0, -1, h, -1, 0, w},
		8: {0, 1, 0, -1, 0, w},
	}
	s2d, ok := storedToDisplayed[orientation]
	if !ok {
		return img
	}
	dw, dh := b.Dx(), b.Dy()
	if orientation >= 5 {
		dw, dh = dh, dw
	}
	// NRGBA rather than premultiplied RGBA, which would damage soft-alpha edges
	dst := image.NewNRGBA(image.Rect(0, 0, dw, dh))
	draw.NearestNeighbor.Transform(dst, s2d, img, b, draw.Src, nil)
	return dst
}

// Turn rotates img clockwise by the given quarter turns (negative for anticlockwise) as an exact
// pixel permutation, with its origin at (0,0). Orient and image/draw round through premultiplied
// values, which changes the colour of low-alpha (fibrous edge) pixels.
func Turn(img image.Image, quarters int) *image.NRGBA {
	quarters = (quarters%4 + 4) % 4
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if quarters%2 == 1 {
		w, h = h, w
	}
	dst := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			dx, dy := x, y
			switch quarters {
			case 1:
				dx, dy = b.Dy()-1-y, x
			case 2:
				dx, dy = b.Dx()-1-x, b.Dy()-1-y
			case 3:
				dx, dy = y, b.Dx()-1-x
			}
			dst.Set(dx, dy, img.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	return dst
}
